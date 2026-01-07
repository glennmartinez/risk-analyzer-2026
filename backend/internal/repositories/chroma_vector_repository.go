package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"risk-analyzer/internal/db"
)

// ChromaVectorRepository implements VectorRepository using ChromaDB
type ChromaVectorRepository struct {
	client *db.ChromaDBClient
}

// NewChromaVectorRepository creates a new ChromaDB-backed vector repository
func NewChromaVectorRepository(client *db.ChromaDBClient) VectorRepository {
	return &ChromaVectorRepository{
		client: client,
	}
}

// CreateCollection creates a new collection
func (r *ChromaVectorRepository) CreateCollection(ctx context.Context, name string, metadata map[string]interface{}) error {
	// Check if collection already exists
	exists, err := r.CollectionExists(ctx, name)
	if err != nil {
		return NewVectorRepositoryError("create_collection", err, "")
	}
	if exists {
		return CollectionAlreadyExistsError(name)
	}

	_, err = r.client.CreateCollection(ctx, name, metadata)
	if err != nil {
		return NewVectorRepositoryError("create_collection", err, "failed to create collection: "+name)
	}

	return nil
}

// DeleteCollection deletes a collection
func (r *ChromaVectorRepository) DeleteCollection(ctx context.Context, name string) error {
	err := r.client.DeleteCollection(ctx, name)
	if err != nil {
		return NewVectorRepositoryError("delete_collection", err, "failed to delete collection: "+name)
	}
	return nil
}

// GetCollection retrieves collection information
func (r *ChromaVectorRepository) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	collection, err := r.client.GetCollection(ctx, name)
	if err != nil {
		return nil, CollectionNotFoundError(name)
	}

	return &CollectionInfo{
		ID:       collection.ID,
		Name:     collection.Name,
		Metadata: collection.Metadata,
	}, nil
}

// ListCollections returns all collection names
func (r *ChromaVectorRepository) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := r.client.ListCollections(ctx)
	if err != nil {
		return nil, NewVectorRepositoryError("list_collections", err, "")
	}

	names := make([]string, len(collections))
	for i, col := range collections {
		names[i] = col.Name
	}

	return names, nil
}

// GetCollectionStats returns statistics for a collection
func (r *ChromaVectorRepository) GetCollectionStats(ctx context.Context, name string) (*CollectionStats, error) {
	// Get collection to verify it exists
	collection, err := r.client.GetCollection(ctx, name)
	if err != nil {
		return nil, CollectionNotFoundError(name)
	}

	// Get count
	count, err := r.client.CountCollection(ctx, name)
	if err != nil {
		return nil, NewVectorRepositoryError("get_collection_stats", err, "failed to count collection: "+name)
	}

	// Count unique documents
	docs, err := r.ListDocuments(ctx, name)
	if err != nil {
		return nil, NewVectorRepositoryError("get_collection_stats", err, "failed to list documents")
	}

	return &CollectionStats{
		Name:          collection.Name,
		DocumentCount: len(docs),
		ChunkCount:    count,
		Metadata:      collection.Metadata,
	}, nil
}

// CollectionExists checks if a collection exists
func (r *ChromaVectorRepository) CollectionExists(ctx context.Context, name string) (bool, error) {
	_, err := r.client.GetCollection(ctx, name)
	if err != nil {
		// Assume not found error means collection doesn't exist
		return false, nil
	}
	return true, nil
}

