package routes

import (
	"net/http"
	"risk-analyzer/internal/handlers"

	"github.com/gorilla/mux"
)

// Handlers holds all HTTP handlers for dependency injection
type Handlers struct {
	// Legacy handlers
	Health                         http.HandlerFunc
	Home                           http.HandlerFunc
	Issues                         http.HandlerFunc
	IssuesWithKeywords             http.HandlerFunc
	KeywordsOnly                   http.HandlerFunc
	Chat                           http.HandlerFunc
	LLMHealth                      http.HandlerFunc
	DocumentsList                  http.HandlerFunc
	DocumentsChunks                http.HandlerFunc
	DocumentsCollectionStats       http.HandlerFunc
	DocumentsProcessExample        http.HandlerFunc
	DocumentsUpload                http.HandlerFunc
	DocumentsSearchResetCollection http.HandlerFunc
	DocumentsSearchCollectionStats http.HandlerFunc
	DocumentsSearchCollections     http.HandlerFunc
	DocumentsSearchQuery           http.HandlerFunc
	DocumentsSearch                http.HandlerFunc
	RAGChat                        http.HandlerFunc
	ListDocuments                  http.HandlerFunc
	ListVectorDocuments            http.HandlerFunc
	DocumentServiceHealth          http.HandlerFunc
	UploadDocument                 http.HandlerFunc
	GetDocumentChunks              http.HandlerFunc
	DeleteCollection               http.HandlerFunc
	DeleteDocument                 http.HandlerFunc

	// New orchestration handlers
	DocHandler        *handlers.DocumentHandler
	SearchHandler     *handlers.SearchHandler
	CollectionHandler *handlers.CollectionHandler
}

// RegisterRoutes sets up all application routes
func RegisterRoutes(router *mux.Router, h *Handlers) {
	// Health endpoints
	router.HandleFunc("/health", h.Health)

	// Main routes
	router.HandleFunc("/", h.Home)

	// Handle issues
	router.HandleFunc("/issues", h.Issues)
	router.HandleFunc("/issues/with-keywords", h.IssuesWithKeywords)
	router.HandleFunc("/issues/keywords-only", h.KeywordsOnly)

	// Chat / LLM routes
	router.HandleFunc("/chat", h.Chat)
	router.HandleFunc("/llm/chat", h.Chat)
	router.HandleFunc("/llm/health", h.LLMHealth)

	// Python backend proxy routes (legacy)
	router.HandleFunc("/documents/list", h.DocumentsList)
	router.HandleFunc("/documents/chunks", h.DocumentsChunks)
	router.HandleFunc("/documents/collection-stats", h.DocumentsCollectionStats)
	router.HandleFunc("/documents/process-example", h.DocumentsProcessExample)
	router.HandleFunc("/documents/upload", h.DocumentsUpload)

	// Search proxy routes (legacy - ordered from most specific to least specific)
	router.HandleFunc("/search/collections/{collection_name}", h.DocumentsSearchResetCollection).Methods("DELETE")
	router.HandleFunc("/search/collections/{collection_name}/stats", h.DocumentsSearchCollectionStats)
	router.HandleFunc("/search/collections", h.DocumentsSearchCollections)
	router.HandleFunc("/search/query", h.DocumentsSearchQuery)
	router.HandleFunc("/search", h.DocumentsSearch)

	// RAG-enabled chat routes
	router.HandleFunc("/chat/rag", h.RAGChat)

	// Microservice document routes (legacy - via Go service layer)
	// Only register if handlers are available
	if h.ListDocuments != nil {
		router.HandleFunc("/api/ms/documents/list", h.ListDocuments).Methods("GET")
	}
	if h.ListVectorDocuments != nil {
		router.HandleFunc("/api/ms/documents/vector", h.ListVectorDocuments).Methods("GET")
	}
	if h.DocumentServiceHealth != nil {
		router.HandleFunc("/api/ms/documents/health", h.DocumentServiceHealth).Methods("GET")
	}
	if h.UploadDocument != nil {
		router.HandleFunc("/api/ms/documents/upload", h.UploadDocument).Methods("POST")
	}
	if h.GetDocumentChunks != nil {
		router.HandleFunc("/api/ms/documents/chunks", h.GetDocumentChunks).Methods("GET")
	}
	if h.DeleteCollection != nil {
		router.HandleFunc("/api/ms/documents/collection/{collection_name}", h.DeleteCollection).Methods("DELETE")
	}
	if h.DeleteDocument != nil {
		router.HandleFunc("/api/ms/documents/{document_id}", h.DeleteDocument).Methods("DELETE")
	}

	// ========================================================================
	// NEW ORCHESTRATION API ROUTES (Phase 3)
	// ========================================================================

	// Document routes - New orchestrated endpoints
	if h.DocHandler != nil {
		router.HandleFunc("/api/v1/documents/upload", h.DocHandler.UploadDocument).Methods("POST")
		router.HandleFunc("/api/v1/documents", h.DocHandler.ListDocuments).Methods("GET")
		router.HandleFunc("/api/v1/documents/{id}", h.DocHandler.GetDocument).Methods("GET")
		router.HandleFunc("/api/v1/documents/{id}", h.DocHandler.DeleteDocument).Methods("DELETE")
		router.HandleFunc("/api/v1/documents/{id}/status", h.DocHandler.GetDocumentStatus).Methods("GET")
		router.HandleFunc("/api/v1/documents/{id}/chunks", h.DocHandler.GetDocumentChunks).Methods("GET")
	}

	// Search routes - New orchestrated endpoints
	if h.SearchHandler != nil {
		router.HandleFunc("/api/v1/search", h.SearchHandler.Search).Methods("POST")
		router.HandleFunc("/api/v1/search", h.SearchHandler.SearchSimple).Methods("GET")
	}

	// Collection routes - New orchestrated endpoints
	if h.CollectionHandler != nil {
		router.HandleFunc("/api/v1/collections", h.CollectionHandler.CreateCollection).Methods("POST")
		router.HandleFunc("/api/v1/collections", h.CollectionHandler.ListCollections).Methods("GET")
		router.HandleFunc("/api/v1/collections/{name}", h.CollectionHandler.GetCollection).Methods("GET")
		router.HandleFunc("/api/v1/collections/{name}", h.CollectionHandler.DeleteCollection).Methods("DELETE")
		router.HandleFunc("/api/v1/collections/{name}/stats", h.CollectionHandler.GetCollectionStats).Methods("GET")
	}
}
