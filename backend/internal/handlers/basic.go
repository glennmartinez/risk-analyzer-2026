// HealthCheckHandler godoc
// @Summary Health check endpoint
// @Description Check if the server is healthy
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} models.BasicResponse
// @Router /health [get]
package handlers

import (
	"encoding/json"
	"net/http"
	"risk-analyzer/internal/models"
)

// HealthCheckHandler godoc
// @Summary Health check endpoint
// @Description Check if the server is healthy
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} models.BasicResponse
// @Router /health [get]
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	response := models.BasicResponse{
		Message: "Server is healthy",
		Status:  "success",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
