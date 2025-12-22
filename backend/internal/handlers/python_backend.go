package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Python backend URL - should be configurable
const PYTHON_BACKEND_URL = "http://localhost:8000"

// proxyRequest forwards the request to the Python backend
func proxyRequest(w http.ResponseWriter, r *http.Request, pythonPath string) {
	// Build the Python backend URL
	pythonURL, err := url.Parse(PYTHON_BACKEND_URL + pythonPath)
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
// @Summary List all documents
// @Description Get a list of all documents stored in the vector database
// @Tags documents
// @Accept json
// @Produce json
// @Param collection_name query string false "Collection name" default(documents)
// @Success 200 {object} map[string]interface{}
// @Router /documents/list [get]
func DocumentsListHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/documents/list")
}

// DocumentsChunksHandler godoc
// @Summary Get document chunks
// @Description Get chunks from the vector store with pagination
// @Tags documents
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
// @Summary Get collection statistics
// @Description Get statistics about the vector store collection
// @Tags documents
// @Accept json
// @Produce json
// @Param collection_name query string false "Collection name" default(documents)
// @Success 200 {object} map[string]interface{}
// @Router /documents/collection-stats [get]
func DocumentsCollectionStatsHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/documents/collection-stats")
}

// DocumentsProcessExampleHandler godoc
// @Summary Process example PDF
// @Description Process the bundled example PDF through the full pipeline
// @Tags documents
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
// @Summary Upload and process document
// @Description Upload a PDF or document file and process it
// @Tags documents
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
// @Summary Search documents
// @Description Search for similar chunks in the vector store
// @Tags search
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
// @Summary Quick search documents
// @Description Perform a quick search across document chunks
// @Tags search
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
// @Summary List search collections
// @Description Get all vector store collections
// @Tags search
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/collections [get]
func DocumentsSearchCollectionsHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/search/collections")
}

// DocumentsSearchCollectionStatsHandler godoc
// @Summary Get collection statistics
// @Description Get statistics for a specific search collection
// @Tags search
// @Accept json
// @Produce json
// @Param collection_name path string true "Collection name"
// @Success 200 {object} map[string]interface{}
// @Router /search/collections/{collection_name}/stats [get]
func DocumentsSearchCollectionStatsHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/search/collections/stats")
}

// DocumentsSearchResetCollectionHandler godoc
// @Summary Reset collection
// @Description Delete and recreate a search collection
// @Tags search
// @Accept json
// @Produce json
// @Param collection_name path string true "Collection name"
// @Success 200 {object} map[string]interface{}
// @Router /search/collections/{collection_name} [delete]
func DocumentsSearchResetCollectionHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "/search/collections/reset")
}
