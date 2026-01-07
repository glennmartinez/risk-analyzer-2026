// Package integration_test contains integration tests that verify the full document upload flow
// including Redis (document metadata) and ChromaDB (chunks/embeddings) storage.
//
// Prerequisites:
// - Redis running on localhost:6379
// - ChromaDB running on localhost:8000
// - Python backend running on localhost:8001
// - Go server running on localhost:8080
//
// Run with: go test -v ./internal/integration_test/... -tags=integration
//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	goServerURL     = "http://localhost:8080"
	pythonServerURL = "http://localhost:8001" // Python backend port
	chromaDBURL     = "http://localhost:8000" // ChromaDB port
	redisAddr       = "localhost:6379"

	testCollectionName = "integration_test_collection"
	testTimeout        = 120 * time.Second
)

// Environment variables that may override defaults:
// - PYTHON_BACKEND_URL: URL of Python backend (default: http://localhost:8000)
// - CHROMA_HOST/CHROMA_PORT: ChromaDB connection (default: localhost:8000)
// - REDIS_HOST/REDIS_PORT: Redis connection (default: localhost:6379)
//
// Make sure Go server's PYTHON_BACKEND_URL matches where Python is running!

// DocumentUploadResponse represents the response from document upload
type DocumentUploadResponse struct {
	DocumentID       string                 `json:"document_id"`
	JobID            string                 `json:"job_id,omitempty"`
	Filename         string                 `json:"filename"`
	Collection       string                 `json:"collection"`
	ChunkCount       int                    `json:"chunk_count"`
	Status           string                 `json:"status"`
	ProcessingTimeMs int64                  `json:"processing_time_ms,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Message          string                 `json:"message,omitempty"`
}

// DocumentStatusResponse represents the response from document status endpoint
type DocumentStatusResponse struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
	Progress   int    `json:"progress,omitempty"`
	Message    string `json:"message,omitempty"`
	JobID      string `json:"job_id,omitempty"`
}

// RedisDocument represents document data stored in Redis
type RedisDocument struct {
	ID               string                 `json:"document_id"`
	Filename         string                 `json:"filename"`
	Collection       string                 `json:"collection"`
	ChunkCount       int                    `json:"chunk_count"`
	FileSize         int64                  `json:"file_size"`
	Status           string                 `json:"status"`
	StoredInVectorDB bool                   `json:"stored_in_vector_db"`
	ChunkingStrategy string                 `json:"chunking_strategy"`
	ChunkSize        int                    `json:"chunk_size"`
	ChunkOverlap     int                    `json:"chunk_overlap"`
	ExtractMetadata  bool                   `json:"extract_metadata"`
	NumQuestions     int                    `json:"num_questions"`
	MaxPages         int                    `json:"max_pages"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ChromaGetResponse represents the response from ChromaDB get endpoint
type ChromaGetResponse struct {
	IDs       []string                 `json:"ids"`
	Documents []string                 `json:"documents"`
	Metadatas []map[string]interface{} `json:"metadatas"`
}

// CollectionResponse represents ChromaDB collection info
type CollectionResponse struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

// TestDocumentUploadIntegration tests the full document upload flow
func TestDocumentUploadIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Setup: Check all services are running
	t.Log("Checking prerequisites...")
	checkServices(t, ctx)

	// Setup: Create test file
	testFilePath := createTestFile(t)
	defer os.Remove(testFilePath)

	// Use unique collection name for this test to avoid conflicts
	syncTestCollection := testCollectionName + "_sync"

	// Setup: Ensure clean state - delete test collection if exists
	t.Log("Cleaning up test collection...")
	cleanupCollection(t, ctx, syncTestCollection)

	// Setup: Create test collection
	t.Log("Creating test collection...")
	createCollection(t, ctx, syncTestCollection)
	defer cleanupCollection(t, ctx, syncTestCollection)

	// Test parameters
	uploadParams := map[string]string{
		"collection":        syncTestCollection,
		"chunking_strategy": "sentence",
		"chunk_size":        "256",
		"chunk_overlap":     "25",
		"extract_metadata":  "false",
		"num_questions":     "2",
		"max_pages":         "5",
		"async":             "false", // Synchronous for easier testing
	}

	// Step 1: Upload document via Go API
	t.Log("Step 1: Uploading document via Go API...")
	uploadResp := uploadDocument(t, ctx, testFilePath, uploadParams)

	t.Logf("Upload response: DocumentID=%s, Status=%s, ChunkCount=%d",
		uploadResp.DocumentID, uploadResp.Status, uploadResp.ChunkCount)

	if uploadResp.DocumentID == "" {
		t.Fatal("Expected document ID in response")
	}

	// Step 2: Verify document metadata in Redis
	t.Log("Step 2: Verifying document metadata in Redis...")
	redisDoc := verifyRedisDocument(t, ctx, uploadResp.DocumentID, uploadParams)

	t.Logf("Redis document: ID=%s, Filename=%s, Collection=%s, ChunkCount=%d, Status=%s",
		redisDoc.ID, redisDoc.Filename, redisDoc.Collection, redisDoc.ChunkCount, redisDoc.Status)

	// Step 3: Verify chunks in ChromaDB
	t.Log("Step 3: Verifying chunks in ChromaDB...")
	chunks := verifyChromaDBChunks(t, ctx, uploadResp.DocumentID, syncTestCollection)

	t.Logf("ChromaDB chunks: Found %d chunks for document %s", len(chunks.IDs), uploadResp.DocumentID)

	// Step 4: Verify 1:1 mapping between Redis chunk count and ChromaDB chunks
	t.Log("Step 4: Verifying 1:1 mapping between Redis and ChromaDB...")
	if redisDoc.ChunkCount != len(chunks.IDs) {
		t.Errorf("Chunk count mismatch: Redis has %d, ChromaDB has %d",
			redisDoc.ChunkCount, len(chunks.IDs))
	}

	// Step 5: Verify chunk metadata contains document_id
	t.Log("Step 5: Verifying chunk metadata...")
	for i, metadata := range chunks.Metadatas {
		docID, ok := metadata["document_id"].(string)
		if !ok || docID != uploadResp.DocumentID {
			t.Errorf("Chunk %d has incorrect document_id: expected %s, got %v",
				i, uploadResp.DocumentID, metadata["document_id"])
		}

		// Verify other expected metadata fields
		if _, ok := metadata["chunk_index"]; !ok {
			t.Errorf("Chunk %d missing chunk_index metadata", i)
		}
		if _, ok := metadata["filename"]; !ok {
			t.Logf("Warning: Chunk %d missing filename metadata (non-critical)", i)
		}
	}

	// Step 6: Test document retrieval via API
	t.Log("Step 6: Testing document retrieval via API...")
	verifyDocumentAPI(t, ctx, uploadResp.DocumentID)

	// Step 7: Test chunks retrieval via API
	t.Log("Step 7: Testing chunks retrieval via API...")
	verifyChunksAPI(t, ctx, uploadResp.DocumentID, len(chunks.IDs))

	// Step 8: Test document deletion (verifies both Redis and ChromaDB cleanup)
	t.Log("Step 8: Testing document deletion...")
	deleteDocument(t, ctx, uploadResp.DocumentID)

	// Step 9: Verify document is deleted from Redis
	t.Log("Step 9: Verifying document deleted from Redis...")
	verifyDocumentDeletedFromRedis(t, ctx, uploadResp.DocumentID)

	// Step 10: Verify chunks are deleted from ChromaDB
	t.Log("Step 10: Verifying chunks deleted from ChromaDB...")
	verifyChunksDeletedFromChromaDB(t, ctx, uploadResp.DocumentID, syncTestCollection)

	t.Log("✅ All integration tests passed!")
}

