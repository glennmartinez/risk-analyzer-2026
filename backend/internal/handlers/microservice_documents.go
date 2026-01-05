package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"risk-analyzer/internal/services"
)

// UploadDocumentRequest represents the form data for document upload
type UploadDocumentRequest struct {
	ChunkingStrategy string `form:"chunking_strategy" json:"chunking_strategy" example:"sentence"`
	ChunkSize        int    `form:"chunk_size" json:"chunk_size" example:"512"`
	ChunkOverlap     int    `form:"chunk_overlap" json:"chunk_overlap" example:"50"`
	StoreInVectorDB  bool   `form:"store_in_vector_db" json:"store_in_vector_db" example:"true"`
	ExtractTables    bool   `form:"extract_tables" json:"extract_tables" example:"true"`
	ExtractFigures   bool   `form:"extract_figures" json:"extract_figures" example:"true"`
	ExtractMetadata  bool   `form:"extract_metadata" json:"extract_metadata" example:"false"`
	NumQuestions     int    `form:"num_questions" json:"num_questions" example:"3"`
	MaxPages         int    `form:"max_pages" json:"max_pages" example:"30"`
	CollectionName   string `form:"collection_name" json:"collection_name"`
}

// DocumentService instance
var documentService = services.NewDocumentService()

// ListDocumentsHandler godoc
// @Summary List all registered documents
// @Description Get a list of all documents registered in the Redis registry via Python microservice
// @Tags microservice-documents
// @Accept json
// @Produce json
// @Success 200 {object} services.ListDocumentsResponse
// @Failure 500 {object} map[string]string
// @Router /api/ms/documents/list [get]
func ListDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response, err := documentService.ListDocuments(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListVectorDocumentsHandler godoc
// @Summary List all documents in vector store
// @Description Get a list of all unique documents stored in the vector database via Python microservice
// @Tags microservice-documents
// @Accept json
// @Produce json
// @Param collection_name query string false "Collection name"
// @Success 200 {object} services.ListVectorDocumentsResponse
// @Failure 500 {object} map[string]string
// @Router /api/ms/documents/vector [get]
func ListVectorDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	collectionName := r.URL.Query().Get("collection_name")

	response, err := documentService.ListVectorDocuments(ctx, collectionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DocumentServiceHealthHandler godoc
// @Summary Check Python microservice health
// @Description Verify the Python document microservice is running
// @Tags microservice-documents
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/ms/documents/health [get]
func DocumentServiceHealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := documentService.HealthCheck(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// UploadDocumentHandler godoc
// @Summary Upload and process a document
// @Description Upload a PDF or document file to be parsed, chunked, and optionally stored in vector DB
// @Tags microservice-documents
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Document file to upload"
// @Param chunking_strategy formData string false "Chunking strategy (sentence, semantic, markdown, hierarchical)" default(sentence)
// @Param chunk_size formData int false "Target chunk size in tokens" default(512)
// @Param chunk_overlap formData int false "Overlap between chunks" default(50)
// @Param store_in_vector_db formData bool false "Store chunks in vector database" default(true)
// @Param extract_tables formData bool false "Extract tables from document" default(true)
// @Param extract_figures formData bool false "Extract figures from document" default(true)
// @Param extract_metadata formData bool false "Extract metadata via LLM (title, questions, keywords)" default(false)
// @Param num_questions formData int false "Number of questions to generate per chunk" default(3)
// @Param max_pages formData int false "Maximum number of pages to process" default(30)
// @Param collection_name formData string false "Vector DB collection name"
// @Success 200 {object} services.UploadDocumentResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/ms/documents/upload [post]
func UploadDocumentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse multipart form (32MB max)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get uploaded file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Parse request struct from form
	req := parseUploadRequest(r)

	// Convert to service options
	opts := services.UploadDocumentRequest{
		ChunkingStrategy: req.ChunkingStrategy,
		ChunkSize:        req.ChunkSize,
		ChunkOverlap:     req.ChunkOverlap,
		StoreInVectorDB:  req.StoreInVectorDB,
		ExtractTables:    req.ExtractTables,
		ExtractFigures:   req.ExtractFigures,
		ExtractMetadata:  req.ExtractMetadata,
		NumQuestions:     req.NumQuestions,
		MaxPages:         req.MaxPages,
		CollectionName:   req.CollectionName,
	}

	// Upload to Python microservice
	response, err := documentService.UploadDocument(ctx, header.Filename, file, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// parseUploadRequest parses form values into UploadDocumentRequest with defaults
func parseUploadRequest(r *http.Request) UploadDocumentRequest {
	req := UploadDocumentRequest{
		ChunkingStrategy: "sentence",
		ChunkSize:        512,
		ChunkOverlap:     50,
		StoreInVectorDB:  true,
		ExtractTables:    true,
		ExtractFigures:   true,
		ExtractMetadata:  false,
		NumQuestions:     3,
		MaxPages:         30,
		CollectionName:   "",
	}

	if v := r.FormValue("chunking_strategy"); v != "" {
		req.ChunkingStrategy = v
	}
	if v := r.FormValue("chunk_size"); v != "" {
		if i := parseInt(v, 512); i > 0 {
			req.ChunkSize = i
		}
	}
	if v := r.FormValue("chunk_overlap"); v != "" {
		if i := parseInt(v, 50); i >= 0 {
			req.ChunkOverlap = i
		}
	}
	if v := r.FormValue("store_in_vector_db"); v != "" {
		req.StoreInVectorDB = parseBool(v)
	}
	if v := r.FormValue("extract_tables"); v != "" {
		req.ExtractTables = parseBool(v)
	}
	if v := r.FormValue("extract_figures"); v != "" {
		req.ExtractFigures = parseBool(v)
	}
	if v := r.FormValue("extract_metadata"); v != "" {
		req.ExtractMetadata = parseBool(v)
	}
	if v := r.FormValue("num_questions"); v != "" {
		if i := parseInt(v, 3); i > 0 {
			req.NumQuestions = i
		}
	}
	if v := r.FormValue("max_pages"); v != "" {
		if i := parseInt(v, 30); i > 0 {
			req.MaxPages = i
		}
	}
	if v := r.FormValue("collection_name"); v != "" {
		req.CollectionName = v
	}

	return req
}

// parseInt parses string to int, returns defaultVal on error
func parseInt(s string, defaultVal int) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return defaultVal
}

// parseBool parses string to bool
func parseBool(s string) bool {
	return s == "true" || s == "1" || s == "yes"
}

// GetDocumentChunksHandler godoc
// @Summary Get chunks for a document
// @Description Get all chunks for a specific document from the vector store
// @Tags microservice-documents
// @Accept json
// @Produce json
// @Param document_id query string true "Document ID"
// @Param collection_name query string false "Collection name"
// @Param limit query int false "Maximum number of chunks" default(100)
// @Param offset query int false "Number of chunks to skip" default(0)
// @Success 200 {object} services.GetDocumentChunksResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/ms/documents/chunks [get]
func GetDocumentChunksHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	documentID := r.URL.Query().Get("document_id")
	if documentID == "" {
		http.Error(w, "document_id is required", http.StatusBadRequest)
		return
	}

	collectionName := r.URL.Query().Get("collection_name")
	limit := parseInt(r.URL.Query().Get("limit"), 100)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	response, err := documentService.GetDocumentChunks(ctx, documentID, collectionName, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
