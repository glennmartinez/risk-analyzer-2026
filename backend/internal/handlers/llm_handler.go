package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"risk-analyzer/internal/models"
	"risk-analyzer/internal/services"
)

var llmService = services.NewLLMService()

// ChatHandler handles chat requests from the frontend
// ChatHandler godoc
// @Summary Chat with LLM
// @Description Send a message to the LLM and get a response
// @Tags chat
// @Accept json
// @Produce json
// @Param request body models.ChatRequest true "Chat request with message and optional history"
// @Success 200 {object} models.ChatResponse
// @Failure 400 {object} models.ChatResponse
// @Failure 500 {object} models.ChatResponse
// @Router /chat [post]
func ChatHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var request models.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ChatResponse{
			Message: "Invalid request body: " + err.Error(),
			Status:  "error",
		})
		return
	}

	// Validate message
	if request.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ChatResponse{
			Message: "Message is required",
			Status:  "error",
		})
		return
	}

	// Call LLM service
	response, err := llmService.Chat(r.Context(), request)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ChatResponse{
			Message: "Failed to get response from LLM: " + err.Error(),
			Status:  "error",
		})
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// LLMHealthHandler checks if LM Studio is available
// LLMHealthHandler godoc
// @Summary Check LLM health
// @Description Check if the LLM service (LM Studio) is available
// @Tags chat
// @Accept json
// @Produce json
// @Success 200 {object} models.ChatResponse
// @Failure 503 {object} models.ChatResponse
// @Router /llm/health [get]
func LLMHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if err := llmService.HealthCheck(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(models.ChatResponse{
			Message: "LM Studio is not available: " + err.Error(),
			Status:  "error",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.ChatResponse{
		Message: "LM Studio is available",
		Status:  "success",
	})
}

// RAGChatHandler handles chat requests with automatic document retrieval
// RAGChatHandler godoc
// @Summary Chat with RAG (Retrieval-Augmented Generation)
// @Description Send a message to the LLM with automatic document retrieval for context
// @Tags chat
// @Accept json
// @Produce json
// @Param request body models.RAGChatRequest true "RAG chat request with message and optional RAG settings"
// @Success 200 {object} models.RAGChatResponse
// @Failure 400 {object} models.RAGChatResponse
// @Failure 500 {object} models.RAGChatResponse
// @Router /chat/rag [post]
func RAGChatHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var request models.RAGChatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.RAGChatResponse{
			Message: "Invalid request body: " + err.Error(),
			Status:  "error",
		})
		return
	}

	// Validate message
	if request.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.RAGChatResponse{
			Message: "Message is required",
			Status:  "error",
		})
		return
	}

	// Set defaults
	useRAG := request.UseRAG
	if !request.UseRAG { // Default to true if not specified
		useRAG = true
	}
	maxChunks := request.MaxChunks
	if maxChunks == 0 {
		maxChunks = 3
	}
	collectionName := request.CollectionName
	if collectionName == "" {
		collectionName = "documents"
	}

	// Prepare response
	ragResponse := models.RAGChatResponse{
		Query: request.Message,
	}

	// If RAG is enabled, search for relevant chunks
	if useRAG {
		contextChunks, err := searchDocumentsForContext(request.Message, maxChunks, collectionName)
		if err != nil {
			// Log error but continue without context
			ragResponse.Context = []models.RAGContextChunk{}
		} else {
			ragResponse.Context = contextChunks
		}
	}

	// Build enhanced prompt with context
	enhancedRequest := buildEnhancedChatRequest(request, ragResponse.Context)

	// Call LLM service with enhanced context
	response, err := llmService.Chat(r.Context(), enhancedRequest)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.RAGChatResponse{
			Message: "Failed to get response from LLM: " + err.Error(),
			Status:  "error",
			Query:   request.Message,
		})
		return
	}

	// Return enhanced response
	ragResponse.Message = response.Message
	ragResponse.Status = response.Status

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ragResponse)
}

// searchDocumentsForContext searches the Python backend for relevant document chunks
func searchDocumentsForContext(query string, maxChunks int, collectionName string) ([]models.RAGContextChunk, error) {
	// Build Python backend URL - use the GET /search/query endpoint
	pythonURL, err := url.Parse("http://localhost:8000/search/query")
	if err != nil {
		return nil, fmt.Errorf("invalid Python backend URL: %v", err)
	}

	// Add query parameters
	params := url.Values{}
	params.Add("q", query)
	params.Add("collection", collectionName)
	params.Add("top_k", fmt.Sprintf("%d", maxChunks))
	pythonURL.RawQuery = params.Encode()

	// Make request to Python backend
	resp, err := http.Get(pythonURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Python backend: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse JSON response - Python returns VectorSearchResponse with results field
	var searchResponse struct {
		Query   string `json:"query"`
		Results []struct {
			ChunkID  string                 `json:"chunk_id"`
			Text     string                 `json:"text"`
			Score    float64                `json:"score"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"results"`
		TotalResults      int     `json:"total_results"`
		SearchTimeSeconds float64 `json:"search_time_seconds"`
	}

	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %v", err)
	}

	// Convert to RAGContextChunk format
	var contextChunks []models.RAGContextChunk
	for _, result := range searchResponse.Results {
		chunk := models.RAGContextChunk{
			Text:     result.Text,
			Score:    result.Score,
			Metadata: result.Metadata,
		}
		contextChunks = append(contextChunks, chunk)
	}

	return contextChunks, nil
}

// buildEnhancedChatRequest builds a ChatRequest with context from retrieved chunks
func buildEnhancedChatRequest(request models.RAGChatRequest, context []models.RAGContextChunk) models.ChatRequest {
	// Build context string from chunks
	var contextText string
	if len(context) > 0 {
		contextText = "Here is relevant information from the documents:\n\n"
		for i, chunk := range context {
			contextText += fmt.Sprintf("[Document %d]: %s\n\n", i+1, chunk.Text)
		}
		contextText += "\nPlease use this information to answer the user's question.\n\n"
	}

	// Create enhanced message with context
	enhancedMessage := contextText + "User question: " + request.Message

	return models.ChatRequest{
		Message: enhancedMessage,
		History: request.History,
	}
}

// LLMHandler - keeping for backwards compatibility
func LLMHandler(w http.ResponseWriter, r *http.Request) {
	ChatHandler(w, r)
}
