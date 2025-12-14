package routes

import (
	"net/http"
	"risk-analyzer/internal/handlers"
)

// RegisterRoutes sets up all application routes
func RegisterRoutes(mux *http.ServeMux) {
	// Health endpoints
	mux.HandleFunc("/health", handlers.HealthCheckHandler)

	// Main routes
	mux.HandleFunc("/", handlers.HomeHandler)

	// Handle issues
	mux.HandleFunc("/issues", handlers.IssuesHandler)
	mux.HandleFunc("/issues/with-keywords", handlers.IssuesWithKeywordsHandler)
	mux.HandleFunc("/issues/keywords-only", handlers.KeywordsOnlyHandler)

	// Future API routes can be added here
	// mux.HandleFunc("/api/v1/analyze", handlers.AnalyzeHandler)
}
