package repositories

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes
	documentKeyPrefix  = "document:"
	documentIndexKey   = "documents:index"
	collectionIndexKey = "collection:"
	filenameIndexKey   = "filename:"
	statusIndexKey     = "status:"
)

// RedisDocumentRepository implements DocumentRepository using Redis
type RedisDocumentRepository struct {
	client *redis.Client
}

// NewRedisDocumentRepository creates a new Redis-based document repository
func NewRedisDocumentRepository(client *redis.Client) *RedisDocumentRepository {
	return &RedisDocumentRepository{
		client: client,
	}
}

// Register stores a new document in the registry
func (r *RedisDocumentRepository) Register(ctx context.Context, doc *Document) error {
	if err := doc.Validate(); err != nil {
		return err
	}

	// Check if document already exists
	exists, err := r.Exists(ctx, doc.ID)
	if err != nil {
		return NewDocumentRepositoryError("register", doc.ID, err, "")
	}
	if exists {
		return DocumentAlreadyExistsError(doc.ID)
	}

	// Set timestamps
	now := time.Now()
	doc.CreatedAt = now
	doc.UpdatedAt = now

	// Use transaction to ensure atomicity
	pipe := r.client.TxPipeline()

	// Serialize document to JSON
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return NewDocumentRepositoryError("register", doc.ID, err, "failed to marshal document")
	}

	// Store document
	docKey := documentKeyPrefix + doc.ID
	pipe.Set(ctx, docKey, docJSON, 0)

	// Add to global index
	pipe.SAdd(ctx, documentIndexKey, doc.ID)

	// Add to collection index
	collKey := collectionIndexKey + doc.Collection
	pipe.SAdd(ctx, collKey, doc.ID)

	// Add to filename index
	filenameKey := filenameIndexKey + doc.Filename
	pipe.Set(ctx, filenameKey, doc.ID, 0)

	// Add to status index
	statusKey := statusIndexKey + string(doc.Status)
	pipe.SAdd(ctx, statusKey, doc.ID)

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return NewDocumentRepositoryError("register", doc.ID, err, "failed to execute transaction")
	}

	return nil
}

// Get retrieves a document by ID
func (r *RedisDocumentRepository) Get(ctx context.Context, documentID string) (*Document, error) {
	docKey := documentKeyPrefix + documentID

	docJSON, err := r.client.Get(ctx, docKey).Result()
	if err == redis.Nil {
		return nil, DocumentNotFoundError(documentID)
	}
	if err != nil {
		return nil, NewDocumentRepositoryError("get", documentID, err, "")
	}

	var doc Document
	if err := json.Unmarshal([]byte(docJSON), &doc); err != nil {
		return nil, NewDocumentRepositoryError("get", documentID, err, "failed to unmarshal document")
	}

	return &doc, nil
}

// List retrieves all documents
func (r *RedisDocumentRepository) List(ctx context.Context) ([]*Document, error) {
	// Get all document IDs from index
	docIDs, err := r.client.SMembers(ctx, documentIndexKey).Result()
	if err != nil {
		return nil, NewDocumentRepositoryError("list", "", err, "")
	}

	if len(docIDs) == 0 {
		return []*Document{}, nil
	}

	return r.GetBatch(ctx, docIDs)
}

// Delete removes a document from the registry
func (r *RedisDocumentRepository) Delete(ctx context.Context, documentID string) error {
	// First get the document to access its metadata for index cleanup
	doc, err := r.Get(ctx, documentID)
	if err != nil {
		return err
	}

	// Use transaction to ensure atomicity
	pipe := r.client.TxPipeline()

	// Delete document
	docKey := documentKeyPrefix + documentID
	pipe.Del(ctx, docKey)

	// Remove from global index
	pipe.SRem(ctx, documentIndexKey, documentID)

	// Remove from collection index
	collKey := collectionIndexKey + doc.Collection
	pipe.SRem(ctx, collKey, documentID)

	// Remove from filename index
	filenameKey := filenameIndexKey + doc.Filename
	pipe.Del(ctx, filenameKey)

	// Remove from old status index
	statusKey := statusIndexKey + string(doc.Status)
	pipe.SRem(ctx, statusKey, documentID)

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return NewDocumentRepositoryError("delete", documentID, err, "failed to execute transaction")
	}

	return nil
}

