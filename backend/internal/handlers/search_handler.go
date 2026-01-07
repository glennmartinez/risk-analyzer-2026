package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"risk-analyzer/internal/services"
)

// SearchHandler handles HTTP requests for search operations
type SearchHandler struct {
	searchService *services.SearchService
	logger        *log.Logger
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(searchService *services.SearchService, logger *log.Logger) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		logger:        logger,
	}
}

// Search handles search requests
// @Summary Search documents
// @Description Perform vector similarity search across documents
// @Tags search
// @Accept json
// @Produce json
// @Param query body SearchRequestBody true "Search request"
// @Success 200 {object} services.SearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/search [post]
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	h.logger.Printf("Search request from %s", r.RemoteAddr)

	var reqBody SearchRequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		h.logger.Printf("Failed to decode request: %v", err)
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Build search request
	req := &services.SearchRequest{
		Query:      reqBody.Query,
		Collection: reqBody.Collection,
		TopK:       reqBody.TopK,
		Filter:     reqBody.Filter,
		MinScore:   reqBody.MinScore,
		UseCache:   reqBody.UseCache,
	}

	// Perform search
	resp, err := h.searchService.SearchDocuments(r.Context(), req)
	if err != nil {
		h.logger.Printf("Search failed: %v", err)
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, resp)
}

// SearchSimple handles simple search requests via query parameters
// @Summary Simple search
// @Description Perform a simple search using query parameters
// @Tags search
// @Produce json
// @Param q query string true "Search query"
// @Param collection query string true "Collection name"
// @Param top_k query int false "Number of results" default(10)
// @Param use_cache query bool false "Use cache" default(true)
// @Success 200 {object} services.SearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/search [get]
func (h *SearchHandler) SearchSimple(w http.ResponseWriter, r *http.Request) {
	h.logger.Printf("Simple search request from %s", r.RemoteAddr)

	query := r.URL.Query().Get("q")
	if query == "" {
		h.sendError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	collection := r.URL.Query().Get("collection")
	if collection == "" {
		h.sendError(w, http.StatusBadRequest, "Query parameter 'collection' is required")
		return
	}

	topK := 10
	if topKStr := r.URL.Query().Get("top_k"); topKStr != "" {
		if parsed, err := strconv.Atoi(topKStr); err == nil {
			topK = parsed
		}
	}

	useCache := true
	if useCacheStr := r.URL.Query().Get("use_cache"); useCacheStr != "" {
		if parsed, err := strconv.ParseBool(useCacheStr); err == nil {
			useCache = parsed
		}
	}

	req := &services.SearchRequest{
		Query:      query,
		Collection: collection,
		TopK:       topK,
		UseCache:   useCache,
	}

	// Perform search
	resp, err := h.searchService.SearchDocuments(r.Context(), req)
	if err != nil {
		h.logger.Printf("Search failed: %v", err)
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, resp)
}

// Helper methods

func (h *SearchHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Printf("Failed to encode JSON: %v", err)
	}
}

func (h *SearchHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Status:  status,
	})
}

// Request types

type SearchRequestBody struct {
	Query      string                 `json:"query"`
	Collection string                 `json:"collection"`
	TopK       int                    `json:"top_k,omitempty"`
	Filter     map[string]interface{} `json:"filter,omitempty"`
	MinScore   *float32               `json:"min_score,omitempty"`
	UseCache   bool                   `json:"use_cache"`
}
