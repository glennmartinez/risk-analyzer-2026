package repositories

import (
	"context"
	"time"
)

// DocumentRepository defines the interface for document registry operations
// This abstracts Redis operations for document metadata storage
type DocumentRepository interface {
	// Document Registry Operations
	Register(ctx context.Context, doc *Document) error
	Get(ctx context.Context, documentID string) (*Document, error)
	List(ctx context.Context) ([]*Document, error)
	Delete(ctx context.Context, documentID string) error
	Update(ctx context.Context, documentID string, updates map[string]interface{}) error
	Exists(ctx context.Context, documentID string) (bool, error)

	// Bulk Operations
	RegisterBatch(ctx context.Context, docs []*Document) error
	GetBatch(ctx context.Context, documentIDs []string) ([]*Document, error)
	DeleteBatch(ctx context.Context, documentIDs []string) error

	// Query Operations
	ListByCollection(ctx context.Context, collection string) ([]*Document, error)
	ListByStatus(ctx context.Context, status DocumentStatus) ([]*Document, error)
	CountByCollection(ctx context.Context, collection string) (int, error)
	CountTotal(ctx context.Context) (int, error)

	// Search and Filter
	FindByFilename(ctx context.Context, filename string) (*Document, error)
	FilterByMetadata(ctx context.Context, filter map[string]interface{}) ([]*Document, error)

	// Health and Cleanup
	Ping(ctx context.Context) error
	Close() error
	Cleanup(ctx context.Context, olderThan time.Duration) (int, error)
}

// Document represents a document in the registry
type Document struct {
	ID               string                 `json:"document_id"`
	Filename         string                 `json:"filename"`
	Collection       string                 `json:"collection"`
	ChunkCount       int                    `json:"chunk_count"`
	FileSize         int64                  `json:"file_size,omitempty"`
	Status           DocumentStatus         `json:"status"`
	StoredInVectorDB bool                   `json:"stored_in_vector_db"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`

	// Processing metadata
	ChunkingStrategy string `json:"chunking_strategy,omitempty"`
	ChunkSize        int    `json:"chunk_size,omitempty"`
	ChunkOverlap     int    `json:"chunk_overlap,omitempty"`
	ExtractMetadata  bool   `json:"extract_metadata,omitempty"`
	NumQuestions     int    `json:"num_questions,omitempty"`
	MaxPages         int    `json:"max_pages,omitempty"`

	// LLM metadata
	LLMProvider string `json:"llm_provider,omitempty"`
	LLMModel    string `json:"llm_model,omitempty"`
}

// DocumentStatus represents the status of a document
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusCompleted  DocumentStatus = "completed"
	DocumentStatusFailed     DocumentStatus = "failed"
	DocumentStatusDeleted    DocumentStatus = "deleted"
)

// DocumentFilter represents filter criteria for document queries
type DocumentFilter struct {
	Collection       string
	Status           DocumentStatus
	StoredInVectorDB *bool
	FilenamePattern  string
	CreatedAfter     *time.Time
	CreatedBefore    *time.Time
	MinChunkCount    *int
	MaxChunkCount    *int
}

// DocumentStats represents statistics about documents
type DocumentStats struct {
	TotalDocuments        int                    `json:"total_documents"`
	DocumentsByStatus     map[DocumentStatus]int `json:"documents_by_status"`
	DocumentsByCollection map[string]int         `json:"documents_by_collection"`
	TotalChunks           int                    `json:"total_chunks"`
	TotalSize             int64                  `json:"total_size"`
	AverageChunkCount     float64                `json:"average_chunk_count"`
}

// DocumentRepositoryError represents errors from the document repository
type DocumentRepositoryError struct {
	Operation  string
	DocumentID string
	Err        error
	Message    string
}

func (e *DocumentRepositoryError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	prefix := e.Operation
	if e.DocumentID != "" {
		prefix += " (doc: " + e.DocumentID + ")"
	}
	if e.Err != nil {
		return prefix + ": " + e.Err.Error()
	}
	return prefix + ": unknown error"
}

func (e *DocumentRepositoryError) Unwrap() error {
	return e.Err
}

// NewDocumentRepositoryError creates a new document repository error
func NewDocumentRepositoryError(operation string, documentID string, err error, message string) *DocumentRepositoryError {
	return &DocumentRepositoryError{
		Operation:  operation,
		DocumentID: documentID,
		Err:        err,
		Message:    message,
	}
}

// Common error constructors
func DocumentNotFoundError(documentID string) error {
	return NewDocumentRepositoryError(
		"get_document",
		documentID,
		nil,
		"document not found: "+documentID,
	)
}

func DocumentAlreadyExistsError(documentID string) error {
	return NewDocumentRepositoryError(
		"register_document",
		documentID,
		nil,
		"document already exists: "+documentID,
	)
}

func InvalidDocumentError(documentID string, reason string) error {
	return NewDocumentRepositoryError(
		"validate_document",
		documentID,
		nil,
		"invalid document: "+reason,
	)
}

// Validation helpers
func (d *Document) Validate() error {
	if d.ID == "" {
		return InvalidDocumentError("", "document ID is required")
	}
	if d.Filename == "" {
		return InvalidDocumentError(d.ID, "filename is required")
	}
	if d.Collection == "" {
		return InvalidDocumentError(d.ID, "collection is required")
	}
	if d.ChunkCount < 0 {
		return InvalidDocumentError(d.ID, "chunk count cannot be negative")
	}
	return nil
}

// IsValid checks if document status is valid
func (s DocumentStatus) IsValid() bool {
	switch s {
	case DocumentStatusPending, DocumentStatusProcessing, DocumentStatusCompleted, DocumentStatusFailed, DocumentStatusDeleted:
		return true
	default:
		return false
	}
}

// String returns the string representation of document status
func (s DocumentStatus) String() string {
	return string(s)
}