// Update modifies document fields
func (r *RedisDocumentRepository) Update(ctx context.Context, documentID string, updates map[string]interface{}) error {
	// Get existing document
	doc, err := r.Get(ctx, documentID)
	if err != nil {
		return err
	}

	oldStatus := doc.Status
	oldCollection := doc.Collection

	// Apply updates
	for key, value := range updates {
		switch key {
		case "filename":
			if v, ok := value.(string); ok {
				doc.Filename = v
			}
		case "collection":
			if v, ok := value.(string); ok {
				doc.Collection = v
			}
		case "chunk_count":
			if v, ok := value.(int); ok {
				doc.ChunkCount = v
			} else if v, ok := value.(float64); ok {
				doc.ChunkCount = int(v)
			}
		case "file_size":
			if v, ok := value.(int64); ok {
				doc.FileSize = v
			} else if v, ok := value.(float64); ok {
				doc.FileSize = int64(v)
			}
		case "status":
			if v, ok := value.(string); ok {
				doc.Status = DocumentStatus(v)
			} else if v, ok := value.(DocumentStatus); ok {
				doc.Status = v
			}
		case "stored_in_vector_db":
			if v, ok := value.(bool); ok {
				doc.StoredInVectorDB = v
			}
		case "metadata":
			if v, ok := value.(map[string]interface{}); ok {
				doc.Metadata = v
			}
		case "chunking_strategy":
			if v, ok := value.(string); ok {
				doc.ChunkingStrategy = v
			}
		case "chunk_size":
			if v, ok := value.(int); ok {
				doc.ChunkSize = v
			} else if v, ok := value.(float64); ok {
				doc.ChunkSize = int(v)
			}
		case "chunk_overlap":
			if v, ok := value.(int); ok {
				doc.ChunkOverlap = v
			} else if v, ok := value.(float64); ok {
				doc.ChunkOverlap = int(v)
			}
		case "extract_metadata":
			if v, ok := value.(bool); ok {
				doc.ExtractMetadata = v
			}
		case "num_questions":
			if v, ok := value.(int); ok {
				doc.NumQuestions = v
			} else if v, ok := value.(float64); ok {
				doc.NumQuestions = int(v)
			}
		case "max_pages":
			if v, ok := value.(int); ok {
				doc.MaxPages = v
			} else if v, ok := value.(float64); ok {
				doc.MaxPages = int(v)
			}
		case "llm_provider":
			if v, ok := value.(string); ok {
				doc.LLMProvider = v
			}
		case "llm_model":
			if v, ok := value.(string); ok {
				doc.LLMModel = v
			}
		}
	}

	// Update timestamp
	doc.UpdatedAt = time.Now()

	// Validate updated document
	if err := doc.Validate(); err != nil {
		return err
	}

	// Use transaction
	pipe := r.client.TxPipeline()

	// Serialize updated document
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return NewDocumentRepositoryError("update", documentID, err, "failed to marshal document")
	}

	// Update document
	docKey := documentKeyPrefix + documentID
	pipe.Set(ctx, docKey, docJSON, 0)

	// Update indexes if collection or status changed
	if oldCollection != doc.Collection {
		oldCollKey := collectionIndexKey + oldCollection
		newCollKey := collectionIndexKey + doc.Collection
		pipe.SRem(ctx, oldCollKey, documentID)
		pipe.SAdd(ctx, newCollKey, documentID)
	}

	if oldStatus != doc.Status {
		oldStatusKey := statusIndexKey + string(oldStatus)
		newStatusKey := statusIndexKey + string(doc.Status)
		pipe.SRem(ctx, oldStatusKey, documentID)
		pipe.SAdd(ctx, newStatusKey, documentID)
	}

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return NewDocumentRepositoryError("update", documentID, err, "failed to execute transaction")
	}

	return nil
}

// Exists checks if a document exists
func (r *RedisDocumentRepository) Exists(ctx context.Context, documentID string) (bool, error) {
	docKey := documentKeyPrefix + documentID
	exists, err := r.client.Exists(ctx, docKey).Result()
	if err != nil {
		return false, NewDocumentRepositoryError("exists", documentID, err, "")
	}
	return exists > 0, nil
}

// RegisterBatch registers multiple documents atomically
func (r *RedisDocumentRepository) RegisterBatch(ctx context.Context, docs []*Document) error {
	if len(docs) == 0 {
		return nil
	}

	// Validate all documents first
	for _, doc := range docs {
		if err := doc.Validate(); err != nil {
			return err
		}
	}

	now := time.Now()
	pipe := r.client.TxPipeline()

	for _, doc := range docs {
		// Set timestamps
		doc.CreatedAt = now
		doc.UpdatedAt = now

		// Serialize document
		docJSON, err := json.Marshal(doc)
		if err != nil {
			return NewDocumentRepositoryError("register_batch", doc.ID, err, "failed to marshal document")
		}

		// Store document
		docKey := documentKeyPrefix + doc.ID
		pipe.Set(ctx, docKey, docJSON, 0)

		// Add to indexes
		pipe.SAdd(ctx, documentIndexKey, doc.ID)
		pipe.SAdd(ctx, collectionIndexKey+doc.Collection, doc.ID)
		pipe.Set(ctx, filenameIndexKey+doc.Filename, doc.ID, 0)
		pipe.SAdd(ctx, statusIndexKey+string(doc.Status), doc.ID)
	}

	// Execute transaction
	_, err := pipe.Exec(ctx)
	if err != nil {
		return NewDocumentRepositoryError("register_batch", "", err, "failed to execute batch transaction")
	}

	return nil
}

