package processors

import (
	"context"
	"fmt"

	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"
)

type DocumentUploadProcessor struct {
	BaseProcessor
}

func NewDocumentUploadProcessor(py services.PythonClientInterface, jr repositories.JobRepository, dr repositories.DocumentRepository, cbBase string) *DocumentUploadProcessor {
	bp := BaseProcessor{Py: py, JobRepo: jr, DocRepo: dr, CallbackBase: cbBase, CallbackPath: "/api/v1/documents/upload-callback"}
	return &DocumentUploadProcessor{BaseProcessor: bp}
}

func (p *DocumentUploadProcessor) StartProcessing(ctx context.Context, job *repositories.Job) error {
	//log job receipt
	fmt.Printf("Document Upload processor starting job with Id: %s", job.ID)

	cbURL := p.CallbackURL() // shared helper from BaseProcessor

	payload := services.DocumentCallbackPayload{
		DocumentID:  fmt.Sprint(job.Payload["document_id"]),
		CallbackUrl: cbURL,
		Status:      "processing",
		Message:     "Kickoff from Go",
		Job:         job,
	}

	pythonJobID, status, err := p.Py.CreateJobWithCallback(ctx, payload)
	if err != nil {
		return fmt.Errorf("python kickoff failed: %w", err)
	}

	// persist python id/status
	_ = p.JobRepo.UpdateJobResult(ctx, job.ID, map[string]interface{}{"python_job_id": pythonJobID, "python_status": status})
	_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusCompleted, 100, "Document upload processing started")
	return nil
}

// Optionally override callback handling if upload processor needs to do per-job work
func (p *DocumentUploadProcessor) HandleCallback(ctx context.Context, job *repositories.Job, payload map[string]interface{}) error {
	// Example: persist extracted metadata/chunk count if present in payload
	if md, ok := payload["metadata"].(map[string]interface{}); ok && md != nil {
		_ = p.DocRepo.Update(ctx, job.Payload["document_id"].(string), map[string]interface{}{"metadata": md})
	}
	return nil
}
