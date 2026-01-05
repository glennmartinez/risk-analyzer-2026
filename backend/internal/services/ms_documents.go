package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"risk-analyzer/internal/models"
	"strconv"
	"time"
)

const (
	MICROSERVICE_BASE_URL = "http://localhost:8000"
)

type DocumentService struct {
	baseURL    string
	httpClient *http.Client
}

func NewDocumentService() *DocumentService {
	return &DocumentService{
		baseURL: MICROSERVICE_BASE_URL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ListDocumentsResponse represents the response from the Python backend
type ListDocumentsResponse struct {
	Documents []models.Document `json:"documents"`
	Total     int               `json:"total"`
}

// ListDocuments fetches all documents from the Python microservice
func (s *DocumentService) ListDocuments(ctx context.Context) (*ListDocumentsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/documents/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Python backend: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Python backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var response ListDocumentsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// ListVectorDocumentsResponse represents the response from the vector store endpoint
type ListVectorDocumentsResponse struct {
	Documents  []VectorDocument `json:"documents"`
	Total      int              `json:"total"`
	Collection string           `json:"collection"`
}

// VectorDocument represents a document in the vector store
type VectorDocument struct {
	DocumentID string `json:"document_id"`
	Filename   string `json:"filename"`
	Title      string `json:"title"`
	ChunkCount int    `json:"chunk_count"`
	Collection string `json:"collection"`
}

// ListVectorDocuments fetches all documents from the vector store via Python microservice
func (s *DocumentService) ListVectorDocuments(ctx context.Context, collectionName string) (*ListVectorDocumentsResponse, error) {
	url := s.baseURL + "/documents/vector"
	if collectionName != "" {
		url += "?collection_name=" + collectionName
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Python backend: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Python backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var response ListVectorDocumentsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// HealthCheck verifies the Python microservice is running
func (s *DocumentService) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Python microservice not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Python microservice returned status %d", resp.StatusCode)
	}

	return nil
}

// UploadDocumentRequest contains options for document upload
type UploadDocumentRequest struct {
	ChunkingStrategy string
	ChunkSize        int
	ChunkOverlap     int
	StoreInVectorDB  bool
	ExtractTables    bool
	ExtractFigures   bool
	ExtractMetadata  bool
	NumQuestions     int
	MaxPages         int
	CollectionName   string
}

// UploadDocumentResponse represents the response from document upload
type UploadDocumentResponse struct {
	DocumentID       string                 `json:"document_id"`
	Filename         string                 `json:"filename"`
	Status           string                 `json:"status"`
	TotalChunks      int                    `json:"total_chunks"`
	ProcessingTimeMs float64                `json:"processing_time_ms"`
	VectorDBStored   bool                   `json:"vector_db_stored"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// DefaultUploadRequest returns default upload options
func DefaultUploadRequest() UploadDocumentRequest {
	return UploadDocumentRequest{
		ChunkingStrategy: "sentence",
		ChunkSize:        512,
		ChunkOverlap:     50,
		StoreInVectorDB:  true,
		ExtractTables:    true,
		ExtractFigures:   true,
		ExtractMetadata:  false,
		NumQuestions:     3,
		MaxPages:         30,
		CollectionName:   "",
	}
}

// UploadDocument uploads a file to the Python microservice for processing
func (s *DocumentService) UploadDocument(ctx context.Context, filename string, fileContent io.Reader, opts UploadDocumentRequest) (*UploadDocumentResponse, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, fileContent); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	// Add form fields
	writer.WriteField("chunking_strategy", opts.ChunkingStrategy)
	writer.WriteField("chunk_size", strconv.Itoa(opts.ChunkSize))
	writer.WriteField("chunk_overlap", strconv.Itoa(opts.ChunkOverlap))
	writer.WriteField("store_in_vector_db", strconv.FormatBool(opts.StoreInVectorDB))
	writer.WriteField("extract_tables", strconv.FormatBool(opts.ExtractTables))
	writer.WriteField("extract_figures", strconv.FormatBool(opts.ExtractFigures))
	writer.WriteField("extract_metadata", strconv.FormatBool(opts.ExtractMetadata))
	writer.WriteField("num_questions", strconv.Itoa(opts.NumQuestions))
	writer.WriteField("max_pages", strconv.Itoa(opts.MaxPages))
	if opts.CollectionName != "" {
		writer.WriteField("collection_name", opts.CollectionName)
	}

	writer.Close()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/documents/upload", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Python backend: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Python backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var response UploadDocumentResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DocumentChunk represents a single chunk from the vector store
type DocumentChunk struct {
	ID       string                 `json:"id"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata"`
}

// GetDocumentChunksResponse represents the response from the chunks endpoint
type GetDocumentChunksResponse struct {
	Chunks []DocumentChunk `json:"chunks"`
	Count  int             `json:"count"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// GetDocumentChunks fetches chunks for a specific document from the vector store
func (s *DocumentService) GetDocumentChunks(ctx context.Context, documentID string, collectionName string, limit int, offset int) (*GetDocumentChunksResponse, error) {
	// Build URL with query parameters
	url := fmt.Sprintf("%s/documents/chunks?document_id=%s", s.baseURL, documentID)
	if collectionName != "" {
		url += "&collection_name=" + collectionName
	}
	if limit > 0 {
		url += fmt.Sprintf("&limit=%d", limit)
	}
	if offset > 0 {
		url += fmt.Sprintf("&offset=%d", offset)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Python backend: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Python backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var response GetDocumentChunksResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteDocumentResponse represents the response from deleting a document
type DeleteDocumentResponse struct {
	Success             bool   `json:"success"`
	DocumentID          string `json:"document_id"`
	DeletedChunks       int    `json:"deleted_chunks"`
	DeletedFromRegistry bool   `json:"deleted_from_registry"`
}

// DeleteDocument deletes a document from both vector store and Redis
func (s *DocumentService) DeleteDocument(ctx context.Context, documentID string, collectionName string) (*DeleteDocumentResponse, error) {
	url := fmt.Sprintf("%s/documents/%s", s.baseURL, documentID)
	if collectionName != "" {
		url += "?collection_name=" + collectionName
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Python backend: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Python backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var response DeleteDocumentResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteCollectionResponse represents the response from deleting a collection
type DeleteCollectionResponse struct {
	Success                      bool   `json:"success"`
	CollectionName               string `json:"collection_name"`
	DocumentsRemovedFromRegistry int    `json:"documents_removed_from_registry"`
	TotalDocuments               int    `json:"total_documents"`
}

// DeleteCollection deletes an entire collection from vector store and cleans up Redis
func (s *DocumentService) DeleteCollection(ctx context.Context, collectionName string) (*DeleteCollectionResponse, error) {
	url := fmt.Sprintf("%s/documents/collection/%s", s.baseURL, collectionName)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Python backend: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Python backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var response DeleteCollectionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
