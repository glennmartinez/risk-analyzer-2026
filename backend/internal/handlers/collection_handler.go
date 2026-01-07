package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"risk-analyzer/internal/services"

	"github.com/gorilla/mux"
)

// CollectionHandler handles HTTP requests for collection operations
type CollectionHandler struct {
	collectionService *services.CollectionService
	logger            *log.Logger
}

// NewCollectionHandler creates a new collection handler
func NewCollectionHandler(collectionService *services.CollectionService, logger *log.Logger) *CollectionHandler {
	return &CollectionHandler{
		collectionService: collectionService,
		logger:            logger,
	}
}

// CreateCollection handles collection creation requests
// @Summary Create collection
// @Description Create a new vector collection
// @Tags collections
// @Accept json
// @Produce json
// @Param collection body services.CreateCollectionRequest true "Collection request"
// @Success 201 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/collections [post]
func (h *CollectionHandler) CreateCollection(w http.ResponseWriter, r *http.Request) {
	h.logger.Printf("Create collection request")

	var req services.CreateCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Printf("Failed to decode request: %v", err)
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.collectionService.CreateCollection(r.Context(), &req)
	if err != nil {
		h.logger.Printf("Failed to create collection: %v", err)
		if contains(err.Error(), "already exists") {
			h.sendError(w, http.StatusConflict, err.Error())
		} else if contains(err.Error(), "invalid collection name") {
			h.sendError(w, http.StatusBadRequest, err.Error())
		} else {
			h.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.sendJSON(w, http.StatusCreated, SuccessResponse{
		Success: true,
		Message: "Collection created successfully",
	})
}

// ListCollections handles requests to list all collections
// @Summary List collections
// @Description Get a list of all collections
// @Tags collections
// @Produce json
// @Success 200 {object} CollectionsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/collections [get]
func (h *CollectionHandler) ListCollections(w http.ResponseWriter, r *http.Request) {
	h.logger.Printf("List collections request")

	collections, err := h.collectionService.ListCollections(r.Context())
	if err != nil {
		h.logger.Printf("Failed to list collections: %v", err)
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, CollectionsResponse{
		Collections: collections,
		Total:       len(collections),
	})
}

// GetCollection handles requests to get collection info
// @Summary Get collection
// @Description Get detailed information about a collection
// @Tags collections
// @Produce json
// @Param name path string true "Collection name"
// @Success 200 {object} services.CollectionInfo
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/collections/{name} [get]
func (h *CollectionHandler) GetCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	h.logger.Printf("Get collection: %s", name)

	info, err := h.collectionService.GetCollectionInfo(r.Context(), name)
	if err != nil {
		h.logger.Printf("Failed to get collection: %v", err)
		if contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, err.Error())
		} else {
			h.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.sendJSON(w, http.StatusOK, info)
}

// DeleteCollection handles requests to delete a collection
// DeleteCollection handles collection deletion requests
// @Summary Delete collection
// @Description Delete a collection and all its documents
// @Tags collections
// @Produce json
// @Param name path string true "Collection name"
// @Success 200 {object} SuccessResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/collections/{name} [delete]
func (h *CollectionHandler) DeleteCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	h.logger.Printf("Delete collection: %s", name)

	resp, err := h.collectionService.DeleteCollection(r.Context(), name)
	if err != nil {
		h.logger.Printf("Failed to delete collection: %v", err)
		if contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, err.Error())
		} else {
			h.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.sendJSON(w, http.StatusOK, resp)
}

// GetCollectionStats handles requests to get collection statistics
// GetCollectionStats handles collection statistics requests
// @Summary Get collection statistics
// @Description Get detailed statistics for a collection
// @Tags collections
// @Produce json
// @Param name path string true "Collection name"
// @Success 200 {object} SuccessResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/collections/{name}/stats [get]
func (h *CollectionHandler) GetCollectionStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	h.logger.Printf("Get collection stats: %s", name)

	stats, err := h.collectionService.GetCollectionStats(r.Context(), name)
	if err != nil {
		h.logger.Printf("Failed to get collection stats: %v", err)
		if contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, err.Error())
		} else {
			h.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.sendJSON(w, http.StatusOK, stats)
}

// Helper methods

func (h *CollectionHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Printf("Failed to encode JSON: %v", err)
	}
}

func (h *CollectionHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Status:  status,
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Response types

type CollectionsResponse struct {
	Collections []string `json:"collections"`
	Total       int      `json:"total"`
}
