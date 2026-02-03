package processors

import (
	"context"
	"fmt"
	"log"
	"time"

	"risk-analyzer/internal/models"
	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"
	"risk-analyzer/internal/services/adapters"
)

type ContentIngestProcessor struct {
	BaseProcessor
	adapterRegistry *adapters.ContentAdapterRegistry
}

func NewContentIngestProcessor(
	py services.PythonClientInterface,
	jr repositories.JobRepository,
	dr repositories.DocumentRepository,
	cbBase string,
	adapterRegistry *adapters.ContentAdapterRegistry,
) *ContentIngestProcessor {
	bp := BaseProcessor{Py: py, JobRepo: jr, DocRepo: dr, CallbackBase: cbBase, CallbackPath: "/api/v1/documents/upload-callback"}
	return &ContentIngestProcessor{
		BaseProcessor:   bp,
		adapterRegistry: adapterRegistry,
	}
}

func (p *ContentIngestProcessor) StartProcessing(ctx context.Context, job *repositories.Job) error {
	log.Printf("ContentIngestProcessor starting job: %s", job.ID)

	payload := job.Payload
	contentPayload, ok := payload["content"]
	if !ok {
		return fmt.Errorf("missing content in job payload")
	}

	sourceTypeStr, _ := payload["source_type"].(string)
	sourceType := models.ContentSourceType(sourceTypeStr)

	adapter := p.adapterRegistry.GetAdapter(sourceType)
	if adapter == nil {
		return fmt.Errorf("no adapter found for source type: %s", sourceType)
	}

	text, err := adapter.ToText(contentPayload)
	if err != nil {
		return fmt.Errorf("failed to convert content to text: %w", err)
	}

	if text == "" {
		return fmt.Errorf("content conversion resulted in empty text")
	}

	documentID, _ := payload["document_id"].(string)
	collection, _ := payload["collection"].(string)
	chunkingStrategy, _ := payload["chunking_strategy"].(string)
	chunkSize, _ := payload["chunk_size"].(int)
	chunkOverlap, _ := payload["chunk_overlap"].(int)
	extractMetadata, _ := payload["extract_metadata"].(bool)
	numQuestions, _ := payload["num_questions"].(int)

	log.Printf("ContentIngestProcessor: chunking %d chars (type=%s)", len(text), sourceType)

	_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusProcessing, 20, "Content normalized, starting chunking")

	chunkReq := &services.ChunkRequest{
		Text:            text,
		Strategy:        chunkingStrategy,
		ChunkSize:       chunkSize,
		ChunkOverlap:    chunkOverlap,
		ExtractMetadata: extractMetadata,
		NumQuestions:    numQuestions,
	}

	chunkResp, err := p.Py.Chunk(ctx, chunkReq)
	if err != nil {
		_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, err.Error())
		return fmt.Errorf("chunking failed: %w", err)
	}

	if len(chunkResp.Chunks) == 0 {
		_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, "chunking returned no chunks")
		return fmt.Errorf("chunking returned no chunks")
	}

	log.Printf("ContentIngestProcessor: created %d chunks", len(chunkResp.Chunks))

	_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusProcessing, 50, fmt.Sprintf("Created %d chunks, generating embeddings", len(chunkResp.Chunks)))

	texts := make([]string, len(chunkResp.Chunks))
	for i, chunk := range chunkResp.Chunks {
		texts[i] = chunk.Text
	}

	embedResp, err := p.Py.EmbedBatch(ctx, texts, nil, 32, false)
	if err != nil {
		_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, err.Error())
		return fmt.Errorf("embedding failed: %w", err)
	}

	if len(embedResp.Embeddings) != len(texts) {
		_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, "embedding count mismatch")
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embedResp.Embeddings), len(texts))
	}

	log.Printf("ContentIngestProcessor: generated %d embeddings", len(embedResp.Embeddings))

	_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusProcessing, 80, "Storing in vector database")

	sourceMetadata := adapter.ToMetadata(contentPayload)
	if sourceMetadata == nil {
		sourceMetadata = make(map[string]interface{})
	}
	sourceMetadata["source_type"] = sourceTypeStr

	if err := p.storeChunksInVectorDB(ctx, collection, documentID, chunkResp.Chunks, embedResp.Embeddings, sourceMetadata); err != nil {
		_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, err.Error())
		return fmt.Errorf("vector storage failed: %w", err)
	}

	if p.DocRepo != nil && documentID != "" {
		_ = p.DocRepo.Update(ctx, documentID, map[string]interface{}{
			"status":              repositories.DocumentStatusCompleted,
			"chunk_count":         len(chunkResp.Chunks),
			"stored_in_vector_db": true,
			"metadata":            sourceMetadata,
			"updated_at":          time.Now(),
		})
	}

	_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusCompleted, 100, "Content processed successfully")
	_ = p.JobRepo.UpdateJobResult(ctx, job.ID, map[string]interface{}{
		"document_id": documentID,
		"chunk_count": len(chunkResp.Chunks),
		"collection":  collection,
		"success":     true,
		"char_count":  len(text),
	})

	log.Printf("ContentIngestProcessor: job %s completed successfully", job.ID)
	return nil
}

func (p *ContentIngestProcessor) storeChunksInVectorDB(
	ctx context.Context,
	collection string,
	documentID string,
	chunks []services.TextChunk,
	embeddings [][]float32,
	parseMetadata map[string]interface{},
) error {
	if p.VectorRepo == nil {
		return fmt.Errorf("vector repository not available")
	}

	exists, err := p.VectorRepo.CollectionExists(ctx, collection)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}

	if !exists {
		if err := p.VectorRepo.CreateCollection(ctx, collection, nil); err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
		log.Printf("Created collection: %s", collection)
	}

	vectorChunks := make([]*repositories.Chunk, len(chunks))
	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("%s_chunk_%d", documentID, i)

		chunkMetadata := map[string]interface{}{
			"document_id": documentID,
			"chunk_index": i,
		}

		if chunk.Metadata != nil {
			if chunk.Metadata.Title != nil {
				chunkMetadata["title"] = *chunk.Metadata.Title
			}
			if chunk.Metadata.Keywords != nil {
				chunkMetadata["keywords"] = chunk.Metadata.Keywords
			}
			if chunk.Metadata.Questions != nil {
				chunkMetadata["questions"] = chunk.Metadata.Questions
			}
			if chunk.Metadata.TokenCount != nil {
				chunkMetadata["token_count"] = *chunk.Metadata.TokenCount
			}
		}

		for k, v := range parseMetadata {
			if _, exists := chunkMetadata[k]; !exists {
				chunkMetadata[k] = v
			}
		}

		vectorChunks[i] = &repositories.Chunk{
			ID:         chunkID,
			DocumentID: documentID,
			Text:       chunk.Text,
			Embedding:  embeddings[i],
			Metadata:   chunkMetadata,
			ChunkIndex: i,
		}
	}

	batchSize := 100
	for i := 0; i < len(vectorChunks); i += batchSize {
		end := i + batchSize
		if end > len(vectorChunks) {
			end = len(vectorChunks)
		}

		batch := vectorChunks[i:end]
		if err := p.VectorRepo.StoreChunks(ctx, collection, batch); err != nil {
			return fmt.Errorf("failed to store batch %d-%d: %w", i, end, err)
		}

		log.Printf("Stored batch %d-%d of %d chunks", i, end, len(vectorChunks))
	}

	return nil
}
