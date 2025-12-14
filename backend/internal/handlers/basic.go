package handlers

import (
	"encoding/json"
	"net/http"
	"risk-analyzer/internal/models"
)

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
