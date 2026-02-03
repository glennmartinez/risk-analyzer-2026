package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"risk-analyzer/internal/repositories"
)

// SearchService handles document search using vector similarity
type SearchService struct {
	pythonClient PythonClientInterface
	vectorRepo   repositories.VectorRepository
	docRepo      repositories.DocumentRepository
	logger       *log.Logger
	cache        *searchCache
}

// NewSearchService creates a new search service
func NewSearchService(
	pythonClient PythonClientInterface,
	vectorRepo repositories.VectorRepository,
	docRepo repositories.DocumentRepository,
	logger *log.Logger,
) *SearchService {
	return &SearchService{
		pythonClient: pythonClient,
		vectorRepo:   vectorRepo,
		docRepo:      docRepo,
		logger:       logger,
		cache:        newSearchCache(5 * time.Minute), // 5 minute TTL
	}
}

// SearchRequest represents a search query request
type SearchRequest struct {
	Query      string                 `json:"query"`
	Collection string                 `json:"collection"`
	TopK       int                    `json:"top_k"`
	MinScore   *float32               `json:"min_score,omitempty"`
	Filter     map[string]interface{} `json:"filter,omitempty"`
	UseCache   bool                   `json:"use_cache"`
	Model      *string                `json:"model,omitempty"`
}

// SearchResult represents a single search result with context
type SearchResult struct {
	ChunkID      string                 `json:"chunk_id"`
	DocumentID   string                 `json:"document_id"`
	Text         string                 `json:"text"`
	Score        float32                `json:"score"`
	Distance     float32                `json:"distance"`
	Metadata     map[string]interface{} `json:"metadata"`
	DocumentName string                 `json:"document_name,omitempty"`
}

// SearchResponse represents the response from a search operation
type SearchResponse struct {
	Results       []*SearchResult `json:"results"`
	Query         string          `json:"query"`
	Collection    string          `json:"collection"`
	TotalResults  int             `json:"total_results"`
	SearchTimeMs  float64         `json:"search_time_ms"`
	FromCache     bool            `json:"from_cache"`
	EmbeddingTime float64         `json:"embedding_time_ms,omitempty"`
}

