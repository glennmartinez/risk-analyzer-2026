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
	_ "risk-analyzer/docs" // This imports the docs package to initialize swagger
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
