package services

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"risk-analyzer/internal/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// Test Setup
// ============================================================================

func setupTestSearchService(t *testing.T, enableCache bool) (*SearchService, *MockPythonClient, *MockVectorRepository, *MockDocumentRepository) {
	mockPython := new(MockPythonClient)
	mockVectorRepo := new(MockVectorRepository)
	mockDocRepo := new(MockDocumentRepository)

	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	service := NewSearchService(
		mockPython,
		mockVectorRepo,
		mockDocRepo,
		logger,
		enableCache,
	)

	return service, mockPython, mockVectorRepo, mockDocRepo
}

func createTestSearchRequest() *SearchRequest {
	return &SearchRequest{
		Query:      "What is risk management?",
		Collection: "test-collection",
		TopK:       10,
		UseCache:   false,
	}
}

func createMockEmbedQueryResponse() *EmbeddingResponse {
	return &EmbeddingResponse{
		Embedding: make([]float32, 384),
		Dimension: 384,
		Model:     "all-MiniLM-L6-v2",
		Cached:    false,
	}
}

func createMockVectorSearchResults() []*repositories.SearchResult {
	return []*repositories.SearchResult{
		{
			ChunkID:    "doc1_chunk_0",
			DocumentID: "doc1",
			Text:       "Risk management is the process of identifying and mitigating risks.",
			Score:      0.95,
			Distance:   0.05,
			Metadata: map[string]interface{}{
				"chunk_index": 0,
				"page":        1,
			},
		},
		{
			ChunkID:    "doc2_chunk_5",
			DocumentID: "doc2",
			Text:       "Effective risk management requires continuous monitoring.",
			Score:      0.87,
			Distance:   0.13,
			Metadata: map[string]interface{}{
				"chunk_index": 5,
				"page":        3,
			},
		},
	}
}

func createMockDocuments() []*repositories.Document {
	return []*repositories.Document{
		{
			ID:       "doc1",
			Filename: "risk-guide.pdf",
			Metadata: map[string]interface{}{
				"title": "Risk Management Guide",
			},
		},
		{
			ID:       "doc2",
			Filename: "risk-practices.pdf",
			Metadata: map[string]interface{}{
				"title": "Best Risk Practices",
			},
		},
	}
}

// ============================================================================
// Tests
// ============================================================================

func TestNewSearchService(t *testing.T) {
	service, _, _, _ := setupTestSearchService(t, true)

	assert.NotNil(t, service)
	assert.NotNil(t, service.pythonClient)
	assert.NotNil(t, service.vectorRepo)
	assert.NotNil(t, service.docRepo)
	assert.NotNil(t, service.cache)
	assert.NotNil(t, service.logger)
}

func TestNewSearchService_NoCaching(t *testing.T) {
	service, _, _, _ := setupTestSearchService(t, false)

	assert.NotNil(t, service)
	assert.Nil(t, service.cache)
}

func TestValidateSearchRequest(t *testing.T) {
	service, _, _, _ := setupTestSearchService(t, false)

	tests := []struct {
		name        string
		req         *SearchRequest
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid request",
			req:         createTestSearchRequest(),
			expectError: false,
		},
		{
			name: "Missing query",
			req: &SearchRequest{
				Collection: "test",
				TopK:       10,
			},
			expectError: true,
			errorMsg:    "query is required",
		},
		{
			name: "Missing collection",
			req: &SearchRequest{
				Query: "test query",
				TopK:  10,
			},
			expectError: true,
			errorMsg:    "collection is required",
		},
		{
			name: "TopK too large",
			req: &SearchRequest{
				Query:      "test",
				Collection: "test",
				TopK:       200,
			},
			expectError: true,
			errorMsg:    "topK cannot exceed 100",
		},
		{
			name: "TopK defaults to 10",
			req: &SearchRequest{
				Query:      "test",
				Collection: "test",
				TopK:       0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateSearchRequest(tt.req)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.req.TopK == 0 {
					assert.Equal(t, 10, tt.req.TopK)
				}
			}
		})
	}
}

func TestSearchDocuments_Success(t *testing.T) {
	service, mockPython, mockVectorRepo, mockDocRepo := setupTestSearchService(t, false)
	ctx := context.Background()
	req := createTestSearchRequest()

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil)
	mockPython.On("EmbedQuery", ctx, req.Query, mock.Anything, true).Return(createMockEmbedQueryResponse(), nil)
	mockVectorRepo.On("SearchChunks", ctx, req.Collection, mock.AnythingOfType("[]float32"), req.TopK, req.Filter).Return(createMockVectorSearchResults(), nil)
	mockDocRepo.On("GetBatch", ctx, []string{"doc1", "doc2"}).Return(createMockDocuments(), nil)

	// Execute
	resp, err := service.SearchDocuments(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, resp.TotalResults)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, req.Query, resp.Query)
	assert.Equal(t, req.Collection, resp.Collection)
	assert.GreaterOrEqual(t, resp.TimeTakenMs, 0.0)
	assert.False(t, resp.FromCache)

	// Check first result
	assert.Equal(t, "doc1_chunk_0", resp.Results[0].ChunkID)
	assert.Equal(t, "doc1", resp.Results[0].DocumentID)
	assert.Equal(t, float32(0.95), resp.Results[0].Score)
	assert.NotNil(t, resp.Results[0].Document)
	assert.Equal(t, "risk-guide.pdf", resp.Results[0].Document.Filename)
	assert.Equal(t, "Risk Management Guide", resp.Results[0].Document.Title)

	mockVectorRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
	mockDocRepo.AssertExpectations(t)
}

