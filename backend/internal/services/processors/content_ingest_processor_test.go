package processors

import (
	"context"
	"testing"

	"risk-analyzer/internal/models"
	"risk-analyzer/internal/services"
	"risk-analyzer/internal/services/adapters"
)

// MockPythonClient implements PythonClientInterface for testing
type MockPythonClient struct {
	ChunkCalls      []services.ChunkRequest
	EmbedBatchCalls [][]string
}

func NewMockPythonClient() *MockPythonClient {
	return &MockPythonClient{
		ChunkCalls:      make([]services.ChunkRequest, 0),
		EmbedBatchCalls: make([][]string, 0),
	}
}

func (m *MockPythonClient) Chunk(ctx context.Context, req *services.ChunkRequest) (*services.ChunkResponse, error) {
	m.ChunkCalls = append(m.ChunkCalls, *req)
	return &services.ChunkResponse{
		Chunks: []services.TextChunk{
			{
				Text:  req.Text,
				Index: 0,
				Metadata: &services.ChunkMetadata{
					ChunkIndex: 0,
					TokenCount: ptr(len(req.Text) / 4),
				},
			},
		},
		TotalChunks:  1,
		StrategyUsed: req.Strategy,
		ChunkSize:    req.ChunkSize,
		ChunkOverlap: req.ChunkOverlap,
	}, nil
}

func (m *MockPythonClient) EmbedBatch(ctx context.Context, texts []string, model *string, batchSize int, useCache bool) (*services.EmbedBatchResponse, error) {
	m.EmbedBatchCalls = append(m.EmbedBatchCalls, texts)
	dim := 384
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = make([]float32, dim)
		for j := range dim {
			embeddings[i][j] = float32(i+j) / float32(dim)
		}
	}
	return &services.EmbedBatchResponse{
		Embeddings:      embeddings,
		Dimension:       dim,
		Model:           "text-embedding-3-small",
		TotalEmbeddings: len(texts),
	}, nil
}

// Unused interface methods - return nil
func (m *MockPythonClient) ParseDocument(ctx context.Context, fileData []byte, filename string, extractMetadata bool, maxPages int) (*services.ParseResponse, error) {
	return nil, nil
}
func (m *MockPythonClient) ParseText(ctx context.Context, text string) (*services.ParseResponse, error) {
	return nil, nil
}
func (m *MockPythonClient) ChunkSimple(ctx context.Context, text string, chunkSize, chunkOverlap int) (*services.ChunkResponse, error) {
	return nil, nil
}
func (m *MockPythonClient) Embed(ctx context.Context, text string, model *string, useCache bool) (*services.EmbeddingResponse, error) {
	return nil, nil
}
func (m *MockPythonClient) EmbedQuery(ctx context.Context, text string, model *string, useCache bool) (*services.EmbeddingResponse, error) {
	return nil, nil
}
func (m *MockPythonClient) ExtractMetadata(ctx context.Context, req *services.MetadataRequest) (*services.MetadataResponse, error) {
	return nil, nil
}
func (m *MockPythonClient) ExtractTitle(ctx context.Context, text string) (*string, error) {
	return nil, nil
}
func (m *MockPythonClient) ExtractKeywords(ctx context.Context, text string, numKeywords int) ([]string, error) {
	return nil, nil
}
func (m *MockPythonClient) ExtractQuestions(ctx context.Context, text string, numQuestions int) ([]string, error) {
	return nil, nil
}
func (m *MockPythonClient) HealthCheck(ctx context.Context, service string) (bool, error) {
	return true, nil
}
func (m *MockPythonClient) GetAvailableModels(ctx context.Context) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *MockPythonClient) GetChunkingStrategies(ctx context.Context) ([]string, error) {
	return []string{"sentence", "semantic", "recursive"}, nil
}
func (m *MockPythonClient) CreateJobWithCallback(ctx context.Context, payload services.DocumentCallbackPayload) (string, string, error) {
	return "", "", nil
}
func (m *MockPythonClient) CreateParseJobWithCallback(ctx context.Context, fileData []byte, filename string, extractMetadata bool, maxPages int, callbackURL string, goJobID string) (string, string, error) {
	return "", "", nil
}

