package models

type Document struct {
	ID                string `json:"document_id"`
	RegisteredAt      string `json:"registered_at"`
	Filename          string `json:"filename"`
	Chunk_count       int    `json:"chunk_count"`
	Collection        string `json:"collection"`
	File_size         int    `json:"file_size"`
	Chunking_strategy string `json:"chunking_strategy"`
	Chunk_size        int    `json:"chunk_size"`
	Chunk_overlap     int    `json:"chunk_overlap"`
	Extract_metadata  bool   `json:"extract_metadata"`
	Num_questions     int    `json:"num_questions"`
	Max_pages         int    `json:"max_pages"`
	Llm_provider      string `json:"llm_provider"`
	Llm_model         string `json:"llm_model"`
}

// create the from and to dto functions for Document

func (d *Document) ToDTO() DocumentDTO {
	return DocumentDTO{
		ID:                d.ID,
		RegisteredAt:      d.RegisteredAt,
		Filename:          d.Filename,
		Chunk_count:       d.Chunk_count,
		Collection:        d.Collection,
		File_size:         d.File_size,
		Chunking_strategy: d.Chunking_strategy,
		Chunk_size:        d.Chunk_size,
		Chunk_overlap:     d.Chunk_overlap,
		Extract_metadata:  d.Extract_metadata,
		Num_questions:     d.Num_questions,
		Max_pages:         d.Max_pages,
		Llm_provider:      d.Llm_provider,
		Llm_model:         d.Llm_model,
	}
}

func DocumentFromDTO(dto DocumentDTO) Document {
	return Document{
		ID:                dto.ID,
		RegisteredAt:      dto.RegisteredAt,
		Filename:          dto.Filename,
		Chunk_count:       dto.Chunk_count,
		Collection:        dto.Collection,
		File_size:         dto.File_size,
		Chunking_strategy: dto.Chunking_strategy,
		Chunk_size:        dto.Chunk_size,
		Chunk_overlap:     dto.Chunk_overlap,
		Extract_metadata:  dto.Extract_metadata,
		Num_questions:     dto.Num_questions,
		Max_pages:         dto.Max_pages,
		Llm_provider:      dto.Llm_provider,
		Llm_model:         dto.Llm_model,
	}
}

// DocumentDTO - API Request/Response (what clients see)
type DocumentDTO struct {
	ID                string `json:"document_id"`
	RegisteredAt      string `json:"registered_at"`
	Filename          string `json:"filename"`
	Chunk_count       int    `json:"chunk_count"`
	Collection        string `json:"collection"`
	File_size         int    `json:"file_size"`
	Chunking_strategy string `json:"chunking_strategy"`
	Chunk_size        int    `json:"chunk_size"`
	Chunk_overlap     int    `json:"chunk_overlap"`
	Extract_metadata  bool   `json:"extract_metadata"`
	Num_questions     int    `json:"num_questions"`
	Max_pages         int    `json:"max_pages"`
	Llm_provider      string `json:"llm_provider"`
	Llm_model         string `json:"llm_model"`
}
