package models

import (
	"time"
)

// Document represents a document in the system with all metadata
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

// DocumentDTO - API Request/Response (what clients see)
type DocumentDTO struct {
	ID               string                 `json:"document_id"`
	Filename         string                 `json:"filename"`
	Collection       string                 `json:"collection"`
	ChunkCount       int                    `json:"chunk_count"`
	FileSize         int64                  `json:"file_size,omitempty"`
	Status           string                 `json:"status"`
	StoredInVectorDB bool                   `json:"stored_in_vector_db"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	ChunkingStrategy string                 `json:"chunking_strategy,omitempty"`
	ChunkSize        int                    `json:"chunk_size,omitempty"`
	ChunkOverlap     int                    `json:"chunk_overlap,omitempty"`
	ExtractMetadata  bool                   `json:"extract_metadata,omitempty"`
	NumQuestions     int                    `json:"num_questions,omitempty"`
	MaxPages         int                    `json:"max_pages,omitempty"`
	LLMProvider      string                 `json:"llm_provider,omitempty"`
	LLMModel         string                 `json:"llm_model,omitempty"`
}

// ToDTO converts Document domain model to DTO
func (d *Document) ToDTO() DocumentDTO {
	return DocumentDTO{
		ID:               d.ID,
		Filename:         d.Filename,
		Collection:       d.Collection,
		ChunkCount:       d.ChunkCount,
		FileSize:         d.FileSize,
		Status:           string(d.Status),
		StoredInVectorDB: d.StoredInVectorDB,
		CreatedAt:        d.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        d.UpdatedAt.Format(time.RFC3339),
		Metadata:         d.Metadata,
		ChunkingStrategy: d.ChunkingStrategy,
		ChunkSize:        d.ChunkSize,
		ChunkOverlap:     d.ChunkOverlap,
		ExtractMetadata:  d.ExtractMetadata,
		NumQuestions:     d.NumQuestions,
		MaxPages:         d.MaxPages,
		LLMProvider:      d.LLMProvider,
		LLMModel:         d.LLMModel,
	}
}

// FromDTO converts DocumentDTO to Document domain model
func DocumentFromDTO(dto DocumentDTO) (*Document, error) {
	createdAt, err := time.Parse(time.RFC3339, dto.CreatedAt)
	if err != nil {
		createdAt = time.Now()
	}

	updatedAt, err := time.Parse(time.RFC3339, dto.UpdatedAt)
	if err != nil {
		updatedAt = time.Now()
	}

	return &Document{
		ID:               dto.ID,
		Filename:         dto.Filename,
		Collection:       dto.Collection,
		ChunkCount:       dto.ChunkCount,
		FileSize:         dto.FileSize,
		Status:           DocumentStatus(dto.Status),
		StoredInVectorDB: dto.StoredInVectorDB,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		Metadata:         dto.Metadata,
		ChunkingStrategy: dto.ChunkingStrategy,
		ChunkSize:        dto.ChunkSize,
		ChunkOverlap:     dto.ChunkOverlap,
		ExtractMetadata:  dto.ExtractMetadata,
		NumQuestions:     dto.NumQuestions,
		MaxPages:         dto.MaxPages,
		LLMProvider:      dto.LLMProvider,
		LLMModel:         dto.LLMModel,
	}, nil
}

// Validate checks if the document is valid
func (d *Document) Validate() error {
	if d.ID == "" {
		return &ValidationError{Field: "document_id", Message: "document ID is required"}
	}
	if d.Filename == "" {
		return &ValidationError{Field: "filename", Message: "filename is required"}
	}
	if d.Collection == "" {
		return &ValidationError{Field: "collection", Message: "collection is required"}
	}
	if d.ChunkCount < 0 {
		return &ValidationError{Field: "chunk_count", Message: "chunk count cannot be negative"}
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

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

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