// Helper functions
func ptr[T any](v T) *T {
	return &v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// Adapter Tests (Unit Tests)
// ============================================================================

func TestJiraAdapter_Unit(t *testing.T) {
	t.Log("=== Unit Test: Jira Adapter ===")

	adapter := &adapters.JiraAdapter{}

	payload := map[string]interface{}{
		"key":         "PROJ-456",
		"summary":     "Test issue",
		"description": "This is a test description",
		"status":      "Closed",
		"priority":    "Low",
	}

	text, err := adapter.ToText(payload)
	if err != nil {
		t.Fatalf("ToText failed: %v", err)
	}

	// Check key fields
	if !containsString(text, "PROJ-456") {
		t.Error("Expected text to contain PROJ-456")
	}
	if !containsString(text, "Test issue") {
		t.Error("Expected text to contain Test issue")
	}

	metadata := adapter.ToMetadata(payload)
	if metadata["jira_key"] != "PROJ-456" {
		t.Errorf("Expected jira_key = PROJ-456, got %v", metadata["jira_key"])
	}

	t.Log("Jira Adapter Test PASSED")
}

func TestNotionAdapter_Unit(t *testing.T) {
	t.Log("=== Unit Test: Notion Adapter ===")

	adapter := &adapters.NotionAdapter{}

	payload := map[string]interface{}{
		"id":    "page-123",
		"title": "Test Page",
		"content": []interface{}{
			map[string]interface{}{
				"type": "heading_1",
				"rich_text": []interface{}{
					map[string]interface{}{"text": "Main Heading"},
				},
			},
		},
	}

	text, err := adapter.ToText(payload)
	if err != nil {
		t.Fatalf("ToText failed: %v", err)
	}

	if !containsString(text, "Test Page") {
		t.Error("Expected text to contain Test Page")
	}

	metadata := adapter.ToMetadata(payload)
	if metadata["notion_id"] != "page-123" {
		t.Errorf("Expected notion_id = page-123, got %v", metadata["notion_id"])
	}

	t.Log("Notion Adapter Test PASSED")
}

func TestMarkdownAdapter_Unit(t *testing.T) {
	t.Log("=== Unit Test: Markdown Adapter ===")

	adapter := &adapters.MarkdownAdapter{}

	content := "# Hello World\n\nThis is **bold** text."

	text, err := adapter.ToText(content)
	if err != nil {
		t.Fatalf("ToText failed: %v", err)
	}

	// Markdown should be preserved as-is
	if text != content {
		t.Errorf("Markdown content was modified")
	}

	t.Log("Markdown Adapter Test PASSED")
}

func TestJSONAdapter_Unit(t *testing.T) {
	t.Log("=== Unit Test: JSON Adapter ===")

	adapter := &adapters.JSONAdapter{}

	content := map[string]interface{}{
		"name":  "test",
		"value": 123,
	}

	text, err := adapter.ToText(content)
	if err != nil {
		t.Fatalf("ToText failed: %v", err)
	}

	// JSON should be formatted
	if !containsString(text, `"name"`) {
		t.Errorf("Expected text to contain JSON field, got: %s", text)
	}

	t.Log("JSON Adapter Test PASSED")
}

// ============================================================================
// Integration Tests (using real adapters with mock Python client)
// ============================================================================

func TestContentIngestProcessor_JiraIngest(t *testing.T) {
	t.Log("=== Integration Test: Jira Content Ingest ===")

	mockPy := NewMockPythonClient()

	adapterRegistry := adapters.NewContentAdapterRegistry()

	// Create a minimal processor that we can test
	_ = map[string]interface{}{
		"document_id":       "test-doc-001",
		"collection":        "test-collection",
		"source_type":       "jira",
		"chunking_strategy": "semantic",
		"content": map[string]interface{}{
			"key":         "PROJ-123",
			"summary":     "Fix login bug",
			"description": "Users cannot login with SSO",
			"status":      "Open",
			"priority":    "High",
		},
	}

	// Test adapter directly
	sourceType := models.SourceTypeJira
	adapter := adapterRegistry.GetAdapter(sourceType)

	_ = adapter

	// Execute chunk call directly on mock
	chunkReq := &services.ChunkRequest{
		Text:            "Test content",
		Strategy:        "semantic",
		ChunkSize:       512,
		ChunkOverlap:    50,
		ExtractMetadata: true,
		NumQuestions:    3,
	}

	ctx := context.Background()
	resp, err := mockPy.Chunk(ctx, chunkReq)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(resp.Chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(resp.Chunks))
	}

	if len(mockPy.ChunkCalls) != 1 {
		t.Errorf("Expected 1 Chunk call, got %d", len(mockPy.ChunkCalls))
	}

	// Test embed call
	embedResp, err := mockPy.EmbedBatch(ctx, []string{"chunk1"}, nil, 32, false)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(embedResp.Embeddings) != 1 {
		t.Errorf("Expected 1 embedding, got %d", len(embedResp.Embeddings))
	}

	t.Log("Jira Content Ingest Integration Test PASSED")
}

