package processors

import (
	"risk-analyzer/internal/models"
	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"
)

// RegisterAll registers the processors for the state machine.
// Call this once during server/app bootstrap and pass the returned error if any.
func RegisterAll(
	sm *services.JobStateMachine,
	py services.PythonClientInterface, // concrete client from your services package
	jobRepo repositories.JobRepository,
	docRepo repositories.DocumentRepository,
	callbackBase string,
) error {
	// Document Upload processor (async, uses callback)
	uploadProc := NewDocumentUploadProcessor(py, jobRepo, docRepo, callbackBase)
	sm.RegisterProcessor(repositories.JobTypeDocumentUpload, uploadProc)

	// Document Parse processor (sync)
	parseProc := NewDocumentParseProcessor(py, jobRepo, docRepo, callbackBase)
	// models.JobTypeDocumentParce is defined in models (string "document_parse")
	sm.RegisterProcessor(repositories.JobType(models.JobTypeDocumentParce), parseProc)

	return nil
}
