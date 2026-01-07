package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"

	"github.com/gorilla/mux"
)

// DocumentHandler handles HTTP requests for document operations
type DocumentHandler struct {
	docService *services.DocumentService
	logger     *log.Logger
}

// NewDocumentHandler creates a new document handler
func NewDocumentHandler(docService *services.DocumentService, logger *log.Logger) *DocumentHandler {
	return &DocumentHandler{
		docService: docService,
		logger:     logger,
	}
}

// UploadDocument handles document upload requests
// @Summary Upload a document
// @Description Upload and process a document for vector storage
// @Tags documents
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Document file"
// @Param collection formData string true "Collection name"
// @Param chunking_strategy formData string false "Chunking strategy" default(semantic)
// @Param chunk_size formData int false "Chunk size" default(512)
// @Param chunk_overlap formData int false "Chunk overlap" default(50)
// @Param extract_metadata formData bool false "Extract metadata" default(false)
// @Param num_questions formData int false "Number of questions" default(3)
// @Param max_pages formData int false "Max pages to process" default(0)
// @Param async formData bool false "Process asynchronously" default(false)
// @Success 200 {object} services.UploadDocumentResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/documents/upload [post]
func (h *DocumentHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	h.logger.Printf("Upload request from %s", r.RemoteAddr)

	// Parse multipart form (max 100MB)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		h.logger.Printf("Failed to parse form: %v", err)
		h.sendError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	// Get file
	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.Printf("No file uploaded: %v", err)
		h.sendError(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	// Get file info
	fileSize := header.Size
	filename := header.Filename

	// Parse form parameters
	collection := r.FormValue("collection")
	if collection == "" {
		h.sendError(w, http.StatusBadRequest, "Collection name is required")
		return
	}

	chunkingStrategy := r.FormValue("chunking_strategy")
	if chunkingStrategy == "" {
		chunkingStrategy = "semantic"
	}

	chunkSize := h.getIntParam(r, "chunk_size", 512)
	chunkOverlap := h.getIntParam(r, "chunk_overlap", 50)
	extractMetadata := h.getBoolParam(r, "extract_metadata", false)
	numQuestions := h.getIntParam(r, "num_questions", 3)
	maxPages := h.getIntParam(r, "max_pages", 0)
	async := h.getBoolParam(r, "async", false)

	// Debug logging for form parameters
	h.logger.Printf("DEBUG Form params - chunk_size=%d, chunk_overlap=%d, extract_metadata=%v, num_questions=%d, max_pages=%d, async=%v",
		chunkSize, chunkOverlap, extractMetadata, numQuestions, maxPages, async)
	h.logger.Printf("DEBUG Raw form values - max_pages='%s', extract_metadata='%s', num_questions='%s'",
		r.FormValue("max_pages"), r.FormValue("extract_metadata"), r.FormValue("num_questions"))

	// Create upload request
	req := &services.UploadDocumentRequest{
		Filename:         filename,
		FileContent:      file,
		FileSize:         fileSize,
		Collection:       collection,
		ChunkingStrategy: chunkingStrategy,
		ChunkSize:        chunkSize,
		ChunkOverlap:     chunkOverlap,
		ExtractMetadata:  extractMetadata,
		NumQuestions:     numQuestions,
		MaxPages:         maxPages,
		Async:            async,
	}

	// Upload document
	resp, err := h.docService.UploadDocument(r.Context(), req)
	if err != nil {
		h.logger.Printf("Upload failed: %v", err)
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Upload failed: %v", err))
		return
	}

	h.sendJSON(w, http.StatusOK, resp)
}

// DocumentListResponse represents a list of documents response
type DocumentListResponse struct {
	Documents interface{} `json:"documents"`
	Count     int         `json:"count"`
}

// ListDocuments handles requests to list all documents
// @Summary List documents
// @Description Get a list of all documents
// @Tags documents
// @Produce json
// @Success 200 {object} DocumentListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/documents [get]
func (h *DocumentHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	h.logger.Printf("List documents request")

	// Check for collection filter
	collection := r.URL.Query().Get("collection")

	var docs []*repositories.Document
	var err error

	if collection != "" {
		docs, err = h.docService.ListDocumentsByCollection(r.Context(), collection)
	} else {
		docs, err = h.docService.ListDocuments(r.Context())
	}

	if err != nil {
		h.logger.Printf("Failed to list documents: %v", err)
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list documents: %v", err))
		return
	}

	response := DocumentListResponse{
		Documents: docs,
		Count:     len(docs),
	}
	h.sendJSON(w, http.StatusOK, response)
}

// GetDocument handles requests to get a specific document
// @Summary Get document
// @Description Get document by ID
// @Tags documents
// @Produce json
// @Param id path string true "Document ID"
// @Success 200 {object} repositories.Document
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/documents/{id} [get]
func (h *DocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	h.logger.Printf("Get document: %s", documentID)

	doc, err := h.docService.GetDocument(r.Context(), documentID)
	if err != nil {
		h.logger.Printf("Failed to get document: %v", err)
		if err.Error() == "document not found: "+documentID {
			h.sendError(w, http.StatusNotFound, "Document not found")
		} else {
			h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get document: %v", err))
		}
		return
	}

	h.sendJSON(w, http.StatusOK, doc)
}

// DeleteDocument handles requests to delete a document
// @Summary Delete document
// @Description Delete a document and all its chunks
// @Tags documents
// @Produce json
// @Param id path string true "Document ID"
// @Success 200 {object} SuccessResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/documents/{id} [delete]
func (h *DocumentHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	h.logger.Printf("Delete document: %s", documentID)

	err := h.docService.DeleteDocument(r.Context(), documentID)
	if err != nil {
		h.logger.Printf("Failed to delete document: %v", err)
		if err.Error() == "document not found: "+documentID {
			h.sendError(w, http.StatusNotFound, "Document not found")
		} else {
			h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete document: %v", err))
		}
		return
	}

	h.sendJSON(w, http.StatusOK, SuccessResponse{
		Success: true,
		Message: "Document deleted successfully",
	})
}

// GetDocumentStatus handles requests to get document processing status
// @Summary Get document status
// @Description Get the processing status of a document
// @Tags documents
// @Produce json
// @Param id path string true "Document ID"
// @Success 200 {object} StatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/documents/{id}/status [get]
func (h *DocumentHandler) GetDocumentStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	h.logger.Printf("Get document status: %s", documentID)

	statusDetails, err := h.docService.GetDocumentStatusWithProgress(r.Context(), documentID)
	if err != nil {
		h.logger.Printf("Failed to get document status: %v", err)
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get status: %v", err))
		return
	}

	h.sendJSON(w, http.StatusOK, StatusResponse{
		DocumentID: documentID,
		Status:     statusDetails.Status,
		Progress:   statusDetails.Progress,
		Message:    statusDetails.Message,
		JobID:      statusDetails.JobID,
	})
}

