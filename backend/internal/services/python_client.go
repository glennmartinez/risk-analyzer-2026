package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// PythonClientInterface defines the interface for Python backend communication
type PythonClientInterface interface {
	ParseDocument(ctx context.Context, fileData []byte, filename string, extractMetadata bool, maxPages int) (*ParseResponse, error)
	ParseText(ctx context.Context, text string) (*ParseResponse, error)
	Chunk(ctx context.Context, req *ChunkRequest) (*ChunkResponse, error)
	ChunkSimple(ctx context.Context, text string, chunkSize, chunkOverlap int) (*ChunkResponse, error)
	Embed(ctx context.Context, text string, model *string, useCache bool) (*EmbeddingResponse, error)
	EmbedBatch(ctx context.Context, texts []string, model *string, batchSize int, useCache bool) (*EmbedBatchResponse, error)
	EmbedQuery(ctx context.Context, text string, model *string, useCache bool) (*EmbeddingResponse, error)
	ExtractMetadata(ctx context.Context, req *MetadataRequest) (*MetadataResponse, error)
	ExtractTitle(ctx context.Context, text string) (*string, error)
	ExtractKeywords(ctx context.Context, text string, numKeywords int) ([]string, error)
	ExtractQuestions(ctx context.Context, text string, numQuestions int) ([]string, error)
	HealthCheck(ctx context.Context, service string) (bool, error)
	GetAvailableModels(ctx context.Context) ([]map[string]interface{}, error)
	GetChunkingStrategies(ctx context.Context) ([]string, error)
}

// PythonClient handles communication with the Python backend compute endpoints
type PythonClient struct {
	baseURL    string
	httpClient *http.Client
	retries    int
}

// NewPythonClient creates a new Python client with default settings
func NewPythonClient(baseURL string) *PythonClient {
	return &PythonClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		retries: 3,
	}
}

// NewPythonClientWithOptions creates a client with custom settings
func NewPythonClientWithOptions(baseURL string, timeout time.Duration, retries int) *PythonClient {
	return &PythonClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		retries: retries,
	}
}

// ============================================================================
// Request/Response Models
// ============================================================================

// ParseResponse represents the response from the parse endpoint
type ParseResponse struct {
	Text             string                 `json:"text"`
	Markdown         string                 `json:"markdown,omitempty"`
	Metadata         map[string]interface{} `json:"metadata"`
	Pages            []interface{}          `json:"pages"`
	Tables           []interface{}          `json:"tables"`
	Figures          []interface{}          `json:"figures"`
	ExtractionMethod string                 `json:"extraction_method"`
	TotalPages       int                    `json:"total_pages"`
}

// ChunkRequest represents a request to chunk text
type ChunkRequest struct {
	Text            string `json:"text"`
	Strategy        string `json:"strategy"`
	ChunkSize       int    `json:"chunk_size"`
	ChunkOverlap    int    `json:"chunk_overlap"`
	ExtractMetadata bool   `json:"extract_metadata"`
	NumQuestions    int    `json:"num_questions"`
}