// checkServices verifies all required services are running
func checkServices(t *testing.T, ctx context.Context) {
	t.Helper()

	// Check Go server
	resp, err := http.Get(goServerURL + "/health")
	if err != nil {
		t.Fatalf("Go server not reachable: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Go server health check failed: %d", resp.StatusCode)
	}

	// Check Redis
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis not reachable: %v", err)
	}

	// Check Python backend health
	resp, err = http.Get(pythonServerURL + "/health")
	if err != nil {
		t.Fatalf("Python backend not reachable at %s: %v", pythonServerURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Python backend health check failed: %d", resp.StatusCode)
	}

	// Check ChromaDB
	resp, err = http.Get(chromaDBURL + "/api/v2/heartbeat")
	if err != nil {
		t.Fatalf("ChromaDB not reachable at %s: %v", chromaDBURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ChromaDB health check failed: %d", resp.StatusCode)
	}

	t.Log("All services are running")
	t.Logf("  - Go server: %s", goServerURL)
	t.Logf("  - Python backend: %s", pythonServerURL)
	t.Logf("  - ChromaDB: %s", chromaDBURL)
	t.Logf("  - Redis: %s", redisAddr)
}

// createTestFile creates a temporary test file with sample content
func createTestFile(t *testing.T) string {
	t.Helper()

	content := `# Test Document for Integration Testing

## Introduction
This is a test document created for integration testing purposes.
It contains multiple paragraphs to ensure proper chunking behavior.

## Section 1: Overview
The document upload system processes files through several stages:
1. File upload and validation
2. Document parsing with Docling
3. Text chunking with configurable strategies
4. Embedding generation
5. Storage in ChromaDB vector database

## Section 2: Technical Details
The system maintains a 1:1 mapping between Redis document metadata and ChromaDB chunks.
When a document is uploaded, metadata is stored in Redis including:
- Document ID
- Filename
- Collection name
- Chunk count
- Processing status
- Chunking parameters

## Section 3: Conclusion
This test verifies that the upload flow correctly stores data in both Redis and ChromaDB,
and that deletion properly removes data from both stores.
`

	tmpFile, err := os.CreateTemp("", "integration_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}

// cleanupTestCollection deletes the test collection if it exists
func cleanupTestCollection(t *testing.T, ctx context.Context) {
	t.Helper()
	cleanupCollection(t, ctx, testCollectionName)
}

// cleanupCollection deletes a specific collection if it exists
func cleanupCollection(t *testing.T, ctx context.Context, collectionName string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/api/v1/collections/%s", goServerURL, collectionName), nil)
	if err != nil {
		t.Logf("Failed to create cleanup request: %v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Cleanup request failed (may not exist): %v", err)
		return
	}
	defer resp.Body.Close()

	// 200 or 404 are both acceptable
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Logf("Cleanup returned unexpected status: %d", resp.StatusCode)
	}
}

// createTestCollection creates the test collection in ChromaDB
func createTestCollection(t *testing.T, ctx context.Context) {
	t.Helper()
	createCollection(t, ctx, testCollectionName)
}

// createCollection creates a specific collection in ChromaDB
func createCollection(t *testing.T, ctx context.Context, collectionName string) {
	t.Helper()

	payload := map[string]interface{}{
		"name": collectionName,
		"metadata": map[string]interface{}{
			"hnsw:space": "cosine",
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal collection request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v1/collections", goServerURL),
		bytes.NewReader(jsonData))
	if err != nil {
		t.Fatalf("Failed to create collection request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create collection: %d - %s", resp.StatusCode, string(body))
	}
}

// uploadDocument uploads a document via the Go API
func uploadDocument(t *testing.T, ctx context.Context, filePath string, params map[string]string) *DocumentUploadResponse {
	t.Helper()

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		t.Fatalf("Failed to copy file content: %v", err)
	}

	// Add form fields
	for key, value := range params {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("Failed to write field %s: %v", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v1/documents/upload", goServerURL), body)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: testTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Upload request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Upload failed: %d - %s", resp.StatusCode, string(respBody))
	}

	var uploadResp DocumentUploadResponse
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		t.Fatalf("Failed to parse upload response: %v - %s", err, string(respBody))
	}

	return &uploadResp
}

// verifyRedisDocument verifies document metadata is stored in Redis
func verifyRedisDocument(t *testing.T, ctx context.Context, documentID string, expectedParams map[string]string) *RedisDocument {
	t.Helper()

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()

	docKey := fmt.Sprintf("document:%s", documentID)
	docJSON, err := redisClient.Get(ctx, docKey).Result()
	if err != nil {
		t.Fatalf("Document not found in Redis: %v", err)
	}

	var doc RedisDocument
	if err := json.Unmarshal([]byte(docJSON), &doc); err != nil {
		t.Fatalf("Failed to parse Redis document: %v", err)
	}

	// Verify expected fields
	if doc.ID != documentID {
		t.Errorf("Document ID mismatch: expected %s, got %s", documentID, doc.ID)
	}

	if doc.Collection != expectedParams["collection"] {
		t.Errorf("Collection mismatch: expected %s, got %s", expectedParams["collection"], doc.Collection)
	}

	if doc.ChunkingStrategy != expectedParams["chunking_strategy"] {
		t.Errorf("ChunkingStrategy mismatch: expected %s, got %s",
			expectedParams["chunking_strategy"], doc.ChunkingStrategy)
	}

	if doc.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", doc.Status)
	}

	if !doc.StoredInVectorDB {
		t.Error("Expected StoredInVectorDB to be true")
	}

	if doc.ChunkCount == 0 {
		t.Error("Expected ChunkCount > 0")
	}

	return &doc
}

