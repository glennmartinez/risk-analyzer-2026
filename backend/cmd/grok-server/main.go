package main

import (
	"log"
	"risk-analyzer/internal/server"
)

func main() {
	log.Println("Starting Grok Server...")
	srv := server.NewServer()
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	log.Println("Grok Server started successfully.")
}
