package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// getPythonBackendURL returns the Python backend URL from environment or default
func getPythonBackendURL() string {
	url := os.Getenv("PYTHON_BACKEND_URL")
	if url == "" {
		url = "http://localhost:8001"
	}
	return url
}

// proxyRequest forwards the request to the Python backend
func proxyRequest(w http.ResponseWriter, r *http.Request, pythonPath string) {
	// Build the Python backend URL
	pythonURL, err := url.Parse(getPythonBackendURL() + pythonPath)
	if err != nil {
		http.Error(w, "Invalid Python backend URL", http.StatusInternalServerError)
		return
	}

	// Copy query parameters
	pythonURL.RawQuery = r.URL.RawQuery

	// Create the request to Python backend
	var reqBody io.Reader
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		reqBody = bytes.NewReader(body)
	}

	pythonReq, err := http.NewRequest(r.Method, pythonURL.String(), reqBody)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			pythonReq.Header.Add(key, value)
		}
	}

	// Make the request to Python backend
	client := &http.Client{}
	resp, err := client.Do(pythonReq)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to connect to Python backend: %v", err)
		http.Error(w, errorMsg, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers, but skip CORS headers as they are handled by the middleware
	corsHeaders := map[string]bool{
		"Access-Control-Allow-Origin":      true,
		"Access-Control-Allow-Methods":     true,
		"Access-Control-Allow-Headers":     true,
		"Access-Control-Allow-Credentials": true,
	}

	for key, values := range resp.Header {
		if !corsHeaders[key] {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}
	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

// DocumentsListHandler godoc
// @Summary [DEPRECATED] List all documents
// @Description DEPRECATED: Use /api/v1/documents instead. Get a list of all documents stored in the vector database
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Param collection_name query string false "Collection name" default(documents)
// @Success 200 {object} map[string]interface{}
// @Router /documents/list [get]
func DocumentsListHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/documents/list")
}

// DocumentsChunksHandler godoc
// @Summary [DEPRECATED] Get document chunks
// @Description DEPRECATED: Legacy endpoint. Get chunks from the vector store with pagination
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Param collection_name query string false "Collection name" default(documents)
// @Param document_id query string false "Filter by document ID"
// @Param limit query int false "Maximum number of chunks" default(100)
// @Param offset query int false "Number of chunks to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /documents/chunks [get]
func DocumentsChunksHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/documents/chunks")
}

// DocumentsCollectionStatsHandler godoc
// @Summary [DEPRECATED] Get collection statistics
// @Description DEPRECATED: Use /api/v1/collections/{name}/stats instead. Get statistics about the vector store collection
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Param collection_name query string false "Collection name" default(documents)
// @Success 200 {object} map[string]interface{}
// @Router /documents/collection-stats [get]
func DocumentsCollectionStatsHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/documents/collection-stats")
}

// DocumentsProcessExampleHandler godoc
// @Summary [DEPRECATED] Process example PDF
// @Description DEPRECATED: Legacy endpoint. Process the bundled example PDF through the full pipeline
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Param chunking_strategy query string false "Chunking strategy" default(sentence)
// @Param chunk_size query int false "Chunk size" default(512)
// @Param chunk_overlap query int false "Chunk overlap" default(50)
// @Param store_in_vector_db query bool false "Store in vector DB" default(true)
// @Param collection_name query string false "Collection name" default(documents)
// @Success 200 {object} map[string]interface{}
// @Router /documents/process-example [post]
func DocumentsProcessExampleHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/documents/process-example")
}

// DocumentsUploadHandler godoc
// @Summary [DEPRECATED] Upload and process document
// @Description DEPRECATED: Use /api/v1/documents/upload instead. Upload a PDF or document file and process it
// @Tags deprecated
// @Deprecated
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Document file to upload"
// @Param chunking_strategy query string false "Chunking strategy" default(sentence)
// @Param chunk_size query int false "Chunk size" default(512)
// @Param chunk_overlap query int false "Chunk overlap" default(50)
// @Param store_in_vector_db query bool false "Store in vector DB" default(false)
// @Param collection_name query string false "Collection name" default(documents)
// @Success 200 {object} map[string]interface{}
// @Router /documents/upload [post]
func DocumentsUploadHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/documents/upload")
}

// DocumentsSearchHandler godoc
// @Summary [DEPRECATED] Search documents
// @Description DEPRECATED: Use /api/v1/search instead. Search for similar chunks in the vector store
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param collection_name query string false "Collection name" default(documents)
// @Param top_k query int false "Number of results" default(5)
// @Success 200 {object} map[string]interface{}
// @Router /search [get]
func DocumentsSearchHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/search")
}

// DocumentsSearchQueryHandler godoc
// @Summary [DEPRECATED] Quick search documents
// @Description DEPRECATED: Use /api/v1/search instead. Perform a quick search across documents
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param top_k query int false "Number of results" default(5)
// @Param collection query string false "Collection name"
// @Param document_id query string false "Filter by document ID"
// @Success 200 {object} map[string]interface{}
// @Router /search/query [get]
func DocumentsSearchQueryHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/search/query")
}

// DocumentsSearchCollectionsHandler godoc
// @Summary [DEPRECATED] List search collections
// @Description DEPRECATED: Use /api/v1/collections instead. Get all vector store collections
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/collections [get]
func DocumentsSearchCollectionsHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/search/collections")
}

// DocumentsSearchCollectionStatsHandler godoc
// @Summary [DEPRECATED] Get collection statistics
// @Description DEPRECATED: Use /api/v1/collections/{name}/stats instead. Get statistics for a specific collection
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Param collection_name path string true "Collection name"
// @Success 200 {object} map[string]interface{}
// @Router /search/collections/{collection_name}/stats [get]
func DocumentsSearchCollectionStatsHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/search/collections/stats")
}

// DocumentsSearchResetCollectionHandler godoc
// @Summary [DEPRECATED] Reset collection
// @Description DEPRECATED: Use /api/v1/collections/{name} DELETE instead. Delete and reset a collection
// @Tags deprecated
// @Deprecated
// @Accept json
// @Produce json
// @Param collection_name path string true "Collection name"
// @Success 200 {object} map[string]interface{}
// @Router /search/collections/{collection_name} [delete]
func DocumentsSearchResetCollectionHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/search/collections/reset")
}
