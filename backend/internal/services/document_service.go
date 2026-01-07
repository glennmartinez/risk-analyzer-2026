package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"risk-analyzer/internal/repositories"

	"github.com/google/uuid"
)

// DocumentService orchestrates document processing using Python compute endpoints
// and manages persistence through repositories
type DocumentService struct {
	pythonClient PythonClientInterface
	docRepo      repositories.DocumentRepository
	vectorRepo   repositories.VectorRepository
	jobRepo      repositories.JobRepository
	logger       *log.Logger
}

// NewDocumentService creates a new document service
func NewDocumentService(
	pythonClient PythonClientInterface,
	docRepo repositories.DocumentRepository,
	vectorRepo repositories.VectorRepository,
	jobRepo repositories.JobRepository,
	logger *log.Logger,
) *DocumentService {
	return &DocumentService{
		pythonClient: pythonClient,
		docRepo:      docRepo,
		vectorRepo:   vectorRepo,
		jobRepo:      jobRepo,
		logger:       logger,
	}
}

// UploadDocumentRequest represents a request to upload and process a document
type UploadDocumentRequest struct {
	Filename         string
	FileContent      io.Reader
	FileSize         int64
	Collection       string
	ChunkingStrategy string
	ChunkSize        int
	ChunkOverlap     int
	ExtractMetadata  bool
	NumQuestions     int
	MaxPages         int
	Async            bool
}

