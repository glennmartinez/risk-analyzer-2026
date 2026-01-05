package models

import (
	"time"
)

// Chunk represents a text chunk with embedding and metadata
type Chunk struct {
	ID         string                 `json:"id"`
	DocumentID string                 `json:"document_id"`
	Text       string                 `json:"text"`
	Embedding  []float32              `json:"embedding,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	ChunkIndex int                    `json:"chunk_index"`
	PageNumber *int                   `json:"page_number,omitempty"`
	TokenCount *int                   `json:"token_count,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// ChunkDTO represents the API view of a chunk
type ChunkDTO struct {
	ID         string                 `json:"id"`
	DocumentID string                 `json:"document_id"`
	Text       string                 `json:"text"`
	Embedding  []float32              `json:"embedding,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	ChunkIndex int                    `json:"chunk_index"`
	PageNumber *int                   `json:"page_number,omitempty"`
	TokenCount *int                   `json:"token_count,omitempty"`
	CreatedAt  string                 `json:"created_at"`
}

// ToDTO converts Chunk domain model to DTO
func (c *Chunk) ToDTO() ChunkDTO {
	return ChunkDTO{
		ID:         c.ID,
		DocumentID: c.DocumentID,
		Text:       c.Text,
		Embedding:  c.Embedding,
		Metadata:   c.Metadata,
		ChunkIndex: c.ChunkIndex,
		PageNumber: c.PageNumber,
		TokenCount: c.TokenCount,
		CreatedAt:  c.CreatedAt.Format(time.RFC3339),
	}
}

// ChunkFromDTO converts ChunkDTO to Chunk domain model
func ChunkFromDTO(dto ChunkDTO) (*Chunk, error) {
	createdAt, err := time.Parse(time.RFC3339, dto.CreatedAt)
	if err != nil {
		createdAt = time.Now()
	}

	return &Chunk{
		ID:         dto.ID,
		DocumentID: dto.DocumentID,
		Text:       dto.Text,
		Embedding:  dto.Embedding,
		Metadata:   dto.Metadata,
		ChunkIndex: dto.ChunkIndex,
		PageNumber: dto.PageNumber,
		TokenCount: dto.TokenCount,
		CreatedAt:  createdAt,
	}, nil
}

// Validate checks if the chunk is valid
func (c *Chunk) Validate() error {
	if c.ID == "" {
		return &ValidationError{Field: "id", Message: "chunk ID is required"}
	}
	if c.DocumentID == "" {
		return &ValidationError{Field: "document_id", Message: "document ID is required"}
	}
	if c.Text == "" {
		return &ValidationError{Field: "text", Message: "text is required"}
	}
	if c.ChunkIndex < 0 {
		return &ValidationError{Field: "chunk_index", Message: "chunk index cannot be negative"}
	}
	return nil
}

// SearchResult represents a single search result from vector similarity search
type SearchResult struct {
	ChunkID    string                 `json:"chunk_id"`
	DocumentID string                 `json:"document_id"`
	Text       string                 `json:"text"`
	Score      float32                `json:"score"`    // Similarity score (0-1, higher is better)
	Distance   float32                `json:"distance"` // Distance metric
	Metadata   map[string]interface{} `json:"metadata"`
	ChunkIndex int                    `json:"chunk_index"`
}

// SearchResultDTO represents the API view of a search result
type SearchResultDTO struct {
	ChunkID    string                 `json:"chunk_id"`
	DocumentID string                 `json:"document_id"`
	Text       string                 `json:"text"`
	Score      float32                `json:"score"`
	Distance   float32                `json:"distance"`
	Metadata   map[string]interface{} `json:"metadata"`
	ChunkIndex int                    `json:"chunk_index"`
	Filename   string                 `json:"filename,omitempty"`
	Collection string                 `json:"collection,omitempty"`
}

// ToDTO converts SearchResult to DTO
func (sr *SearchResult) ToDTO() SearchResultDTO {
	dto := SearchResultDTO{
		ChunkID:    sr.ChunkID,
		DocumentID: sr.DocumentID,
		Text:       sr.Text,
		Score:      sr.Score,
		Distance:   sr.Distance,
		Metadata:   sr.Metadata,
		ChunkIndex: sr.ChunkIndex,
	}

	// Extract filename and collection from metadata if available
	if sr.Metadata != nil {
		if filename, ok := sr.Metadata["filename"].(string); ok {
			dto.Filename = filename
		}
		if collection, ok := sr.Metadata["collection"].(string); ok {
			dto.Collection = collection
		}
	}

	return dto
}

// ChunkRequest represents a request to create or update chunks
type ChunkRequest struct {
	DocumentID string                   `json:"document_id"`
	Texts      []string                 `json:"texts"`
	Metadata   []map[string]interface{} `json:"metadata,omitempty"`
}

// Validate validates the chunk request
func (cr *ChunkRequest) Validate() error {
	if cr.DocumentID == "" {
		return &ValidationError{Field: "document_id", Message: "document ID is required"}
	}
	if len(cr.Texts) == 0 {
		return &ValidationError{Field: "texts", Message: "at least one text chunk is required"}
	}
	if cr.Metadata != nil && len(cr.Metadata) != len(cr.Texts) {
		return &ValidationError{Field: "metadata", Message: "metadata length must match texts length"}
	}
	return nil
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query          string                 `json:"query"`
	Collection     string                 `json:"collection"`
	TopK           int                    `json:"top_k"`
	Filter         map[string]interface{} `json:"filter,omitempty"`
	MinScore       *float32               `json:"min_score,omitempty"`
	IncludeContent bool                   `json:"include_content"`
}

// Validate validates the search request
func (sr *SearchRequest) Validate() error {
	if sr.Query == "" {
		return &ValidationError{Field: "query", Message: "query is required"}
	}
	if sr.Collection == "" {
		return &ValidationError{Field: "collection", Message: "collection is required"}
	}
	if sr.TopK <= 0 {
		sr.TopK = 10 // Default to 10 results
	}
	if sr.TopK > 100 {
		return &ValidationError{Field: "top_k", Message: "top_k cannot exceed 100"}
	}
	return nil
}

// SearchResponse represents a search response
type SearchResponse struct {
	Results    []SearchResultDTO `json:"results"`
	Query      string            `json:"query"`
	Collection string            `json:"collection"`
	TotalFound int               `json:"total_found"`
}
