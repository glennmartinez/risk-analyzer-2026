package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ============================================================================
// Test Helpers
// ============================================================================

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *PythonClient) {
	server := httptest.NewServer(handler)
	client := NewPythonClient(server.URL)
	return server, client
}

// ============================================================================
// Parse Tests
// ============================================================================

func TestParseText(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/parse/text" {
			t.Errorf("Expected path /parse/text, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req["text"] != "test text" {
			t.Errorf("Expected text 'test text', got %v", req["text"])
		}

		response := ParseResponse{
			Text:             "test text",
			ExtractionMethod: "direct",
			TotalPages:       1,
			Metadata:         map[string]interface{}{},
			Pages:            []interface{}{},
			Tables:           []interface{}{},
			Figures:          []interface{}{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	result, err := client.ParseText(ctx, "test text")
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}

	if result.Text != "test text" {
		t.Errorf("Expected text 'test text', got %s", result.Text)
	}
	if result.ExtractionMethod != "direct" {
		t.Errorf("Expected method 'direct', got %s", result.ExtractionMethod)
	}
}

// ============================================================================
// Chunk Tests
// ============================================================================

func TestChunkSimple(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chunk/simple" {
			t.Errorf("Expected path /chunk/simple, got %s", r.URL.Path)
		}

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		if req["text"] != "test text to chunk" {
			t.Errorf("Expected text, got %v", req["text"])
		}

		response := ChunkResponse{
			Chunks: []TextChunk{
				{Text: "test text", Index: 0},
				{Text: "to chunk", Index: 1},
			},
			TotalChunks:  2,
			StrategyUsed: "sentence",
			ChunkSize:    512,
			ChunkOverlap: 50,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	result, err := client.ChunkSimple(ctx, "test text to chunk", 512, 50)
	if err != nil {
		t.Fatalf("ChunkSimple failed: %v", err)
	}

	if result.TotalChunks != 2 {
		t.Errorf("Expected 2 chunks, got %d", result.TotalChunks)
	}
	if len(result.Chunks) != 2 {
		t.Errorf("Expected 2 chunks in array, got %d", len(result.Chunks))
	}
	if result.Chunks[0].Text != "test text" {
		t.Errorf("Expected first chunk 'test text', got %s", result.Chunks[0].Text)
	}
}

func TestChunk(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chunk/text" {
			t.Errorf("Expected path /chunk/text, got %s", r.URL.Path)
		}

		var req ChunkRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Text != "test" {
			t.Errorf("Expected text 'test', got %s", req.Text)
		}
		if req.Strategy != "sentence" {
			t.Errorf("Expected strategy 'sentence', got %s", req.Strategy)
		}

		response := ChunkResponse{
			Chunks:       []TextChunk{{Text: "test", Index: 0}},
			TotalChunks:  1,
			StrategyUsed: req.Strategy,
			ChunkSize:    req.ChunkSize,
			ChunkOverlap: req.ChunkOverlap,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	chunkReq := &ChunkRequest{
		Text:            "test",
		Strategy:        "sentence",
		ChunkSize:       512,
		ChunkOverlap:    50,
		ExtractMetadata: false,
		NumQuestions:    3,
	}

	result, err := client.Chunk(ctx, chunkReq)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("Expected 1 chunk, got %d", result.TotalChunks)
	}
}

// ============================================================================
// Embed Tests
// ============================================================================

func TestEmbed(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed/text" {
			t.Errorf("Expected path /embed/text, got %s", r.URL.Path)
		}

		var req EmbedSingleRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Text != "test text" {
			t.Errorf("Expected text 'test text', got %s", req.Text)
		}

		response := EmbeddingResponse{
			Embedding: []float32{0.1, 0.2, 0.3, 0.4},
			Dimension: 4,
			Model:     "all-MiniLM-L6-v2",
			Cached:    false,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	result, err := client.Embed(ctx, "test text", nil, true)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if result.Dimension != 4 {
		t.Errorf("Expected dimension 4, got %d", result.Dimension)
	}
	if len(result.Embedding) != 4 {
		t.Errorf("Expected 4 values in embedding, got %d", len(result.Embedding))
	}
	if result.Model != "all-MiniLM-L6-v2" {
		t.Errorf("Expected model 'all-MiniLM-L6-v2', got %s", result.Model)
	}
}

func TestEmbedBatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed/batch" {
			t.Errorf("Expected path /embed/batch, got %s", r.URL.Path)
		}

		var req EmbedRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Texts) != 3 {
			t.Errorf("Expected 3 texts, got %d", len(req.Texts))
		}

		response := EmbedBatchResponse{
			Embeddings: [][]float32{
				{0.1, 0.2},
				{0.3, 0.4},
				{0.5, 0.6},
			},
			Dimension:       2,
			Model:           "all-MiniLM-L6-v2",
			TotalEmbeddings: 3,
			CachedCount:     1,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	texts := []string{"text1", "text2", "text3"}
	result, err := client.EmbedBatch(ctx, texts, nil, 32, true)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if result.TotalEmbeddings != 3 {
		t.Errorf("Expected 3 embeddings, got %d", result.TotalEmbeddings)
	}
	if result.CachedCount != 1 {
		t.Errorf("Expected 1 cached, got %d", result.CachedCount)
	}
	if len(result.Embeddings) != 3 {
		t.Errorf("Expected 3 embedding arrays, got %d", len(result.Embeddings))
	}
}

