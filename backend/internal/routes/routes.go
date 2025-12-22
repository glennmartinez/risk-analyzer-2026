package routes

import (
	"risk-analyzer/internal/handlers"

	"github.com/gorilla/mux"
)

// RegisterRoutes sets up all application routes
func RegisterRoutes(router *mux.Router) {
	// Health endpoints
	router.HandleFunc("/health", handlers.HealthCheckHandler)

	// Main routes
	router.HandleFunc("/", handlers.HomeHandler)

	// Handle issues
	router.HandleFunc("/issues", handlers.IssuesHandler)
	router.HandleFunc("/issues/with-keywords", handlers.IssuesWithKeywordsHandler)
	router.HandleFunc("/issues/keywords-only", handlers.KeywordsOnlyHandler)

	// Chat / LLM routes
	router.HandleFunc("/chat", handlers.ChatHandler)
	router.HandleFunc("/llm/chat", handlers.ChatHandler)
	router.HandleFunc("/llm/health", handlers.LLMHealthHandler)

	// Python backend proxy routes
	router.HandleFunc("/documents/list", handlers.DocumentsListHandler)
	router.HandleFunc("/documents/chunks", handlers.DocumentsChunksHandler)
	router.HandleFunc("/documents/collection-stats", handlers.DocumentsCollectionStatsHandler)
	router.HandleFunc("/documents/process-example", handlers.DocumentsProcessExampleHandler)
	router.HandleFunc("/documents/upload", handlers.DocumentsUploadHandler)

	// Search proxy routes (ordered from most specific to least specific)
	router.HandleFunc("/search/collections/{collection_name}", handlers.DocumentsSearchResetCollectionHandler).Methods("DELETE")
	router.HandleFunc("/search/collections/{collection_name}/stats", handlers.DocumentsSearchCollectionStatsHandler)
	router.HandleFunc("/search/collections", handlers.DocumentsSearchCollectionsHandler)
	router.HandleFunc("/search/query", handlers.DocumentsSearchQueryHandler)
	router.HandleFunc("/search", handlers.DocumentsSearchHandler)

	// RAG-enabled chat routes
	router.HandleFunc("/chat/rag", handlers.RAGChatHandler)

	// Future API routes can be added here
	// mux.HandleFunc("/api/v1/analyze", handlers.AnalyzeHandler)
}