// GetBatch retrieves multiple documents by IDs
func (r *RedisDocumentRepository) GetBatch(ctx context.Context, documentIDs []string) ([]*Document, error) {
	if len(documentIDs) == 0 {
		return []*Document{}, nil
	}

	// Build keys
	keys := make([]string, len(documentIDs))
	for i, id := range documentIDs {
		keys[i] = documentKeyPrefix + id
	}

	// Use pipeline for batch get
	pipe := r.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, NewDocumentRepositoryError("get_batch", "", err, "failed to execute batch get")
	}

	// Parse results
	docs := make([]*Document, 0, len(documentIDs))
	for i, cmd := range cmds {
		docJSON, err := cmd.Result()
		if err == redis.Nil {
			// Skip missing documents
			continue
		}
		if err != nil {
			return nil, NewDocumentRepositoryError("get_batch", documentIDs[i], err, "")
		}

		var doc Document
		if err := json.Unmarshal([]byte(docJSON), &doc); err != nil {
			return nil, NewDocumentRepositoryError("get_batch", documentIDs[i], err, "failed to unmarshal document")
		}
		docs = append(docs, &doc)
	}

	return docs, nil
}

// DeleteBatch deletes multiple documents atomically
func (r *RedisDocumentRepository) DeleteBatch(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}

	// Get all documents first to clean up indexes
	docs, err := r.GetBatch(ctx, documentIDs)
	if err != nil {
		return err
	}

	pipe := r.client.TxPipeline()

	for _, doc := range docs {
		// Delete document
		docKey := documentKeyPrefix + doc.ID
		pipe.Del(ctx, docKey)

		// Remove from indexes
		pipe.SRem(ctx, documentIndexKey, doc.ID)
		pipe.SRem(ctx, collectionIndexKey+doc.Collection, doc.ID)
		pipe.Del(ctx, filenameIndexKey+doc.Filename)
		pipe.SRem(ctx, statusIndexKey+string(doc.Status), doc.ID)
	}

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return NewDocumentRepositoryError("delete_batch", "", err, "failed to execute batch delete")
	}

	return nil
}

// ListByCollection retrieves all documents in a collection
func (r *RedisDocumentRepository) ListByCollection(ctx context.Context, collection string) ([]*Document, error) {
	collKey := collectionIndexKey + collection
	docIDs, err := r.client.SMembers(ctx, collKey).Result()
	if err != nil {
		return nil, NewDocumentRepositoryError("list_by_collection", "", err, "")
	}

	if len(docIDs) == 0 {
		return []*Document{}, nil
	}

	return r.GetBatch(ctx, docIDs)
}

// ListByStatus retrieves all documents with a specific status
func (r *RedisDocumentRepository) ListByStatus(ctx context.Context, status DocumentStatus) ([]*Document, error) {
	statusKey := statusIndexKey + string(status)
	docIDs, err := r.client.SMembers(ctx, statusKey).Result()
	if err != nil {
		return nil, NewDocumentRepositoryError("list_by_status", "", err, "")
	}

	if len(docIDs) == 0 {
		return []*Document{}, nil
	}

	return r.GetBatch(ctx, docIDs)
}

// CountByCollection counts documents in a collection
func (r *RedisDocumentRepository) CountByCollection(ctx context.Context, collection string) (int, error) {
	collKey := collectionIndexKey + collection
	count, err := r.client.SCard(ctx, collKey).Result()
	if err != nil {
		return 0, NewDocumentRepositoryError("count_by_collection", "", err, "")
	}
	return int(count), nil
}

// CountTotal counts all documents
func (r *RedisDocumentRepository) CountTotal(ctx context.Context) (int, error) {
	count, err := r.client.SCard(ctx, documentIndexKey).Result()
	if err != nil {
		return 0, NewDocumentRepositoryError("count_total", "", err, "")
	}
	return int(count), nil
}

