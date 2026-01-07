package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ChromaDBClient wraps HTTP calls to ChromaDB v2 API
// This avoids compatibility issues with the official Go client library
type ChromaDBClient struct {
	baseURL    string
	httpClient *http.Client
	tenant     string
	database   string
}

// ChromaDBConfig holds configuration for ChromaDB connection
type ChromaDBConfig struct {
	Host     string
	Port     int
	Tenant   string // default: "default_tenant"
	Database string // default: "default_database"
	Timeout  time.Duration
}

// Collection represents a ChromaDB collection
type Collection struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

// CollectionStats represents statistics for a collection
type CollectionStats struct {
	Name     string                 `json:"name"`
	Count    int                    `json:"count"`
	Metadata map[string]interface{} `json:"metadata"`
}

// GetResponse represents the response from a get request
type GetResponse struct {
	IDs        []string                 `json:"ids"`
	Documents  []string                 `json:"documents"`
	Metadatas  []map[string]interface{} `json:"metadatas"`
	Embeddings [][]float32              `json:"embeddings,omitempty"`
}

// NewChromaDBClient creates a new ChromaDB client with v2 API support
func NewChromaDBClient(config ChromaDBConfig) *ChromaDBClient {
	if config.Tenant == "" {
		config.Tenant = "default_tenant"
	}
	if config.Database == "" {
		config.Database = "default_database"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// ChromaDB v2 API uses tenant and database in the path
	baseURL := fmt.Sprintf("http://%s:%d/api/v2/tenants/%s/databases/%s",
		config.Host, config.Port, config.Tenant, config.Database)

	return &ChromaDBClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		tenant:   config.Tenant,
		database: config.Database,
	}
}

// Heartbeat checks if ChromaDB is alive
func (c *ChromaDBClient) Heartbeat(ctx context.Context) error {
	// Extract host:port from baseURL and use global heartbeat endpoint
	// baseURL format: http://host:port/api/v2/tenants/X/databases/Y
	parts := c.baseURL[7:] // Remove "http://"
	hostPort := parts[:len(parts)-len("/api/v2/tenants/")-len(c.tenant)-len("/databases/")-len(c.database)]
	heartbeatURL := fmt.Sprintf("http://%s/api/v2/heartbeat", hostPort)
	req, err := http.NewRequestWithContext(ctx, "GET", heartbeatURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("heartbeat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}

	return nil
}

// ListCollections returns all collections
func (c *ChromaDBClient) ListCollections(ctx context.Context) ([]Collection, error) {
	url := fmt.Sprintf("%s/collections", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list collections failed (status %d): %s", resp.StatusCode, string(body))
	}

	var collections []Collection
	if err := json.NewDecoder(resp.Body).Decode(&collections); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return collections, nil
}

// CreateCollection creates a new collection
func (c *ChromaDBClient) CreateCollection(ctx context.Context, name string, metadata map[string]interface{}) (*Collection, error) {
	if metadata == nil {
		metadata = map[string]interface{}{
			"hnsw:space": "cosine",
		}
	}

	payload := map[string]interface{}{
		"name":     name,
		"metadata": metadata,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create collection failed (status %d): %s", resp.StatusCode, string(body))
	}

	var collection Collection
	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &collection, nil
}

// GetCollection retrieves a collection by name
func (c *ChromaDBClient) GetCollection(ctx context.Context, name string) (*Collection, error) {
	url := fmt.Sprintf("%s/collections/%s", c.baseURL, name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("collection not found: %s", name)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get collection failed (status %d): %s", resp.StatusCode, string(body))
	}

	var collection Collection
	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &collection, nil
}

// DeleteCollection deletes a collection
func (c *ChromaDBClient) DeleteCollection(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/collections/%s", c.baseURL, name)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete collection failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// CountCollection returns the number of documents in a collection
func (c *ChromaDBClient) CountCollection(ctx context.Context, name string) (int, error) {
	collection, err := c.GetCollection(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("failed to get collection: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/count", c.baseURL, collection.ID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("count collection failed (status %d): %s", resp.StatusCode, string(body))
	}

	var count int
	if err := json.NewDecoder(resp.Body).Decode(&count); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return count, nil
}

// AddDocuments adds documents to a collection
func (c *ChromaDBClient) AddDocuments(ctx context.Context, collectionName string, ids []string, documents []string, embeddings [][]float32, metadatas []map[string]interface{}) error {
	collection, err := c.GetCollection(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("failed to get collection: %w", err)
	}

	payload := map[string]interface{}{
		"ids":        ids,
		"documents":  documents,
		"embeddings": embeddings,
	}

	if metadatas != nil {
		payload["metadatas"] = metadatas
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/add", c.baseURL, collection.ID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add documents failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// QueryResponse represents the response from a query
type QueryResponse struct {
	IDs       [][]string                 `json:"ids"`
	Documents [][]string                 `json:"documents"`
	Metadatas [][]map[string]interface{} `json:"metadatas"`
	Distances [][]float32                `json:"distances"`
}

// Query searches for similar documents
func (c *ChromaDBClient) Query(ctx context.Context, collectionName string, queryEmbeddings [][]float32, nResults int, where map[string]interface{}) (*QueryResponse, error) {
	collection, err := c.GetCollection(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	payload := map[string]interface{}{
		"query_embeddings": queryEmbeddings,
		"n_results":        nResults,
	}

	if where != nil {
		payload["where"] = where
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/query", c.baseURL, collection.ID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query failed (status %d): %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &queryResp, nil
}

// DeleteDocuments deletes documents from a collection by IDs
func (c *ChromaDBClient) DeleteDocuments(ctx context.Context, collectionName string, ids []string) error {
	collection, err := c.GetCollection(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("failed to get collection: %w", err)
	}

	payload := map[string]interface{}{
		"ids": ids,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/delete", c.baseURL, collection.ID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete documents failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetDocuments retrieves documents from a collection with optional filtering
func (c *ChromaDBClient) GetDocuments(ctx context.Context, collectionName string, where map[string]interface{}, limit int, offset int, includeEmbeddings bool) (*GetResponse, error) {
	collection, err := c.GetCollection(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	payload := map[string]interface{}{
		"include": []string{"documents", "metadatas"},
	}

	if includeEmbeddings {
		payload["include"] = []string{"documents", "metadatas", "embeddings"}
	}

	if where != nil && len(where) > 0 {
		payload["where"] = where
	}

	if limit > 0 {
		payload["limit"] = limit
	} else {
		// Default to fetching all documents (use a large limit)
		payload["limit"] = 100000
	}

	if offset > 0 {
		payload["offset"] = offset
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/get", c.baseURL, collection.ID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get documents failed (status %d): %s", resp.StatusCode, string(body))
	}

	var getResp GetResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &getResp, nil
}

// Close closes the HTTP client connections
func (c *ChromaDBClient) Close() {
	c.httpClient.CloseIdleConnections()
}
