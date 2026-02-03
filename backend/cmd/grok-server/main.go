// Package main Risk Analyzer API Server
//
//	@title			Risk Analyzer API
//	@version		1.0
//	@description	A comprehensive API for document processing, analysis, and risk assessment
//	@termsOfService	http://swagger.io/terms/
//
//	@contact.name	API Support
//	@contact.url	http://www.swagger.io/support
//	@contact.email	support@swagger.io
//
//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html
//
//	@host		localhost:8080
//	@BasePath	/
//
//	@externalDocs.description	OpenAPI
//	@externalDocs.url			https://swagger.io/resources/open-api/
package main

import (
	"log"
	"os"
	"path/filepath"

	_ "risk-analyzer/docs" // This imports the docs package to initialize swagger
	"risk-analyzer/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists (from project root)
	envPath := filepath.Join("..", ".env")
	if err := godotenv.Load(envPath); err != nil {
		// .env file is optional - only log if in development
		if os.Getenv("GO_ENV") != "production" {
			log.Printf("Note: No .env file found at %s (this is optional)", envPath)
		}
	} else {
		log.Println("Loaded environment variables from .env file")
	}

	log.Println("Starting Grok Server...")
	srv := server.NewServer()
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	log.Println("Grok Server started successfully.")
}
