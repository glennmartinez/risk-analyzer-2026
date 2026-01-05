package models

import (
	"time"
)

// Collection represents a vector database collection
type Collection struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// CollectionDTO represents the API view of a collection
type CollectionDTO struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// ToDTO converts Collection domain model to DTO
func (c *Collection) ToDTO() CollectionDTO {
	return CollectionDTO{
		ID:        c.ID,
		Name:      c.Name,
		Metadata:  c.Metadata,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
	}
}

// CollectionFromDTO converts CollectionDTO to Collection domain model
func CollectionFromDTO(dto CollectionDTO) (*Collection, error) {
	createdAt, err := time.Parse(time.RFC3339, dto.CreatedAt)
	if err != nil {
		createdAt = time.Now()
	}

	updatedAt, err := time.Parse(time.RFC3339, dto.UpdatedAt)
	if err != nil {
		updatedAt = time.Now()
	}

	return &Collection{
		ID:        dto.ID,
		Name:      dto.Name,
		Metadata:  dto.Metadata,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// Validate checks if the collection is valid
func (c *Collection) Validate() error {
	if c.Name == "" {
		return &ValidationError{Field: "name", Message: "collection name is required"}
	}
	return nil
}

// CollectionInfo represents metadata about a collection
type CollectionInfo struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CollectionStats represents statistics for a collection
type CollectionStats struct {
	Name          string                 `json:"name"`
	DocumentCount int                    `json:"document_count"`
	ChunkCount    int                    `json:"chunk_count"`
	TotalSize     int64                  `json:"total_size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at,omitempty"`
	UpdatedAt     time.Time              `json:"updated_at,omitempty"`
}

// CollectionStatsDTO represents the API view of collection statistics
type CollectionStatsDTO struct {
	Name          string                 `json:"name"`
	DocumentCount int                    `json:"document_count"`
	ChunkCount    int                    `json:"chunk_count"`
	TotalSize     int64                  `json:"total_size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     string                 `json:"created_at,omitempty"`
	UpdatedAt     string                 `json:"updated_at,omitempty"`
}

// ToDTO converts CollectionStats to DTO
func (cs *CollectionStats) ToDTO() CollectionStatsDTO {
	dto := CollectionStatsDTO{
		Name:          cs.Name,
		DocumentCount: cs.DocumentCount,
		ChunkCount:    cs.ChunkCount,
		TotalSize:     cs.TotalSize,
		Metadata:      cs.Metadata,
	}

	if !cs.CreatedAt.IsZero() {
		dto.CreatedAt = cs.CreatedAt.Format(time.RFC3339)
	}
	if !cs.UpdatedAt.IsZero() {
		dto.UpdatedAt = cs.UpdatedAt.Format(time.RFC3339)
	}

	return dto
}

// CollectionRequest represents a request to create a collection
type CollectionRequest struct {
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Validate validates the collection request
func (cr *CollectionRequest) Validate() error {
	if cr.Name == "" {
		return &ValidationError{Field: "name", Message: "collection name is required"}
	}
	// Collection names should be alphanumeric with hyphens and underscores
	for _, char := range cr.Name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return &ValidationError{
				Field:   "name",
				Message: "collection name must contain only alphanumeric characters, hyphens, and underscores",
			}
		}
	}
	if len(cr.Name) > 100 {
		return &ValidationError{Field: "name", Message: "collection name cannot exceed 100 characters"}
	}
	return nil
}

// CollectionListResponse represents a response with a list of collections
type CollectionListResponse struct {
	Collections []CollectionDTO `json:"collections"`
	Total       int             `json:"total"`
}

// VectorDocument represents a document stored in the vector database
type VectorDocument struct {
	DocumentID string    `json:"document_id"`
	Filename   string    `json:"filename"`
	Title      string    `json:"title,omitempty"`
	ChunkCount int       `json:"chunk_count"`
	Collection string    `json:"collection"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

// VectorDocumentDTO represents the API view of a vector document
type VectorDocumentDTO struct {
	DocumentID string `json:"document_id"`
	Filename   string `json:"filename"`
	Title      string `json:"title,omitempty"`
	ChunkCount int    `json:"chunk_count"`
	Collection string `json:"collection"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// ToDTO converts VectorDocument to DTO
func (vd *VectorDocument) ToDTO() VectorDocumentDTO {
	dto := VectorDocumentDTO{
		DocumentID: vd.DocumentID,
		Filename:   vd.Filename,
		Title:      vd.Title,
		ChunkCount: vd.ChunkCount,
		Collection: vd.Collection,
	}

	if !vd.CreatedAt.IsZero() {
		dto.CreatedAt = vd.CreatedAt.Format(time.RFC3339)
	}

	return dto
}