// ChunkMetadata represents metadata for a single chunk
type ChunkMetadata struct {
	ChunkIndex int      `json:"chunk_index"`
	Title      *string  `json:"title,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
	Questions  []string `json:"questions,omitempty"`
	TokenCount *int     `json:"token_count,omitempty"`
}

// TextChunk represents a single text chunk
type TextChunk struct {
	Text     string         `json:"text"`
	Index    int            `json:"index"`
	Metadata *ChunkMetadata `json:"metadata,omitempty"`
}

// ChunkResponse represents the response from the chunk endpoint
type ChunkResponse struct {
	Chunks       []TextChunk `json:"chunks"`
	TotalChunks  int         `json:"total_chunks"`
	StrategyUsed string      `json:"strategy_used"`
	ChunkSize    int         `json:"chunk_size"`
	ChunkOverlap int         `json:"chunk_overlap"`
}

// EmbedRequest represents a request to generate embeddings
type EmbedRequest struct {
	Texts     []string `json:"texts"`
	Model     *string  `json:"model,omitempty"`
	BatchSize int      `json:"batch_size"`
	UseCache  bool     `json:"use_cache"`
}

// EmbedSingleRequest represents a request for single text embedding
type EmbedSingleRequest struct {
	Text     string  `json:"text"`
	Model    *string `json:"model,omitempty"`
	UseCache bool    `json:"use_cache"`
}

// EmbeddingResponse represents the response from embed/text endpoint
type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
	Dimension int       `json:"dimension"`
	Model     string    `json:"model"`
	Cached    bool      `json:"cached"`
}

// EmbedBatchResponse represents the response from embed/batch endpoint
type EmbedBatchResponse struct {
	Embeddings      [][]float32 `json:"embeddings"`
	Dimension       int         `json:"dimension"`
	Model           string      `json:"model"`
	TotalEmbeddings int         `json:"total_embeddings"`
	CachedCount     int         `json:"cached_count"`
}

// MetadataRequest represents a request to extract metadata
type MetadataRequest struct {
	Text             string `json:"text"`
	ExtractTitle     bool   `json:"extract_title"`
	ExtractKeywords  bool   `json:"extract_keywords"`
	ExtractQuestions bool   `json:"extract_questions"`
	NumQuestions     int    `json:"num_questions"`
	NumKeywords      int    `json:"num_keywords"`
}

// MetadataResponse represents the response from metadata endpoint
type MetadataResponse struct {
	Title     *string                `json:"title,omitempty"`
	Keywords  []string               `json:"keywords,omitempty"`
	Questions []string               `json:"questions,omitempty"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ============================================================================
// HTTP Helper Methods
// ============================================================================

// doRequest performs an HTTP request with retry logic
func (c *PythonClient) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.makeRequest(ctx, method, endpoint, body)
		if err == nil && resp.StatusCode < 500 {
			// Success or client error (don't retry 4xx)
			return resp, nil
		}

		lastErr = err
		if resp != nil {
			resp.Body.Close()
		}
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.retries, lastErr)
}

// makeRequest creates and executes an HTTP request
func (c *PythonClient) makeRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	url := c.baseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// parseResponse reads and parses JSON response
func parseResponse(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// ============================================================================
// Parse Methods
// ============================================================================

// ParseDocument parses a document file and returns extracted text
func (c *PythonClient) ParseDocument(ctx context.Context, fileData []byte, filename string, extractMetadata bool, maxPages int) (*ParseResponse, error) {
	url := c.baseURL + "/parse/document"

	// Execute with retry
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			time.Sleep(backoff)
		}

		// Create fresh multipart form data for each attempt (body gets consumed)
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add file
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}
		if _, err := io.Copy(part, bytes.NewReader(fileData)); err != nil {
			return nil, fmt.Errorf("failed to write file data: %w", err)
		}

		// Add form fields
		if err := writer.WriteField("extract_metadata", fmt.Sprintf("%t", extractMetadata)); err != nil {
			return nil, err
		}
		if err := writer.WriteField("max_pages", fmt.Sprintf("%d", maxPages)); err != nil {
			return nil, err
		}

		if err := writer.Close(); err != nil {
			return nil, err
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, "POST", url, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode < 500 {
			var result ParseResponse
			if err := parseResponse(resp, &result); err != nil {
				return nil, err
			}
			return &result, nil
		}

		resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil, fmt.Errorf("parse document failed after retries: %w", lastErr)
}