// GetDocumentChunks handles requests to get chunks for a document
// @Summary Get document chunks
// @Description Get all chunks for a specific document from the vector store
// @Tags documents
// @Produce json
// @Param id path string true "Document ID"
// @Param collection query string false "Collection name (optional, will use document's collection if not provided)"
// @Param limit query int false "Limit results" default(100)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} services.GetDocumentChunksResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/documents/{id}/chunks [get]
func (h *DocumentHandler) GetDocumentChunks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	if documentID == "" {
		h.sendError(w, http.StatusBadRequest, "Document ID is required")
		return
	}

	collection := r.URL.Query().Get("collection")
	limit := h.getIntQueryParam(r, "limit", 100)
	offset := h.getIntQueryParam(r, "offset", 0)

	h.logger.Printf("Get chunks for document: %s, collection: %s, limit: %d, offset: %d", documentID, collection, limit, offset)

	resp, err := h.docService.GetDocumentChunks(r.Context(), documentID, collection, limit, offset)
	if err != nil {
		h.logger.Printf("Failed to get document chunks: %v", err)
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get chunks: %v", err))
		return
	}

	h.sendJSON(w, http.StatusOK, resp)
}

// Helper methods

func (h *DocumentHandler) getIntParam(r *http.Request, key string, defaultValue int) int {
	value := r.FormValue(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

func (h *DocumentHandler) getIntQueryParam(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

func (h *DocumentHandler) getBoolParam(r *http.Request, key string, defaultValue bool) bool {
	value := r.FormValue(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}

func (h *DocumentHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Printf("Failed to encode JSON: %v", err)
	}
}

func (h *DocumentHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Status:  status,
	})
}

// Response types

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type StatusResponse struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
	Progress   int    `json:"progress,omitempty"`
	Message    string `json:"message,omitempty"`
	JobID      string `json:"job_id,omitempty"`
}