// StoreChunks stores chunks in a collection
func (r *ChromaVectorRepository) StoreChunks(ctx context.Context, collectionName string, chunks []*Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Verify collection exists
	exists, err := r.CollectionExists(ctx, collectionName)
	if err != nil {
		return NewVectorRepositoryError("store_chunks", err, "")
	}
	if !exists {
		return CollectionNotFoundError(collectionName)
	}

	// Prepare data for ChromaDB
	ids := make([]string, len(chunks))
	documents := make([]string, len(chunks))
	embeddings := make([][]float32, len(chunks))
	metadatas := make([]map[string]interface{}, len(chunks))

	for i, chunk := range chunks {
		ids[i] = chunk.ID
		documents[i] = chunk.Text
		embeddings[i] = chunk.Embedding

		// Build metadata
		metadata := make(map[string]interface{})
		metadata["document_id"] = chunk.DocumentID
		metadata["chunk_index"] = chunk.ChunkIndex

		if chunk.PageNumber != nil {
			metadata["page_number"] = *chunk.PageNumber
		}
		if chunk.TokenCount != nil {
			metadata["token_count"] = *chunk.TokenCount
		}

		jsonBytes, _ := json.Marshal(chunk.Metadata["keywords"])
		fmt.Println(string(jsonBytes))
		questionsBytes, _ := json.Marshal(chunk.Metadata["questions"])
		fmt.Println(string(questionsBytes))
		// Merge custom metadata, converting arrays to JSON strings for ChromaDB compatibility
		for k, v := range chunk.Metadata {
			// ChromaDB only supports simple types (string, int, float, bool)
			// Arrays and objects must be serialized to JSON strings
			// log the metadata value here for debugging
			//

			switch val := v.(type) {
			case []string:
				// Convert string arrays to JSON
				if jsonBytes, err := json.Marshal(val); err == nil {
					metadata[k] = string(jsonBytes)
				}
			case []interface{}:
				// Convert interface arrays to JSON
				if jsonBytes, err := json.Marshal(val); err == nil {
					metadata[k] = string(jsonBytes)
				}
			case map[string]interface{}:
				// Convert maps to JSON
				if jsonBytes, err := json.Marshal(val); err == nil {
					metadata[k] = string(jsonBytes)
				}
			default:
				// Simple types pass through directly
				metadata[k] = v
			}
		}

		metadatas[i] = metadata
	}

	// Store in ChromaDB
	err = r.client.AddDocuments(ctx, collectionName, ids, documents, embeddings, metadatas)
	if err != nil {
		return NewVectorRepositoryError("store_chunks", err, fmt.Sprintf("failed to store %d chunks", len(chunks)))
	}

	return nil
}

// SearchChunks searches for similar chunks
func (r *ChromaVectorRepository) SearchChunks(ctx context.Context, collectionName string, queryEmbedding []float32, topK int, filter map[string]interface{}) ([]*SearchResult, error) {
	// Verify collection exists
	exists, err := r.CollectionExists(ctx, collectionName)
	if err != nil {
		return nil, NewVectorRepositoryError("search_chunks", err, "")
	}
	if !exists {
		return nil, CollectionNotFoundError(collectionName)
	}

	// Query ChromaDB
	queryEmbeddings := [][]float32{queryEmbedding}
	results, err := r.client.Query(ctx, collectionName, queryEmbeddings, topK, filter)
	if err != nil {
		return nil, NewVectorRepositoryError("search_chunks", err, "query failed")
	}

	// Convert to SearchResult
	searchResults := make([]*SearchResult, 0)
	if len(results.IDs) > 0 && len(results.IDs[0]) > 0 {
		for i := 0; i < len(results.IDs[0]); i++ {
			metadata := make(map[string]interface{})
			if len(results.Metadatas) > 0 && len(results.Metadatas[0]) > i {
				metadata = results.Metadatas[0][i]
			}

			var text string
			if len(results.Documents) > 0 && len(results.Documents[0]) > i {
				text = results.Documents[0][i]
			}

			var distance float32
			if len(results.Distances) > 0 && len(results.Distances[0]) > i {
				distance = results.Distances[0][i]
			}

			// Calculate similarity score (1 - distance for cosine)
			score := 1.0 - distance

			// Extract document_id from metadata
			documentID := ""
			if docID, ok := metadata["document_id"].(string); ok {
				documentID = docID
			}

			searchResults = append(searchResults, &SearchResult{
				ChunkID:    results.IDs[0][i],
				DocumentID: documentID,
				Text:       text,
				Score:      score,
				Distance:   distance,
				Metadata:   metadata,
			})
		}
	}

	return searchResults, nil
}