func TestEmbedQuery(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed/query" {
			t.Errorf("Expected path /embed/query, got %s", r.URL.Path)
		}

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		if req["query"] != "search query" {
			t.Errorf("Expected query 'search query', got %v", req["query"])
		}

		response := EmbeddingResponse{
			Embedding: []float32{0.7, 0.8, 0.9},
			Dimension: 3,
			Model:     "all-MiniLM-L6-v2",
			Cached:    true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	result, err := client.EmbedQuery(ctx, "search query", nil, true)
	if err != nil {
		t.Fatalf("EmbedQuery failed: %v", err)
	}

	if !result.Cached {
		t.Error("Expected cached to be true")
	}
	if result.Dimension != 3 {
		t.Errorf("Expected dimension 3, got %d", result.Dimension)
	}
}

// ============================================================================
// Metadata Tests
// ============================================================================

func TestExtractTitle(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata/title" {
			t.Errorf("Expected path /metadata/title, got %s", r.URL.Path)
		}

		title := "Test Document"
		response := map[string]interface{}{
			"title": title,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	result, err := client.ExtractTitle(ctx, "test text")
	if err != nil {
		t.Fatalf("ExtractTitle failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected title, got nil")
	}
	if *result != "Test Document" {
		t.Errorf("Expected 'Test Document', got %s", *result)
	}
}

func TestExtractKeywords(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata/keywords" {
			t.Errorf("Expected path /metadata/keywords, got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"keywords": []string{"keyword1", "keyword2", "keyword3"},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	result, err := client.ExtractKeywords(ctx, "test text", 5)
	if err != nil {
		t.Fatalf("ExtractKeywords failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 keywords, got %d", len(result))
	}
	if result[0] != "keyword1" {
		t.Errorf("Expected 'keyword1', got %s", result[0])
	}
}

func TestExtractMetadata(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata/extract" {
			t.Errorf("Expected path /metadata/extract, got %s", r.URL.Path)
		}

		title := "Test Title"
		response := MetadataResponse{
			Title:     &title,
			Keywords:  []string{"kw1", "kw2"},
			Questions: []string{"What is this?", "Why?"},
			Metadata:  map[string]interface{}{"extra": "data"},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	req := &MetadataRequest{
		Text:             "test text",
		ExtractTitle:     true,
		ExtractKeywords:  true,
		ExtractQuestions: true,
		NumQuestions:     3,
		NumKeywords:      5,
	}

	result, err := client.ExtractMetadata(ctx, req)
	if err != nil {
		t.Fatalf("ExtractMetadata failed: %v", err)
	}

	if result.Title == nil || *result.Title != "Test Title" {
		t.Error("Expected title 'Test Title'")
	}
	if len(result.Keywords) != 2 {
		t.Errorf("Expected 2 keywords, got %d", len(result.Keywords))
	}
	if len(result.Questions) != 2 {
		t.Errorf("Expected 2 questions, got %d", len(result.Questions))
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestRetryLogic(t *testing.T) {
	attempts := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Succeed on 3rd attempt
		response := ParseResponse{
			Text:             "success",
			ExtractionMethod: "retry",
			Metadata:         map[string]interface{}{},
			Pages:            []interface{}{},
			Tables:           []interface{}{},
			Figures:          []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	result, err := client.ParseText(ctx, "test")
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	if result.Text != "success" {
		t.Errorf("Expected 'success', got %s", result.Text)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClientError4xx(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Client errors should not be retried
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	_, err := client.ParseText(ctx, "test")
	if err == nil {
		t.Fatal("Expected error for 400 response")
	}
}

func TestContextCancellation(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.ParseText(ctx, "test")
	if err == nil {
		t.Fatal("Expected context deadline exceeded error")
	}
}

// ============================================================================
// Health Check Tests
// ============================================================================

func TestHealthCheck(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/parse/health" {
			t.Errorf("Expected path /parse/health, got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"status":  "healthy",
			"service": "parse",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	healthy, err := client.HealthCheck(ctx, "parse")
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	if !healthy {
		t.Error("Expected service to be healthy")
	}
}

func TestGetAvailableModels(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed/models" {
			t.Errorf("Expected path /embed/models, got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"models": []map[string]interface{}{
				{
					"id":        "all-MiniLM-L6-v2",
					"dimension": 384,
					"free":      true,
				},
				{
					"id":        "all-mpnet-base-v2",
					"dimension": 768,
					"free":      true,
				},
			},
			"default": "all-MiniLM-L6-v2",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	server, client := setupTestServer(t, handler)
	defer server.Close()

	ctx := context.Background()
	models, err := client.GetAvailableModels(ctx)
	if err != nil {
		t.Fatalf("GetAvailableModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
}

// ============================================================================
// Custom Client Configuration Tests
// ============================================================================

func TestNewPythonClientWithOptions(t *testing.T) {
	client := NewPythonClientWithOptions(
		"http://localhost:8000",
		30*time.Second,
		5,
	)

	if client.baseURL != "http://localhost:8000" {
		t.Errorf("Expected baseURL http://localhost:8000, got %s", client.baseURL)
	}
	if client.retries != 5 {
		t.Errorf("Expected 5 retries, got %d", client.retries)
	}
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected 30s timeout, got %v", client.httpClient.Timeout)
	}
}
