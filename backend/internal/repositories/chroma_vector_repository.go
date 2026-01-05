package repositories

import (
	"context"
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

		// Merge custom metadata
		for k, v := range chunk.Metadata {
			metadata[k] = v
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

	// Note: ChromaDB's current Go client doesn't have direct support for deleting by metadata filter
	// We would need to:
	// 1. Query all chunks with document_id filter to get their IDs
	// 2. Delete those chunks by ID
	// This is not implemented in current client version

	// For now, return 0 and error indicating this needs implementation
	// This will be implemented once we have better ChromaDB client support or direct API calls
	return 0, NewVectorRepositoryError("delete_document", nil, "delete by document_id not yet implemented - use DeleteChunks with explicit chunk IDs")
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

	// Note: This requires querying all chunks and aggregating by document_id
	// With current ChromaDB Go client, this is challenging
	// Return empty list for now - will implement when client supports better metadata queries
	return []*VectorDocument{}, nil
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
