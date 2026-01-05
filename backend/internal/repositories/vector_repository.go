package repositories

import (
	"context"
)

// VectorRepository defines the interface for vector database operations
// This abstracts ChromaDB operations and allows for easy testing and implementation swapping
type VectorRepository interface {
	// Collection Management
	CreateCollection(ctx context.Context, name string, metadata map[string]interface{}) error
	DeleteCollection(ctx context.Context, name string) error
	GetCollection(ctx context.Context, name string) (*CollectionInfo, error)
	ListCollections(ctx context.Context) ([]string, error)
	GetCollectionStats(ctx context.Context, name string) (*CollectionStats, error)
	CollectionExists(ctx context.Context, name string) (bool, error)

	// Document Operations
	StoreChunks(ctx context.Context, collectionName string, chunks []*Chunk) error
	SearchChunks(ctx context.Context, collectionName string, queryEmbedding []float32, topK int, filter map[string]interface{}) ([]*SearchResult, error)
	DeleteDocument(ctx context.Context, collectionName string, documentID string) (int, error)
	DeleteChunks(ctx context.Context, collectionName string, chunkIDs []string) error
	GetChunk(ctx context.Context, collectionName string, chunkID string) (*Chunk, error)
	ListDocuments(ctx context.Context, collectionName string) ([]*VectorDocument, error)
	CountDocuments(ctx context.Context, collectionName string) (int, error)

	// Batch Operations
	BatchStoreChunks(ctx context.Context, collectionName string, batches [][]*Chunk) error
	BatchDeleteChunks(ctx context.Context, collectionName string, chunkIDs []string) error

	// Health and Stats
	Ping(ctx context.Context) error
	Close() error
}

// CollectionInfo represents metadata about a collection
type CollectionInfo struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

// CollectionStats represents statistics for a collection
type CollectionStats struct {
	Name          string                 `json:"name"`
	DocumentCount int                    `json:"document_count"`
	ChunkCount    int                    `json:"chunk_count"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// Chunk represents a text chunk with embedding and metadata
type Chunk struct {
	ID         string                 `json:"id"`
	DocumentID string                 `json:"document_id"`
	Text       string                 `json:"text"`
	Embedding  []float32              `json:"embedding"`
	Metadata   map[string]interface{} `json:"metadata"`
	ChunkIndex int                    `json:"chunk_index"`
	PageNumber *int                   `json:"page_number,omitempty"`
	TokenCount *int                   `json:"token_count,omitempty"`
}

// SearchResult represents a single search result from vector similarity search
type SearchResult struct {
	ChunkID    string                 `json:"chunk_id"`
	DocumentID string                 `json:"document_id"`
	Text       string                 `json:"text"`
	Score      float32                `json:"score"` // Similarity score (0-1, higher is better)
	Distance   float32                `json:"distance"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// VectorDocument represents a document stored in the vector database
type VectorDocument struct {
	DocumentID string `json:"document_id"`
	Filename   string `json:"filename"`
	Title      string `json:"title,omitempty"`
	ChunkCount int    `json:"chunk_count"`
	Collection string `json:"collection"`
}

// BatchChunkRequest represents a batch of chunks to store
type BatchChunkRequest struct {
	CollectionName string   `json:"collection_name"`
	Chunks         []*Chunk `json:"chunks"`
	BatchSize      int      `json:"batch_size"` // Number of chunks per batch (default: 100)
}

// SearchOptions represents options for vector search
type SearchOptions struct {
	TopK             int                    `json:"top_k"`
	Filter           map[string]interface{} `json:"filter,omitempty"`
	IncludeMetadata  bool                   `json:"include_metadata"`
	IncludeDistances bool                   `json:"include_distances"`
	MinScore         *float32               `json:"min_score,omitempty"`
	CollectionName   string                 `json:"collection_name"`
}

// VectorRepositoryError represents errors from the vector repository
type VectorRepositoryError struct {
	Operation string
	Err       error
	Message   string
}

func (e *VectorRepositoryError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Operation + ": " + e.Err.Error()
	}
	return e.Operation + ": unknown error"
}

func (e *VectorRepositoryError) Unwrap() error {
	return e.Err
}

// NewVectorRepositoryError creates a new vector repository error
func NewVectorRepositoryError(operation string, err error, message string) *VectorRepositoryError {
	return &VectorRepositoryError{
		Operation: operation,
		Err:       err,
		Message:   message,
	}
}

// Common error constructors
func CollectionNotFoundError(name string) error {
	return NewVectorRepositoryError(
		"get_collection",
		nil,
		"collection not found: "+name,
	)
}

func CollectionAlreadyExistsError(name string) error {
	return NewVectorRepositoryError(
		"create_collection",
		nil,
		"collection already exists: "+name,
	)
}

func VectorDocumentNotFoundError(documentID string) error {
	return NewVectorRepositoryError(
		"get_document",
		nil,
		"document not found: "+documentID,
	)
}

func ChunkNotFoundError(chunkID string) error {
	return NewVectorRepositoryError(
		"get_chunk",
		nil,
		"chunk not found: "+chunkID,
	)
}