// verifyChromaDBChunks verifies chunks are stored in ChromaDB
func verifyChromaDBChunks(t *testing.T, ctx context.Context, documentID, collectionName string) *ChromaGetResponse {
	t.Helper()

	// First get collection ID
	collResp, err := http.Get(fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections/%s",
		chromaDBURL, collectionName))
	if err != nil {
		t.Fatalf("Failed to get collection: %v", err)
	}
	defer collResp.Body.Close()

	var collection CollectionResponse
	if err := json.NewDecoder(collResp.Body).Decode(&collection); err != nil {
		t.Fatalf("Failed to parse collection response: %v", err)
	}

	// Query chunks by document_id
	payload := map[string]interface{}{
		"where": map[string]interface{}{
			"document_id": documentID,
		},
		"include": []string{"documents", "metadatas"},
		"limit":   1000,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections/%s/get",
			chromaDBURL, collection.ID),
		bytes.NewReader(jsonData))
	if err != nil {
		t.Fatalf("Failed to create ChromaDB request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("ChromaDB request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("ChromaDB query failed: %d - %s", resp.StatusCode, string(body))
	}

	var getResp ChromaGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		t.Fatalf("Failed to parse ChromaDB response: %v", err)
	}

	if len(getResp.IDs) == 0 {
		t.Fatal("No chunks found in ChromaDB for document")
	}

	return &getResp
}