func TestSearchDocuments_CollectionNotFound(t *testing.T) {
	service, _, mockVectorRepo, _ := setupTestSearchService(t, false)
	ctx := context.Background()
	req := createTestSearchRequest()

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(false, nil)

	// Execute
	resp, err := service.SearchDocuments(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "collection not found")

	mockVectorRepo.AssertExpectations(t)
}

func TestSearchDocuments_CollectionCheckFails(t *testing.T) {
	service, _, mockVectorRepo, _ := setupTestSearchService(t, false)
	ctx := context.Background()
	req := createTestSearchRequest()

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(false, errors.New("connection error"))

	// Execute
	resp, err := service.SearchDocuments(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to check collection")

	mockVectorRepo.AssertExpectations(t)
}

func TestSearchDocuments_EmbedQueryFails(t *testing.T) {
	service, mockPython, mockVectorRepo, _ := setupTestSearchService(t, false)
	ctx := context.Background()
	req := createTestSearchRequest()

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil)
	mockPython.On("EmbedQuery", ctx, req.Query, mock.Anything, true).Return(nil, errors.New("embedding failed"))

	// Execute
	resp, err := service.SearchDocuments(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to embed query")

	mockVectorRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
}

func TestSearchDocuments_VectorSearchFails(t *testing.T) {
	service, mockPython, mockVectorRepo, _ := setupTestSearchService(t, false)
	ctx := context.Background()
	req := createTestSearchRequest()

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil)
	mockPython.On("EmbedQuery", ctx, req.Query, mock.Anything, true).Return(createMockEmbedQueryResponse(), nil)
	mockVectorRepo.On("SearchChunks", ctx, req.Collection, mock.AnythingOfType("[]float32"), req.TopK, req.Filter).Return(nil, errors.New("search failed"))

	// Execute
	resp, err := service.SearchDocuments(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "vector search failed")

	mockVectorRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
}

func TestSearchDocuments_WithMinScore(t *testing.T) {
	service, mockPython, mockVectorRepo, mockDocRepo := setupTestSearchService(t, false)
	ctx := context.Background()
	req := createTestSearchRequest()
	minScore := float32(0.9)
	req.MinScore = &minScore

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil)
	mockPython.On("EmbedQuery", ctx, req.Query, mock.Anything, true).Return(createMockEmbedQueryResponse(), nil)
	mockVectorRepo.On("SearchChunks", ctx, req.Collection, mock.AnythingOfType("[]float32"), req.TopK, req.Filter).Return(createMockVectorSearchResults(), nil)
	mockDocRepo.On("GetBatch", ctx, []string{"doc1"}).Return([]*repositories.Document{createMockDocuments()[0]}, nil)

	// Execute
	resp, err := service.SearchDocuments(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.TotalResults) // Only doc1 has score >= 0.9
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "doc1_chunk_0", resp.Results[0].ChunkID)
	assert.GreaterOrEqual(t, resp.Results[0].Score, minScore)

	mockVectorRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
	mockDocRepo.AssertExpectations(t)
}

func TestSearchDocuments_EnrichmentFails(t *testing.T) {
	service, mockPython, mockVectorRepo, mockDocRepo := setupTestSearchService(t, false)
	ctx := context.Background()
	req := createTestSearchRequest()

	// Setup mocks
	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil)
	mockPython.On("EmbedQuery", ctx, req.Query, mock.Anything, true).Return(createMockEmbedQueryResponse(), nil)
	mockVectorRepo.On("SearchChunks", ctx, req.Collection, mock.AnythingOfType("[]float32"), req.TopK, req.Filter).Return(createMockVectorSearchResults(), nil)
	mockDocRepo.On("GetBatch", ctx, mock.Anything).Return(nil, errors.New("database error"))

	// Execute - should still succeed with unenriched results
	resp, err := service.SearchDocuments(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, resp.TotalResults)
	// Results should not have document info
	assert.Nil(t, resp.Results[0].Document)
	assert.Nil(t, resp.Results[1].Document)

	mockVectorRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
	mockDocRepo.AssertExpectations(t)
}