// FindByFilename finds a document by filename
func (r *RedisDocumentRepository) FindByFilename(ctx context.Context, filename string) (*Document, error) {
	filenameKey := filenameIndexKey + filename
	documentID, err := r.client.Get(ctx, filenameKey).Result()
	if err == redis.Nil {
		return nil, DocumentNotFoundError("filename:" + filename)
	}
	if err != nil {
		return nil, NewDocumentRepositoryError("find_by_filename", "", err, "")
	}

	return r.Get(ctx, documentID)
}

// FilterByMetadata filters documents by metadata (simple implementation)
func (r *RedisDocumentRepository) FilterByMetadata(ctx context.Context, filter map[string]interface{}) ([]*Document, error) {
	// Get all documents
	allDocs, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	// Filter in memory (for simple cases)
	filtered := make([]*Document, 0)
	for _, doc := range allDocs {
		if doc.Metadata == nil {
			continue
		}

		matches := true
		for key, value := range filter {
			docValue, exists := doc.Metadata[key]
			if !exists || docValue != value {
				matches = false
				break
			}
		}

		if matches {
			filtered = append(filtered, doc)
		}
	}

	return filtered, nil
}

// Ping checks if Redis connection is alive
func (r *RedisDocumentRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisDocumentRepository) Close() error {
	return r.client.Close()
}

// Cleanup removes old documents based on age
func (r *RedisDocumentRepository) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	// Get all documents
	allDocs, err := r.List(ctx)
	if err != nil {
		return 0, err
	}

	// Find documents to delete
	cutoff := time.Now().Add(-olderThan)
	toDelete := make([]string, 0)

	for _, doc := range allDocs {
		if doc.CreatedAt.Before(cutoff) && doc.Status == DocumentStatusDeleted {
			toDelete = append(toDelete, doc.ID)
		}
	}

	// Delete old documents
	if len(toDelete) > 0 {
		if err := r.DeleteBatch(ctx, toDelete); err != nil {
			return 0, err
		}
	}

	return len(toDelete), nil
}

// GetStats returns statistics about documents (helper method)
func (r *RedisDocumentRepository) GetStats(ctx context.Context) (*DocumentStats, error) {
	allDocs, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	stats := &DocumentStats{
		TotalDocuments:        len(allDocs),
		DocumentsByStatus:     make(map[DocumentStatus]int),
		DocumentsByCollection: make(map[string]int),
		TotalChunks:           0,
		TotalSize:             0,
	}

	for _, doc := range allDocs {
		stats.DocumentsByStatus[doc.Status]++
		stats.DocumentsByCollection[doc.Collection]++
		stats.TotalChunks += doc.ChunkCount
		stats.TotalSize += doc.FileSize
	}

	if len(allDocs) > 0 {
		stats.AverageChunkCount = float64(stats.TotalChunks) / float64(len(allDocs))
	}

	return stats, nil
}

// ListCollections returns all unique collection names
func (r *RedisDocumentRepository) ListCollections(ctx context.Context) ([]string, error) {
	// Use pattern matching to find all collection index keys
	pattern := collectionIndexKey + "*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, NewDocumentRepositoryError("list_collections", "", err, "")
	}

	// Extract collection names from keys
	collections := make([]string, 0, len(keys))
	prefix := collectionIndexKey
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			collName := strings.TrimPrefix(key, prefix)
			if collName != "" {
				collections = append(collections, collName)
			}
		}
	}

	// Sort for consistent ordering
	sort.Strings(collections)
	return collections, nil
}

// ClearCollection removes all documents from a collection
func (r *RedisDocumentRepository) ClearCollection(ctx context.Context, collection string) error {
	docs, err := r.ListByCollection(ctx, collection)
	if err != nil {
		return err
	}

	if len(docs) == 0 {
		return nil
	}

	docIDs := make([]string, len(docs))
	for i, doc := range docs {
		docIDs[i] = doc.ID
	}

	return r.DeleteBatch(ctx, docIDs)
}

// GetCollectionStats returns statistics for a specific collection
func (r *RedisDocumentRepository) GetCollectionStats(ctx context.Context, collection string) (map[string]interface{}, error) {
	docs, err := r.ListByCollection(ctx, collection)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_documents": len(docs),
		"total_chunks":    0,
		"total_size":      int64(0),
		"statuses":        make(map[DocumentStatus]int),
	}

	totalChunks := 0
	var totalSize int64
	statuses := make(map[DocumentStatus]int)

	for _, doc := range docs {
		totalChunks += doc.ChunkCount
		totalSize += doc.FileSize
		statuses[doc.Status]++
	}

	stats["total_chunks"] = totalChunks
	stats["total_size"] = totalSize
	stats["statuses"] = statuses

	if len(docs) > 0 {
		stats["average_chunks"] = float64(totalChunks) / float64(len(docs))
	}

	return stats, nil
}