// verifyDocumentAPI verifies document can be retrieved via API
func verifyDocumentAPI(t *testing.T, ctx context.Context, documentID string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/documents/%s", goServerURL, documentID), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to get document via API: %d - %s", resp.StatusCode, string(body))
	}
}

// verifyChunksAPI verifies chunks can be retrieved via API
func verifyChunksAPI(t *testing.T, ctx context.Context, documentID string, expectedCount int) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/documents/%s/chunks", goServerURL, documentID), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to get chunks via API: %d - %s", resp.StatusCode, string(body))
	}

	var chunksResp struct {
		Chunks     []interface{} `json:"chunks"`
		TotalCount int           `json:"total_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chunksResp); err != nil {
		t.Fatalf("Failed to parse chunks response: %v", err)
	}

	if len(chunksResp.Chunks) != expectedCount {
		t.Errorf("Chunks count mismatch via API: expected %d, got %d", expectedCount, len(chunksResp.Chunks))
	}
}

// deleteDocument deletes a document via API
func deleteDocument(t *testing.T, ctx context.Context, documentID string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/api/v1/documents/%s", goServerURL, documentID), nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Delete request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to delete document: %d - %s", resp.StatusCode, string(body))
	}
}

// verifyDocumentDeletedFromRedis verifies document is deleted from Redis
func verifyDocumentDeletedFromRedis(t *testing.T, ctx context.Context, documentID string) {
	t.Helper()

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()

	docKey := fmt.Sprintf("document:%s", documentID)
	exists, err := redisClient.Exists(ctx, docKey).Result()
	if err != nil {
		t.Fatalf("Failed to check Redis: %v", err)
	}

	if exists > 0 {
		t.Error("Document still exists in Redis after deletion")
	}
}

// verifyChunksDeletedFromChromaDB verifies chunks are deleted from ChromaDB
func verifyChunksDeletedFromChromaDB(t *testing.T, ctx context.Context, documentID, collectionName string) {
	t.Helper()

	// First get collection ID
	collResp, err := http.Get(fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections/%s",
		chromaDBURL, collectionName))
	if err != nil {
		t.Fatalf("Failed to get collection: %v", err)
	}
	defer collResp.Body.Close()

	var collection CollectionResponse
	if err := json.NewDecoder(collResp.Body).Decode(&collection); err != nil {
		t.Fatalf("Failed to parse collection response: %v", err)
	}

	// Query chunks by document_id
	payload := map[string]interface{}{
		"where": map[string]interface{}{
			"document_id": documentID,
		},
		"include": []string{"metadatas"},
		"limit":   1000,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections/%s/get",
			chromaDBURL, collection.ID),
		bytes.NewReader(jsonData))
	if err != nil {
		t.Fatalf("Failed to create ChromaDB request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("ChromaDB request failed: %v", err)
	}
	defer resp.Body.Close()

	var getResp ChromaGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		t.Fatalf("Failed to parse ChromaDB response: %v", err)
	}

	if len(getResp.IDs) > 0 {
		t.Errorf("Chunks still exist in ChromaDB after deletion: found %d chunks", len(getResp.IDs))
	}
}

// TestAsyncDocumentUpload tests the async upload flow
func TestAsyncDocumentUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Use unique collection name for this test to avoid conflicts
	asyncTestCollection := testCollectionName + "_async"

	// Setup
	checkServices(t, ctx)
	testFilePath := createTestFile(t)
	defer os.Remove(testFilePath)
	cleanupCollection(t, ctx, asyncTestCollection)
	createCollection(t, ctx, asyncTestCollection)
	defer cleanupCollection(t, ctx, asyncTestCollection)

	// Upload with async=true
	uploadParams := map[string]string{
		"collection":        asyncTestCollection,
		"chunking_strategy": "sentence",
		"chunk_size":        "256",
		"chunk_overlap":     "25",
		"extract_metadata":  "false",
		"async":             "true",
	}

	t.Log("Uploading document asynchronously...")
	uploadResp := uploadDocument(t, ctx, testFilePath, uploadParams)

	if uploadResp.DocumentID == "" {
		t.Fatal("Expected document ID in response")
	}

	if uploadResp.Status != "pending" && uploadResp.Status != "queued" && uploadResp.Status != "processing" {
		t.Logf("Note: Status is '%s' (may have completed quickly)", uploadResp.Status)
	}

	t.Logf("Document ID: %s, Initial Status: %s", uploadResp.DocumentID, uploadResp.Status)

	// Poll for completion
	t.Log("Polling for completion...")
	var finalStatus string
	for i := 0; i < 60; i++ {
		status := getDocumentStatus(t, ctx, uploadResp.DocumentID)
		t.Logf("Poll %d: Status=%s, Progress=%d%%, Message=%s",
			i+1, status.Status, status.Progress, status.Message)

		if status.Status == "completed" {
			finalStatus = status.Status
			break
		}
		if status.Status == "failed" {
			t.Fatalf("Document processing failed: %s", status.Message)
		}

		time.Sleep(2 * time.Second)
	}

	if finalStatus != "completed" {
		t.Fatal("Document did not complete within timeout")
	}

	// Verify data in both stores
	t.Log("Verifying Redis document...")
	redisDoc := verifyRedisDocument(t, ctx, uploadResp.DocumentID, uploadParams)

	t.Log("Verifying ChromaDB chunks...")
	chunks := verifyChromaDBChunks(t, ctx, uploadResp.DocumentID, asyncTestCollection)

	// Verify 1:1 mapping
	if redisDoc.ChunkCount != len(chunks.IDs) {
		t.Errorf("Chunk count mismatch: Redis=%d, ChromaDB=%d", redisDoc.ChunkCount, len(chunks.IDs))
	}

	t.Log("✅ Async upload test passed!")
}

// getDocumentStatus retrieves document status via API
func getDocumentStatus(t *testing.T, ctx context.Context, documentID string) *DocumentStatusResponse {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/documents/%s/status", goServerURL, documentID), nil)
	if err != nil {
		t.Fatalf("Failed to create status request: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Status request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to get status: %d - %s", resp.StatusCode, string(body))
	}

	var status DocumentStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}

	return &status
}
