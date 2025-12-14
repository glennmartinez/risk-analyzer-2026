package server

import (
	"net/http"
	"risk-analyzer/internal/routes"
)

func NewServer() *http.Server {
	mux := http.NewServeMux()
	routes.RegisterRoutes(mux)

	return &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
}