// DeleteDocument deletes all chunks for a document
func (r *ChromaVectorRepository) DeleteDocument(ctx context.Context, collectionName string, documentID string) (int, error) {
	// Verify collection exists
	exists, err := r.CollectionExists(ctx, collectionName)
	if err != nil {
		return 0, NewVectorRepositoryError("delete_document", err, "")
	}
	if !exists {
		return 0, CollectionNotFoundError(collectionName)
	}

	// Step 1: Get all chunks for this document
	where := map[string]interface{}{
		"document_id": documentID,
	}
	result, err := r.client.GetDocuments(ctx, collectionName, where, 0, 0, false)
	if err != nil {
		return 0, NewVectorRepositoryError("delete_document", err, "failed to get chunks for document")
	}

	if len(result.IDs) == 0 {
		// No chunks found for this document
		return 0, nil
	}

	// Step 2: Delete all chunks by their IDs
	err = r.client.DeleteDocuments(ctx, collectionName, result.IDs)
	if err != nil {
		return 0, NewVectorRepositoryError("delete_document", err, fmt.Sprintf("failed to delete %d chunks", len(result.IDs)))
	}

	return len(result.IDs), nil
}

// DeleteChunks deletes specific chunks by their IDs
func (r *ChromaVectorRepository) DeleteChunks(ctx context.Context, collectionName string, chunkIDs []string) error {
	if len(chunkIDs) == 0 {
		return nil
	}

	// Verify collection exists
	exists, err := r.CollectionExists(ctx, collectionName)
	if err != nil {
		return NewVectorRepositoryError("delete_chunks", err, "")
	}
	if !exists {
		return CollectionNotFoundError(collectionName)
	}

	err = r.client.DeleteDocuments(ctx, collectionName, chunkIDs)
	if err != nil {
		return NewVectorRepositoryError("delete_chunks", err, fmt.Sprintf("failed to delete %d chunks", len(chunkIDs)))
	}

	return nil
}

// GetChunk retrieves a specific chunk by ID
func (r *ChromaVectorRepository) GetChunk(ctx context.Context, collectionName string, chunkID string) (*Chunk, error) {
	// Note: Current ChromaDB Go client doesn't have a direct "get by ID" method
	// We would need to query the collection or use the REST API directly
	// For now, return not implemented error
	return nil, NewVectorRepositoryError("get_chunk", nil, "get_chunk not yet implemented - ChromaDB client limitation")
}

// GetDocumentChunks retrieves all chunks for a specific document
func (r *ChromaVectorRepository) GetDocumentChunks(ctx context.Context, collectionName string, documentID string, limit int, offset int) ([]*Chunk, int, error) {
	// Verify collection exists
	exists, err := r.CollectionExists(ctx, collectionName)
	if err != nil {
		return nil, 0, NewVectorRepositoryError("get_document_chunks", err, "")
	}
	if !exists {
		return nil, 0, CollectionNotFoundError(collectionName)
	}

	// Build where filter for document_id
	var where map[string]interface{}
	if documentID != "" {
		where = map[string]interface{}{
			"document_id": documentID,
		}
	}

	// Get documents from ChromaDB
	result, err := r.client.GetDocuments(ctx, collectionName, where, limit, offset, false)
	if err != nil {
		return nil, 0, NewVectorRepositoryError("get_document_chunks", err, "failed to get chunks from ChromaDB")
	}

	// Convert to Chunk format
	chunks := make([]*Chunk, len(result.IDs))
	for i, id := range result.IDs {
		metadata := make(map[string]interface{})
		if i < len(result.Metadatas) {
			metadata = result.Metadatas[i]
		}

		text := ""
		if i < len(result.Documents) {
			text = result.Documents[i]
		}

		// Extract document_id from metadata
		docID := ""
		if d, ok := metadata["document_id"].(string); ok {
			docID = d
		}

		// Extract chunk_index from metadata
		chunkIndex := 0
		if ci, ok := metadata["chunk_index"].(float64); ok {
			chunkIndex = int(ci)
		}

		// Extract optional fields
		var pageNumber *int
		if pn, ok := metadata["page_number"].(float64); ok {
			p := int(pn)
			pageNumber = &p
		}

		var tokenCount *int
		if tc, ok := metadata["token_count"].(float64); ok {
			t := int(tc)
			tokenCount = &t
		}

		chunks[i] = &Chunk{
			ID:         id,
			DocumentID: docID,
			Text:       text,
			Metadata:   metadata,
			ChunkIndex: chunkIndex,
			PageNumber: pageNumber,
			TokenCount: tokenCount,
		}
	}

	// Get total count (without pagination)
	totalCount := len(chunks)
	if limit > 0 && len(chunks) == limit {
		// If we hit the limit, there might be more - get actual count
		// For now, just return what we have; a proper implementation would count all
		totalCount = len(chunks)
	}

	return chunks, totalCount, nil
}