func TestSearchDocuments_WithCaching(t *testing.T) {
	service, mockPython, mockVectorRepo, mockDocRepo := setupTestSearchService(t, true)
	ctx := context.Background()
	req := createTestSearchRequest()
	req.UseCache = true

	// Setup mocks - should only be called once
	mockVectorRepo.On("CollectionExists", ctx, req.Collection).Return(true, nil).Once()
	mockPython.On("EmbedQuery", ctx, req.Query, mock.Anything, true).Return(createMockEmbedQueryResponse(), nil).Once()
	mockVectorRepo.On("SearchChunks", ctx, req.Collection, mock.AnythingOfType("[]float32"), req.TopK, req.Filter).Return(createMockVectorSearchResults(), nil).Once()
	mockDocRepo.On("GetBatch", ctx, mock.Anything).Return(createMockDocuments(), nil).Once()

	// First search - should hit the backend
	resp1, err := service.SearchDocuments(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp1)
	assert.False(t, resp1.FromCache)

	// Second search - should hit the cache
	resp2, err := service.SearchDocuments(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp2)
	assert.True(t, resp2.FromCache)

	// Results should be the same
	assert.Equal(t, resp1.TotalResults, resp2.TotalResults)

	// Mocks should only have been called once
	mockVectorRepo.AssertExpectations(t)
	mockPython.AssertExpectations(t)
	mockDocRepo.AssertExpectations(t)
}

func TestSearchCache_Basic(t *testing.T) {
	cache := NewSearchCache(10, 5*time.Minute)

	req := createTestSearchRequest()
	resp := &SearchResponse{
		Results:      []*SearchResult{{ChunkID: "test"}},
		TotalResults: 1,
	}

	// Set and get
	cache.Set(req, resp)
	cached := cache.Get(req)

	assert.NotNil(t, cached)
	assert.Equal(t, 1, cached.TotalResults)
	assert.Equal(t, 1, cache.Size())
}

func TestSearchCache_Miss(t *testing.T) {
	cache := NewSearchCache(10, 5*time.Minute)

	req := createTestSearchRequest()
	cached := cache.Get(req)

	assert.Nil(t, cached)
}

func TestSearchCache_Expiration(t *testing.T) {
	cache := NewSearchCache(10, 100*time.Millisecond)

	req := createTestSearchRequest()
	resp := &SearchResponse{TotalResults: 1}

	cache.Set(req, resp)

	// Should be cached initially
	cached := cache.Get(req)
	assert.NotNil(t, cached)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	cached = cache.Get(req)
	assert.Nil(t, cached)
}

func TestSearchCache_MaxSize(t *testing.T) {
	cache := NewSearchCache(2, 5*time.Minute)

	req1 := &SearchRequest{Query: "query1", Collection: "col1", TopK: 10}
	req2 := &SearchRequest{Query: "query2", Collection: "col1", TopK: 10}
	req3 := &SearchRequest{Query: "query3", Collection: "col1", TopK: 10}

	resp := &SearchResponse{TotalResults: 1}

	cache.Set(req1, resp)
	cache.Set(req2, resp)

	assert.Equal(t, 2, cache.Size())

	// Adding third should evict first
	cache.Set(req3, resp)

	assert.Equal(t, 2, cache.Size())
}

func TestSearchCache_Clear(t *testing.T) {
	cache := NewSearchCache(10, 5*time.Minute)

	req := createTestSearchRequest()
	resp := &SearchResponse{TotalResults: 1}

	cache.Set(req, resp)
	assert.Equal(t, 1, cache.Size())

	cache.Clear()
	assert.Equal(t, 0, cache.Size())

	cached := cache.Get(req)
	assert.Nil(t, cached)
}

func TestSearchCache_DifferentRequests(t *testing.T) {
	cache := NewSearchCache(10, 5*time.Minute)

	req1 := &SearchRequest{Query: "query1", Collection: "col1", TopK: 10}
	req2 := &SearchRequest{Query: "query2", Collection: "col1", TopK: 10}

	resp1 := &SearchResponse{Query: "query1", TotalResults: 1}
	resp2 := &SearchResponse{Query: "query2", TotalResults: 2}

	cache.Set(req1, resp1)
	cache.Set(req2, resp2)

	cached1 := cache.Get(req1)
	cached2 := cache.Get(req2)

	assert.NotNil(t, cached1)
	assert.NotNil(t, cached2)
	assert.Equal(t, "query1", cached1.Query)
	assert.Equal(t, "query2", cached2.Query)
}

func TestSearchCache_NilCache(t *testing.T) {
	var cache *SearchCache // nil cache

	req := createTestSearchRequest()
	resp := &SearchResponse{TotalResults: 1}

	// Should not panic
	cache.Set(req, resp)
	cached := cache.Get(req)
	assert.Nil(t, cached)

	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}
