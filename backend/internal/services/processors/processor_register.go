package processors

import (
	"risk-analyzer/internal/models"
	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"
	"risk-analyzer/internal/services/adapters"
)

// RegisterAll registers the processors for the state machine.
// Call this once during server/app bootstrap and pass the returned error if any.
func RegisterAll(
	sm *services.JobStateMachine,
	py services.PythonClientInterface,
	jobRepo repositories.JobRepository,
	docRepo repositories.DocumentRepository,
	vectorRepo repositories.VectorRepository,
	callbackBase string,
) error {
	// Document Upload processor (async, uses callback)
	uploadProc := NewDocumentUploadProcessor(py, jobRepo, docRepo, callbackBase)
	sm.RegisterProcessor(repositories.JobTypeDocumentUpload, uploadProc)

	// Document Parse processor (sync)
	parseProc := NewDocumentParseProcessor(py, jobRepo, docRepo, callbackBase)
	sm.RegisterProcessor(repositories.JobType(models.JobTypeDocumentParce), parseProc)

	// Content Ingest processors (unified processor for all content types)
	adapterRegistry := adapters.NewContentAdapterRegistry()
	contentProc := NewContentIngestProcessor(py, jobRepo, docRepo, callbackBase, adapterRegistry)
	contentProc.VectorRepo = vectorRepo
	sm.RegisterProcessor(repositories.JobTypeJiraIngest, contentProc)
	sm.RegisterProcessor(repositories.JobTypeNotionIngest, contentProc)
	sm.RegisterProcessor(repositories.JobTypeMarkdownIngest, contentProc)
	sm.RegisterProcessor(repositories.JobTypeJSONIngest, contentProc)

	return nil
}