// ListDocuments lists all unique documents in a collection
func (r *ChromaVectorRepository) ListDocuments(ctx context.Context, collectionName string) ([]*VectorDocument, error) {
	// Verify collection exists
	exists, err := r.CollectionExists(ctx, collectionName)
	if err != nil {
		return nil, NewVectorRepositoryError("list_documents", err, "")
	}
	if !exists {
		return nil, CollectionNotFoundError(collectionName)
	}

	// Fetch all chunks from the collection to aggregate by document_id
	result, err := r.client.GetDocuments(ctx, collectionName, nil, 0, 0, false)
	if err != nil {
		return nil, NewVectorRepositoryError("list_documents", err, "failed to get documents from ChromaDB")
	}

	// Aggregate by document_id
	docMap := make(map[string]*VectorDocument)
	for i, metadata := range result.Metadatas {
		docID := ""
		if d, ok := metadata["document_id"].(string); ok {
			docID = d
		}

		if docID == "" {
			continue
		}

		if doc, exists := docMap[docID]; exists {
			// Increment chunk count for existing document
			doc.ChunkCount++
		} else {
			// Create new document entry
			filename := ""
			if f, ok := metadata["filename"].(string); ok {
				filename = f
			}
			title := ""
			if t, ok := metadata["title"].(string); ok {
				title = t
			}

			docMap[docID] = &VectorDocument{
				DocumentID: docID,
				Filename:   filename,
				Title:      title,
				ChunkCount: 1,
				Collection: collectionName,
			}
		}

		// Avoid processing too many items (safety limit)
		if i > 10000 {
			break
		}
	}

	// Convert map to slice
	docs := make([]*VectorDocument, 0, len(docMap))
	for _, doc := range docMap {
		docs = append(docs, doc)
	}

	return docs, nil
}

// CountDocuments counts unique documents in a collection
func (r *ChromaVectorRepository) CountDocuments(ctx context.Context, collectionName string) (int, error) {
	docs, err := r.ListDocuments(ctx, collectionName)
	if err != nil {
		return 0, err
	}
	return len(docs), nil
}

// BatchStoreChunks stores chunks in batches for better performance
func (r *ChromaVectorRepository) BatchStoreChunks(ctx context.Context, collectionName string, batches [][]*Chunk) error {
	for i, batch := range batches {
		err := r.StoreChunks(ctx, collectionName, batch)
		if err != nil {
			return NewVectorRepositoryError("batch_store_chunks", err, fmt.Sprintf("failed at batch %d", i))
		}
	}
	return nil
}

// BatchDeleteChunks deletes chunks in batches
func (r *ChromaVectorRepository) BatchDeleteChunks(ctx context.Context, collectionName string, chunkIDs []string) error {
	// For now, just call DeleteChunks directly
	// Can be optimized later to split into actual batches if needed
	return r.DeleteChunks(ctx, collectionName, chunkIDs)
}

// Ping checks if ChromaDB is alive
func (r *ChromaVectorRepository) Ping(ctx context.Context) error {
	err := r.client.Heartbeat(ctx)
	if err != nil {
		return NewVectorRepositoryError("ping", err, "ChromaDB heartbeat failed")
	}
	return nil
}

// Close closes the ChromaDB client
func (r *ChromaVectorRepository) Close() error {
	r.client.Close()
	return nil
}