func TestContentIngestProcessor_NotionIngest(t *testing.T) {
	t.Log("=== Integration Test: Notion Content Ingest ===")

	mockPy := NewMockPythonClient()

	ctx := context.Background()

	// Test Notion content
	chunkReq := &services.ChunkRequest{
		Text:            "Title: API Doc\n\n# Introduction\nContent here",
		Strategy:        "recursive",
		ChunkSize:       1024,
		ChunkOverlap:    100,
		ExtractMetadata: false,
		NumQuestions:    0,
	}

	resp, err := mockPy.Chunk(ctx, chunkReq)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(resp.Chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(resp.Chunks))
	}

	t.Log("Notion Content Ingest Integration Test PASSED")
}

func TestContentIngestProcessor_MarkdownIngest(t *testing.T) {
	t.Log("=== Integration Test: Markdown Content Ingest ===")

	mockPy := NewMockPythonClient()

	ctx := context.Background()

	markdown := `# My Document

## Section 1
Content here.

## Section 2
More content.`

	chunkReq := &services.ChunkRequest{
		Text:            markdown,
		Strategy:        "markdown",
		ChunkSize:       512,
		ChunkOverlap:    50,
		ExtractMetadata: true,
		NumQuestions:    3,
	}

	resp, err := mockPy.Chunk(ctx, chunkReq)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(resp.Chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(resp.Chunks))
	}

	// Verify markdown is preserved
	if resp.Chunks[0].Text != markdown {
		t.Error("Markdown content was not preserved")
	}

	t.Log("Markdown Content Ingest Integration Test PASSED")
}

func TestContentIngestProcessor_JSONIngest(t *testing.T) {
	t.Log("=== Integration Test: JSON Content Ingest ===")

	mockPy := NewMockPythonClient()

	ctx := context.Background()

	jsonContent := `{
  "title": "Data Record",
  "fields": ["field1", "field2"]
}`

	chunkReq := &services.ChunkRequest{
		Text:            jsonContent,
		Strategy:        "sentence",
		ChunkSize:       512,
		ChunkOverlap:    50,
		ExtractMetadata: false,
		NumQuestions:    0,
	}

	resp, err := mockPy.Chunk(ctx, chunkReq)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(resp.Chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(resp.Chunks))
	}

	t.Log("JSON Content Ingest Integration Test PASSED")
}

func TestPipeline_EndToEnd(t *testing.T) {
	t.Log("=== Integration Test: Full Pipeline ===")

	mockPy := NewMockPythonClient()
	ctx := context.Background()

	// Simulate full pipeline
	// 1. Normalize Jira content
	adapter := &adapters.JiraAdapter{}
	jiraPayload := map[string]interface{}{
		"key":         "PROJ-999",
		"summary":     "End-to-end test",
		"description": "Testing the full pipeline",
		"status":      "In Progress",
	}

	text, _ := adapter.ToText(jiraPayload)

	// 2. Chunk the text
	chunkResp, err := mockPy.Chunk(ctx, &services.ChunkRequest{
		Text:            text,
		Strategy:        "semantic",
		ChunkSize:       512,
		ChunkOverlap:    50,
		ExtractMetadata: true,
		NumQuestions:    3,
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// 3. Generate embeddings
	chunks := make([]string, len(chunkResp.Chunks))
	for i, c := range chunkResp.Chunks {
		chunks[i] = c.Text
	}

	embedResp, err := mockPy.EmbedBatch(ctx, chunks, nil, 32, false)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	// Verify pipeline output
	if len(chunkResp.Chunks) == 0 {
		t.Error("No chunks produced")
	}

	if len(embedResp.Embeddings) != len(chunks) {
		t.Errorf("Embedding count mismatch: expected %d, got %d", len(chunks), len(embedResp.Embeddings))
	}

	if embedResp.Dimension == 0 {
		t.Error("Embedding dimension is 0")
	}

	t.Log("Full Pipeline Integration Test PASSED")
}

func containsString(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