// ParseText parses plain text content
func (c *PythonClient) ParseText(ctx context.Context, text string) (*ParseResponse, error) {
	req := map[string]interface{}{
		"text": text,
	}

	resp, err := c.doRequest(ctx, "POST", "/parse/text", req)
	if err != nil {
		return nil, fmt.Errorf("parse text request failed: %w", err)
	}

	var result ParseResponse
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ============================================================================
// Chunk Methods
// ============================================================================

// Chunk chunks text into smaller pieces using specified strategy
func (c *PythonClient) Chunk(ctx context.Context, req *ChunkRequest) (*ChunkResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/chunk/text", req)
	if err != nil {
		return nil, fmt.Errorf("chunk request failed: %w", err)
	}

	var result ChunkResponse
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ChunkSimple performs simple text chunking with minimal parameters
func (c *PythonClient) ChunkSimple(ctx context.Context, text string, chunkSize, chunkOverlap int) (*ChunkResponse, error) {
	req := map[string]interface{}{
		"text":          text,
		"chunk_size":    chunkSize,
		"chunk_overlap": chunkOverlap,
	}

	resp, err := c.doRequest(ctx, "POST", "/chunk/simple", req)
	if err != nil {
		return nil, fmt.Errorf("chunk simple request failed: %w", err)
	}

	var result ChunkResponse
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ============================================================================
// Embed Methods
// ============================================================================

// Embed generates embeddings for a single text
func (c *PythonClient) Embed(ctx context.Context, text string, model *string, useCache bool) (*EmbeddingResponse, error) {
	req := &EmbedSingleRequest{
		Text:     text,
		Model:    model,
		UseCache: useCache,
	}

	resp, err := c.doRequest(ctx, "POST", "/embed/text", req)
	if err != nil {
		return nil, fmt.Errorf("embed request failed: %w", err)
	}

	var result EmbeddingResponse
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// EmbedBatch generates embeddings for multiple texts in batch
func (c *PythonClient) EmbedBatch(ctx context.Context, texts []string, model *string, batchSize int, useCache bool) (*EmbedBatchResponse, error) {
	if batchSize <= 0 {
		batchSize = 32 // Default batch size
	}

	req := &EmbedRequest{
		Texts:     texts,
		Model:     model,
		BatchSize: batchSize,
		UseCache:  useCache,
	}

	resp, err := c.doRequest(ctx, "POST", "/embed/batch", req)
	if err != nil {
		return nil, fmt.Errorf("embed batch request failed: %w", err)
	}

	var result EmbedBatchResponse
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// EmbedQuery generates embedding for a search query
func (c *PythonClient) EmbedQuery(ctx context.Context, query string, model *string, useCache bool) (*EmbeddingResponse, error) {
	req := map[string]interface{}{
		"query":     query,
		"use_cache": useCache,
	}
	if model != nil {
		req["model"] = *model
	}

	resp, err := c.doRequest(ctx, "POST", "/embed/query", req)
	if err != nil {
		return nil, fmt.Errorf("embed query request failed: %w", err)
	}

	var result EmbeddingResponse
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ============================================================================
// Metadata Methods
// ============================================================================

// ExtractMetadata extracts metadata from text (title, keywords, questions)
func (c *PythonClient) ExtractMetadata(ctx context.Context, req *MetadataRequest) (*MetadataResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/metadata/extract", req)
	if err != nil {
		return nil, fmt.Errorf("extract metadata request failed: %w", err)
	}

	var result MetadataResponse
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ExtractTitle extracts only the title from text
func (c *PythonClient) ExtractTitle(ctx context.Context, text string) (*string, error) {
	req := map[string]interface{}{
		"text": text,
	}

	resp, err := c.doRequest(ctx, "POST", "/metadata/title", req)
	if err != nil {
		return nil, fmt.Errorf("extract title request failed: %w", err)
	}

	var result struct {
		Title *string `json:"title"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Title, nil
}

// ExtractKeywords extracts keywords from text
func (c *PythonClient) ExtractKeywords(ctx context.Context, text string, numKeywords int) ([]string, error) {
	req := map[string]interface{}{
		"text":         text,
		"num_keywords": numKeywords,
	}

	resp, err := c.doRequest(ctx, "POST", "/metadata/keywords", req)
	if err != nil {
		return nil, fmt.Errorf("extract keywords request failed: %w", err)
	}

	var result struct {
		Keywords []string `json:"keywords"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Keywords, nil
}

// ExtractQuestions extracts questions that the text answers
func (c *PythonClient) ExtractQuestions(ctx context.Context, text string, numQuestions int) ([]string, error) {
	req := map[string]interface{}{
		"text":          text,
		"num_questions": numQuestions,
	}

	resp, err := c.doRequest(ctx, "POST", "/metadata/questions", req)
	if err != nil {
		return nil, fmt.Errorf("extract questions request failed: %w", err)
	}

	var result struct {
		Questions []string `json:"questions"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Questions, nil
}

// ============================================================================
// Health Check Methods
// ============================================================================

// HealthCheck checks if a specific service is healthy
func (c *PythonClient) HealthCheck(ctx context.Context, service string) (bool, error) {
	endpoint := fmt.Sprintf("/%s/health", service)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	status, ok := result["status"].(string)
	return ok && status == "healthy", nil
}

// GetAvailableModels gets list of available embedding models
func (c *PythonClient) GetAvailableModels(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, "GET", "/embed/models", nil)
	if err != nil {
		return nil, fmt.Errorf("get models request failed: %w", err)
	}

	var result struct {
		Models []map[string]interface{} `json:"models"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Models, nil
}

// GetChunkingStrategies gets list of available chunking strategies
func (c *PythonClient) GetChunkingStrategies(ctx context.Context) ([]string, error) {
	resp, err := c.doRequest(ctx, "GET", "/chunk/strategies", nil)
	if err != nil {
		return nil, fmt.Errorf("get strategies request failed: %w", err)
	}

	var result struct {
		Strategies []string `json:"strategies"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Strategies, nil
}