// UploadDocumentResponse represents the response from uploading a document
type UploadDocumentResponse struct {
	DocumentID       string                 `json:"document_id"`
	JobID            string                 `json:"job_id,omitempty"`
	Filename         string                 `json:"filename"`
	Collection       string                 `json:"collection"`
	ChunkCount       int                    `json:"chunk_count"`
	Status           string                 `json:"status"`
	ProcessingTimeMs float64                `json:"processing_time_ms,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Message          string                 `json:"message,omitempty"`
}

// UploadDocument orchestrates the full document upload and processing pipeline
func (s *DocumentService) UploadDocument(ctx context.Context, req *UploadDocumentRequest) (*UploadDocumentResponse, error) {
	startTime := time.Now()

	// Validate request
	if err := s.validateUploadRequest(req); err != nil {
		s.logger.Printf("Invalid upload request: %v", err)
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Generate document ID
	documentID := uuid.New().String()

	// If async mode, create job and return immediately
	if req.Async {
		return s.uploadDocumentAsync(ctx, req, documentID)
	}

	// Otherwise, process synchronously
	return s.uploadDocumentSync(ctx, req, documentID, startTime)
}

// uploadDocumentAsync creates a job for async processing
func (s *DocumentService) uploadDocumentAsync(ctx context.Context, req *UploadDocumentRequest, documentID string) (*UploadDocumentResponse, error) {
	jobID := uuid.New().String()

	// Save file to disk for async processing
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "/tmp/risk-analyzer-uploads"
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		s.logger.Printf("Failed to create upload directory: %v", err)
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save file with unique name
	filePath := filepath.Join(uploadDir, fmt.Sprintf("%s_%s", documentID, req.Filename))
	outFile, err := os.Create(filePath)
	if err != nil {
		s.logger.Printf("Failed to create file: %v", err)
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	// Copy file content
	_, err = io.Copy(outFile, req.FileContent)
	if err != nil {
		s.logger.Printf("Failed to save file: %v", err)
		os.Remove(filePath) // Cleanup
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	s.logger.Printf("Saved uploaded file to: %s", filePath)

	// Create job in repository
	job := &repositories.Job{
		ID:         jobID,
		Type:       repositories.JobTypeDocumentUpload,
		Status:     repositories.JobStatusQueued,
		Priority:   1,
		Progress:   0,
		Message:    "Document upload queued",
		MaxRetries: 3,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Payload: map[string]interface{}{
			"document_id":       documentID,
			"filename":          req.Filename,
			"file_path":         filePath, // Add file path for worker
			"file_size":         req.FileSize,
			"collection":        req.Collection,
			"chunking_strategy": req.ChunkingStrategy,
			"chunk_size":        req.ChunkSize,
			"chunk_overlap":     req.ChunkOverlap,
			"extract_metadata":  req.ExtractMetadata,
			"num_questions":     req.NumQuestions,
			"max_pages":         req.MaxPages,
		},
	}

	// Create the job first
	if err := s.jobRepo.CreateJob(ctx, job); err != nil {
		s.logger.Printf("Failed to create job: %v", err)
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// Enqueue the job for processing
	if err := s.jobRepo.EnqueueJob(ctx, job); err != nil {
		s.logger.Printf("Failed to enqueue job: %v", err)
		// Try to cleanup job
		_ = s.jobRepo.DeleteJob(ctx, jobID)
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Register document in pending state
	doc := &repositories.Document{
		ID:               documentID,
		Filename:         req.Filename,
		Collection:       req.Collection,
		Status:           repositories.DocumentStatusPending,
		FileSize:         req.FileSize,
		ChunkingStrategy: req.ChunkingStrategy,
		ChunkSize:        req.ChunkSize,
		ChunkOverlap:     req.ChunkOverlap,
		ExtractMetadata:  req.ExtractMetadata,
		NumQuestions:     req.NumQuestions,
		MaxPages:         req.MaxPages,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.docRepo.Register(ctx, doc); err != nil {
		s.logger.Printf("Failed to register document: %v", err)
		// Try to cleanup job
		_ = s.jobRepo.DeleteJob(ctx, jobID)
		return nil, fmt.Errorf("failed to register document: %w", err)
	}

	s.logger.Printf("Created async upload job: job_id=%s, document_id=%s", jobID, documentID)

	return &UploadDocumentResponse{
		DocumentID: documentID,
		JobID:      jobID,
		Filename:   req.Filename,
		Collection: req.Collection,
		Status:     "queued",
		Message:    "Document upload queued for processing",
	}, nil
}

// uploadDocumentSync processes document synchronously
func (s *DocumentService) uploadDocumentSync(ctx context.Context, req *UploadDocumentRequest, documentID string, startTime time.Time) (*UploadDocumentResponse, error) {
	// Register document in processing state
	doc := &repositories.Document{
		ID:               documentID,
		Filename:         req.Filename,
		Collection:       req.Collection,
		Status:           repositories.DocumentStatusProcessing,
		FileSize:         req.FileSize,
		ChunkingStrategy: req.ChunkingStrategy,
		ChunkSize:        req.ChunkSize,
		ChunkOverlap:     req.ChunkOverlap,
		ExtractMetadata:  req.ExtractMetadata,
		NumQuestions:     req.NumQuestions,
		MaxPages:         req.MaxPages,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.docRepo.Register(ctx, doc); err != nil {
		s.logger.Printf("Failed to register document: %v", err)
		return nil, fmt.Errorf("failed to register document: %w", err)
	}

	// Process the document
	result, err := s.processDocument(ctx, req, documentID)
	if err != nil {
		s.logger.Printf("Failed to process document %s: %v", documentID, err)
		// Update document status to failed
		_ = s.docRepo.Update(ctx, documentID, map[string]interface{}{
			"status":     repositories.DocumentStatusFailed,
			"updated_at": time.Now(),
		})
		return nil, fmt.Errorf("failed to process document: %w", err)
	}

	// Update document status to completed
	err = s.docRepo.Update(ctx, documentID, map[string]interface{}{
		"status":              repositories.DocumentStatusCompleted,
		"chunk_count":         result.ChunkCount,
		"stored_in_vector_db": true,
		"metadata":            result.Metadata,
		"updated_at":          time.Now(),
	})
	if err != nil {
		s.logger.Printf("Failed to update document status: %v", err)
		// Document is processed but status update failed - not critical
	}

	processingTime := time.Since(startTime).Milliseconds()
	s.logger.Printf("Document processed successfully: document_id=%s, chunks=%d, time_ms=%d", documentID, result.ChunkCount, processingTime)

	return &UploadDocumentResponse{
		DocumentID:       documentID,
		Filename:         req.Filename,
		Collection:       req.Collection,
		ChunkCount:       result.ChunkCount,
		Status:           "completed",
		ProcessingTimeMs: float64(processingTime),
		Metadata:         result.Metadata,
		Message:          "Document processed successfully",
	}, nil
}

// ProcessingResult contains the results of document processing
type ProcessingResult struct {
	ChunkCount int
	Metadata   map[string]interface{}
}

// processDocument handles the core document processing logic
func (s *DocumentService) processDocument(ctx context.Context, req *UploadDocumentRequest, documentID string) (*ProcessingResult, error) {
	s.logger.Printf("Processing document: %s (collection: %s)", documentID, req.Collection)

	// Step 1: Parse document
	s.logger.Printf("[%s] Step 1/4: Parsing document", documentID)

	// Read file content into bytes
	fileBytes, err := io.ReadAll(req.FileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	parseResp, err := s.pythonClient.ParseDocument(ctx, fileBytes, req.Filename, req.ExtractMetadata, req.MaxPages)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	if parseResp.Text == "" {
		return nil, fmt.Errorf("parse returned empty text")
	}

	s.logger.Printf("[%s] Parsed %d pages, %d chars", documentID, parseResp.TotalPages, len(parseResp.Text))

	// Step 2: Chunk text
	s.logger.Printf("[%s] Step 2/4: Chunking text", documentID)
	chunkReq := &ChunkRequest{
		Text:            parseResp.Text,
		Strategy:        req.ChunkingStrategy,
		ChunkSize:       req.ChunkSize,
		ChunkOverlap:    req.ChunkOverlap,
		ExtractMetadata: req.ExtractMetadata,
		NumQuestions:    req.NumQuestions,
	}

	chunkResp, err := s.pythonClient.Chunk(ctx, chunkReq)
	if err != nil {
		return nil, fmt.Errorf("chunking failed: %w", err)
	}

	if len(chunkResp.Chunks) == 0 {
		return nil, fmt.Errorf("chunking returned no chunks")
	}

	s.logger.Printf("[%s] Created %d chunks", documentID, len(chunkResp.Chunks))

	// Step 3: Generate embeddings
	s.logger.Printf("[%s] Step 3/4: Generating embeddings", documentID)
	texts := make([]string, len(chunkResp.Chunks))
	for i, chunk := range chunkResp.Chunks {
		texts[i] = chunk.Text
	}

	embedResp, err := s.pythonClient.EmbedBatch(ctx, texts, nil, 32, false)
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	if len(embedResp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embedResp.Embeddings), len(texts))
	}

	s.logger.Printf("[%s] Generated %d embeddings", documentID, len(embedResp.Embeddings))

	// Step 4: Extract document-level metadata (optional)
	var docMetadata map[string]interface{}
	if req.ExtractMetadata {
		s.logger.Printf("[%s] Step 4/4: Extracting metadata", documentID)
		metadataReq := &MetadataRequest{
			Text:             parseResp.Text,
			ExtractTitle:     true,
			ExtractKeywords:  true,
			ExtractQuestions: true,
			NumQuestions:     req.NumQuestions,
			NumKeywords:      10,
		}
		metadataResp, err := s.pythonClient.ExtractMetadata(ctx, metadataReq)
		if err != nil {
			s.logger.Printf("[%s] Metadata extraction failed (non-critical): %v", documentID, err)
			docMetadata = make(map[string]interface{})
		} else {
			docMetadata = map[string]interface{}{
				"title":       metadataResp.Title,
				"keywords":    metadataResp.Keywords,
				"questions":   metadataResp.Questions,
				"total_pages": parseResp.TotalPages,
			}
		}
	} else {
		s.logger.Printf("[%s] Step 4/4: Skipping metadata extraction", documentID)
		docMetadata = map[string]interface{}{
			"total_pages": parseResp.TotalPages,
		}
	}

	// Step 5: Store chunks in vector database
	s.logger.Printf("[%s] Step 5/5: Storing chunks in vector DB", documentID)
	if err := s.storeChunksInVectorDB(ctx, req.Collection, documentID, chunkResp.Chunks, embedResp.Embeddings, parseResp.Metadata); err != nil {
		return nil, fmt.Errorf("vector storage failed: %w", err)
	}

	s.logger.Printf("[%s] Successfully stored %d chunks in collection '%s'", documentID, len(chunkResp.Chunks), req.Collection)

	return &ProcessingResult{
		ChunkCount: len(chunkResp.Chunks),
		Metadata:   docMetadata,
	}, nil
}

// storeChunksInVectorDB stores chunks with embeddings in the vector database
func (s *DocumentService) storeChunksInVectorDB(
	ctx context.Context,
	collection string,
	documentID string,
	chunks []TextChunk,
	embeddings [][]float32,
	parseMetadata map[string]interface{},
) error {
	// Ensure collection exists
	exists, err := s.vectorRepo.CollectionExists(ctx, collection)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}

	if !exists {
		if err := s.vectorRepo.CreateCollection(ctx, collection, nil); err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
		s.logger.Printf("Created collection: %s", collection)
	}

	// Build chunks for vector storage
	vectorChunks := make([]*repositories.Chunk, len(chunks))
	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("%s_chunk_%d", documentID, i)

		// Build chunk metadata
		chunkMetadata := map[string]interface{}{
			"document_id": documentID,
			"chunk_index": i,
		}

		// Add chunk-specific metadata if available
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

		// Add parse metadata (page numbers, etc.)
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

	// Store chunks in batches
	batchSize := 100
	for i := 0; i < len(vectorChunks); i += batchSize {
		end := i + batchSize
		if end > len(vectorChunks) {
			end = len(vectorChunks)
		}

		batch := vectorChunks[i:end]
		if err := s.vectorRepo.StoreChunks(ctx, collection, batch); err != nil {
			return fmt.Errorf("failed to store batch %d-%d: %w", i, end, err)
		}

		s.logger.Printf("[%s] Stored batch %d-%d of %d chunks", documentID, i, end, len(vectorChunks))
	}

	return nil
}

// validateUploadRequest validates the upload request parameters
func (s *DocumentService) validateUploadRequest(req *UploadDocumentRequest) error {
	if req.Filename == "" {
		return fmt.Errorf("filename is required")
	}

	if req.FileContent == nil {
		return fmt.Errorf("file content is required")
	}

	if req.Collection == "" {
		return fmt.Errorf("collection is required")
	}

	// Set defaults
	if req.ChunkingStrategy == "" {
		req.ChunkingStrategy = "semantic"
	}

	if req.ChunkSize <= 0 {
		req.ChunkSize = 512
	}

	if req.ChunkOverlap < 0 {
		req.ChunkOverlap = 50
	}

	if req.MaxPages <= 0 {
		req.MaxPages = 0 // 0 means no limit
	}

	// Validate file extension
	ext := filepath.Ext(req.Filename)
	validExtensions := map[string]bool{
		".pdf": true, ".txt": true, ".md": true, ".docx": true,
		".doc": true, ".html": true, ".htm": true,
	}

	if !validExtensions[ext] {
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	return nil
}

// DeleteDocument removes a document and all its chunks
func (s *DocumentService) DeleteDocument(ctx context.Context, documentID string) error {
	s.logger.Printf("Deleting document: %s", documentID)

	// Get document to find collection
	doc, err := s.docRepo.Get(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Delete from vector database (ChromaDB)
	deletedCount, err := s.vectorRepo.DeleteDocument(ctx, doc.Collection, documentID)
	if err != nil {
		s.logger.Printf("Failed to delete from vector DB: %v", err)
		return fmt.Errorf("failed to delete chunks from vector DB: %w", err)
	}
	s.logger.Printf("Deleted %d chunks from vector DB", deletedCount)

	// Delete from document registry (Redis)
	err = s.docRepo.Delete(ctx, documentID)
	if err != nil {
		s.logger.Printf("Failed to delete from document registry: %v", err)
		return fmt.Errorf("failed to delete from document registry: %w", err)
	}

	s.logger.Printf("Document deleted successfully from both Redis and ChromaDB: %s", documentID)
	return nil
}

// GetDocument retrieves document metadata
func (s *DocumentService) GetDocument(ctx context.Context, documentID string) (*repositories.Document, error) {
	return s.docRepo.Get(ctx, documentID)
}

// ListDocuments lists all documents
func (s *DocumentService) ListDocuments(ctx context.Context) ([]*repositories.Document, error) {
	return s.docRepo.List(ctx)
}

// ListDocumentsByCollection lists documents in a specific collection
func (s *DocumentService) ListDocumentsByCollection(ctx context.Context, collection string) ([]*repositories.Document, error) {
	return s.docRepo.ListByCollection(ctx, collection)
}

// GetDocumentStatus retrieves the status of a document
// GetDocumentChunksResponse represents the response for document chunks
type GetDocumentChunksResponse struct {
	Chunks     []*repositories.Chunk `json:"chunks"`
	TotalCount int                   `json:"total_count"`
	Limit      int                   `json:"limit"`
	Offset     int                   `json:"offset"`
	DocumentID string                `json:"document_id"`
	Collection string                `json:"collection"`
}

// GetDocumentChunks retrieves chunks for a document from the vector store
func (s *DocumentService) GetDocumentChunks(ctx context.Context, documentID string, collection string, limit int, offset int) (*GetDocumentChunksResponse, error) {
	if s.vectorRepo == nil {
		return nil, fmt.Errorf("vector repository not available")
	}

	if collection == "" {
		// Try to get collection from document metadata in Redis
		doc, err := s.docRepo.Get(ctx, documentID)
		if err == nil {
			collection = doc.Collection
		} else {
			// Document not in Redis - search all collections in ChromaDB
			s.logger.Printf("Document %s not in Redis, searching ChromaDB collections...", documentID)
			collections, err := s.vectorRepo.ListCollections(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list collections: %w", err)
			}

			// Search each collection for this document
			for _, collName := range collections {
				chunks, _, err := s.vectorRepo.GetDocumentChunks(ctx, collName, documentID, 1, 0)
				if err == nil && len(chunks) > 0 {
					collection = collName
					s.logger.Printf("Found document %s in collection %s", documentID, collection)
					break
				}
			}

			if collection == "" {
				return nil, fmt.Errorf("document not found in any collection: %s", documentID)
			}
		}
	}

	if limit <= 0 {
		limit = 100
	}

	chunks, totalCount, err := s.vectorRepo.GetDocumentChunks(ctx, collection, documentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunks: %w", err)
	}

	return &GetDocumentChunksResponse{
		Chunks:     chunks,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
		DocumentID: documentID,
		Collection: collection,
	}, nil
}

func (s *DocumentService) GetDocumentStatus(ctx context.Context, documentID string) (repositories.DocumentStatus, error) {
	doc, err := s.docRepo.Get(ctx, documentID)
	if err != nil {
		return "", err
	}
	return doc.Status, nil
}

// DocumentStatusDetails contains detailed status info including job progress
type DocumentStatusDetails struct {
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
	JobID    string `json:"job_id,omitempty"`
}

// GetDocumentStatusWithProgress retrieves document status with job progress details
func (s *DocumentService) GetDocumentStatusWithProgress(ctx context.Context, documentID string) (*DocumentStatusDetails, error) {
	doc, err := s.docRepo.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}

	details := &DocumentStatusDetails{
		Status:   string(doc.Status),
		Progress: 0,
		Message:  "",
	}

	// If document is completed or failed, return 100% or 0%
	if doc.Status == repositories.DocumentStatusCompleted {
		details.Progress = 100
		details.Message = "Document processing completed"
		return details, nil
	}
	if doc.Status == repositories.DocumentStatusFailed {
		details.Progress = 0
		details.Message = "Document processing failed"
		return details, nil
	}

	// Try to get job progress for pending/processing documents
	if s.jobRepo != nil {
		// Look for job by document ID in payload
		jobs, err := s.jobRepo.ListJobsByType(ctx, repositories.JobTypeDocumentUpload)
		if err == nil {
			for _, job := range jobs {
				if jobDocID, ok := job.Payload["document_id"].(string); ok && jobDocID == documentID {
					details.JobID = job.ID
					details.Progress = job.Progress
					details.Message = job.Message
					details.Status = string(job.Status)
					break
				}
			}
		}
	}

	return details, nil
}

// ProcessJob processes an async upload job (called by worker)
func (s *DocumentService) ProcessJob(ctx context.Context, job *repositories.Job) error {
	s.logger.Printf("Processing job: %s (type: %s)", job.ID, job.Type)

	if job.Type != repositories.JobTypeDocumentUpload {
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}

	// Extract payload
	payload := job.Payload
	documentID, _ := payload["document_id"].(string)
	filename, _ := payload["filename"].(string)
	collection, _ := payload["collection"].(string)
	chunkingStrategy, _ := payload["chunking_strategy"].(string)
	chunkSize, _ := payload["chunk_size"].(int)
	chunkOverlap, _ := payload["chunk_overlap"].(int)
	extractMetadata, _ := payload["extract_metadata"].(bool)
	numQuestions, _ := payload["num_questions"].(int)
	maxPages, _ := payload["max_pages"].(int)

	// Update job status
	_ = s.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusProcessing, 0, "Processing document")

	// Update document status
	_ = s.docRepo.Update(ctx, documentID, map[string]interface{}{
		"status":     repositories.DocumentStatusProcessing,
		"updated_at": time.Now(),
	})

	// Note: In a real worker, you'd need to fetch the file content from storage
	// For now, this is a placeholder showing the structure
	req := &UploadDocumentRequest{
		Filename:         filename,
		FileContent:      nil, // Would be loaded from storage
		Collection:       collection,
		ChunkingStrategy: chunkingStrategy,
		ChunkSize:        chunkSize,
		ChunkOverlap:     chunkOverlap,
		ExtractMetadata:  extractMetadata,
		NumQuestions:     numQuestions,
		MaxPages:         maxPages,
	}

	// Process document
	result, err := s.processDocument(ctx, req, documentID)
	if err != nil {
		s.logger.Printf("Job %s failed: %v", job.ID, err)

		// Update job status
		_ = s.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, err.Error())

		// Update document status
		_ = s.docRepo.Update(ctx, documentID, map[string]interface{}{
			"status":     repositories.DocumentStatusFailed,
			"updated_at": time.Now(),
		})

		return err
	}

	// Update job with result
	_ = s.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusCompleted, 100, "Document processed successfully")
	_ = s.jobRepo.UpdateJobResult(ctx, job.ID, map[string]interface{}{
		"document_id": documentID,
		"chunk_count": result.ChunkCount,
		"collection":  collection,
		"success":     true,
	})

	// Update document status
	_ = s.docRepo.Update(ctx, documentID, map[string]interface{}{
		"status":              repositories.DocumentStatusCompleted,
		"chunk_count":         result.ChunkCount,
		"stored_in_vector_db": true,
		"metadata":            result.Metadata,
		"updated_at":          time.Now(),
	})

	s.logger.Printf("Job %s completed successfully", job.ID)
	return nil
}