// SearchDocuments performs a semantic search across document chunks
func (s *SearchService) SearchDocuments(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	startTime := time.Now()

	// Validate request
	if err := s.validateSearchRequest(req); err != nil {
		s.logger.Printf("Invalid search request: %v", err)
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Check if collection exists
	exists, err := s.vectorRepo.CollectionExists(ctx, req.Collection)
	if err != nil {
		s.logger.Printf("Failed to check collection existence: %v", err)
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}

	if !exists {
		s.logger.Printf("Collection not found: %s", req.Collection)
		return nil, fmt.Errorf("collection not found: %s", req.Collection)
	}

	// Check cache if enabled
	if req.UseCache {
		if cached := s.cache.Get(req); cached != nil {
			s.logger.Printf("Cache hit for query: %s (collection: %s)", req.Query, req.Collection)
			cached.FromCache = true
			cached.SearchTimeMs = time.Since(startTime).Seconds() * 1000
			return cached, nil
		}
	}

	// Embed the query
	s.logger.Printf("Embedding query: %s (collection: %s)", req.Query, req.Collection)
	embedStart := time.Now()
	embedResp, err := s.pythonClient.EmbedQuery(ctx, req.Query, req.Model, req.UseCache)
	if err != nil {
		s.logger.Printf("Failed to embed query: %v", err)
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	embeddingTime := time.Since(embedStart).Seconds() * 1000

	s.logger.Printf("Query embedded in %.2fms (dimension: %d)", embeddingTime, embedResp.Dimension)

	// Search vector database
	s.logger.Printf("Searching collection '%s' with topK=%d", req.Collection, req.TopK)
	searchStart := time.Now()
	chunks, err := s.vectorRepo.SearchChunks(ctx, req.Collection, embedResp.Embedding, req.TopK, req.Filter)
	if err != nil {
		s.logger.Printf("Vector search failed: %v", err)
		return nil, fmt.Errorf("search failed: %w", err)
	}
	searchTime := time.Since(searchStart).Seconds() * 1000

	s.logger.Printf("Found %d results in %.2fms", len(chunks), searchTime)

	// Build search results
	results := make([]*SearchResult, 0, len(chunks))
	for _, chunk := range chunks {
		// Apply score filter if specified
		if req.MinScore != nil && chunk.Score < *req.MinScore {
			continue
		}

		result := &SearchResult{
			ChunkID:    chunk.ChunkID,
			DocumentID: chunk.DocumentID,
			Text:       chunk.Text,
			Score:      chunk.Score,
			Distance:   chunk.Distance,
			Metadata:   chunk.Metadata,
		}

		// Optionally enrich with document name
		if docName, ok := chunk.Metadata["filename"].(string); ok {
			result.DocumentName = docName
		}

		results = append(results, result)
	}

	totalTime := time.Since(startTime).Seconds() * 1000
	s.logger.Printf("Search completed: %d results, %.2fms total (embed: %.2fms, search: %.2fms)",
		len(results), totalTime, embeddingTime, searchTime)

	response := &SearchResponse{
		Results:       results,
		Query:         req.Query,
		Collection:    req.Collection,
		TotalResults:  len(results),
		SearchTimeMs:  totalTime,
		FromCache:     false,
		EmbeddingTime: embeddingTime,
	}

	// Cache the results if enabled
	if req.UseCache {
		s.cache.Set(req, response)
	}

	return response, nil
}

// SearchMultipleCollections searches across multiple collections
func (s *SearchService) SearchMultipleCollections(ctx context.Context, query string, collections []string, topK int) (map[string]*SearchResponse, error) {
	s.logger.Printf("Searching %d collections for query: %s", len(collections), query)

	results := make(map[string]*SearchResponse)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(collections))

	// Search each collection in parallel
	for _, collection := range collections {
		wg.Add(1)
		go func(coll string) {
			defer wg.Done()

			req := &SearchRequest{
				Query:      query,
				Collection: coll,
				TopK:       topK,
				UseCache:   true,
			}

			resp, err := s.SearchDocuments(ctx, req)
			if err != nil {
				s.logger.Printf("Search failed for collection %s: %v", coll, err)
				errChan <- fmt.Errorf("collection %s: %w", coll, err)
				return
			}

			mu.Lock()
			results[coll] = resp
			mu.Unlock()
		}(collection)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("some collections failed: %v", errs)
	}

	s.logger.Printf("Multi-collection search completed: %d collections, %d total results",
		len(collections), countTotalResults(results))

	return results, nil
}

// GetSimilarChunks finds chunks similar to a given chunk ID
func (s *SearchService) GetSimilarChunks(ctx context.Context, collection string, chunkID string, topK int) (*SearchResponse, error) {
	s.logger.Printf("Finding similar chunks to %s in collection %s", chunkID, collection)

	// Get the source chunk
	chunk, err := s.vectorRepo.GetChunk(ctx, collection, chunkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	// Search using the chunk's embedding
	startTime := time.Now()
	results, err := s.vectorRepo.SearchChunks(ctx, collection, chunk.Embedding, topK+1, nil) // +1 to exclude self
	if err != nil {
		return nil, fmt.Errorf("similarity search failed: %w", err)
	}

	// Filter out the source chunk itself
	filtered := make([]*repositories.SearchResult, 0, len(results)-1)
	for _, r := range results {
		if r.ChunkID != chunkID {
			filtered = append(filtered, r)
		}
	}

	// Limit to topK after filtering
	if len(filtered) > topK {
		filtered = filtered[:topK]
	}

	// Build response
	searchResults := make([]*SearchResult, len(filtered))
	for i, r := range filtered {
		searchResults[i] = &SearchResult{
			ChunkID:    r.ChunkID,
			DocumentID: r.DocumentID,
			Text:       r.Text,
			Score:      r.Score,
			Distance:   r.Distance,
			Metadata:   r.Metadata,
		}
	}

	totalTime := time.Since(startTime).Seconds() * 1000

	return &SearchResponse{
		Results:      searchResults,
		Query:        fmt.Sprintf("similar to chunk: %s", chunkID),
		Collection:   collection,
		TotalResults: len(searchResults),
		SearchTimeMs: totalTime,
		FromCache:    false,
	}, nil
}

// ClearCache clears the search cache
func (s *SearchService) ClearCache() {
	s.cache.Clear()
	s.logger.Printf("Search cache cleared")
}

// GetCacheStats returns cache statistics
func (s *SearchService) GetCacheStats() map[string]interface{} {
	return s.cache.Stats()
}

// validateSearchRequest validates search request parameters
func (s *SearchService) validateSearchRequest(req *SearchRequest) error {
	if req.Query == "" {
		return fmt.Errorf("query is required")
	}

	if req.Collection == "" {
		return fmt.Errorf("collection is required")
	}

	if req.TopK <= 0 {
		req.TopK = 10 // Default
	}

	if req.TopK > 100 {
		return fmt.Errorf("topK cannot exceed 100")
	}

	if req.MinScore != nil && (*req.MinScore < 0 || *req.MinScore > 1) {
		return fmt.Errorf("minScore must be between 0 and 1")
	}

	return nil
}

// Helper function to count total results across collections
func countTotalResults(results map[string]*SearchResponse) int {
	total := 0
	for _, resp := range results {
		total += resp.TotalResults
	}
	return total
}

// ============================================================================
// Search Cache Implementation
// ============================================================================

type searchCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
	hits    int64
	misses  int64
}

type cacheEntry struct {
	response  *SearchResponse
	expiresAt time.Time
}

func newSearchCache(ttl time.Duration) *searchCache {
	cache := &searchCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

func (c *searchCache) cacheKey(req *SearchRequest) string {
	return fmt.Sprintf("%s:%s:%d", req.Collection, req.Query, req.TopK)
}

func (c *searchCache) Get(req *SearchRequest) *SearchResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.cacheKey(req)
	entry, exists := c.entries[key]

	if !exists || time.Now().After(entry.expiresAt) {
		c.misses++
		return nil
	}

	c.hits++
	return entry.response
}

func (c *searchCache) Set(req *SearchRequest, resp *SearchResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.cacheKey(req)
	c.entries[key] = &cacheEntry{
		response:  resp,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *searchCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
	c.hits = 0
	c.misses = 0
}

func (c *searchCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hitRate := float64(0)
	total := c.hits + c.misses
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"hits":     c.hits,
		"misses":   c.misses,
		"size":     len(c.entries),
		"hit_rate": hitRate,
	}
}

func (c *searchCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *searchCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}
