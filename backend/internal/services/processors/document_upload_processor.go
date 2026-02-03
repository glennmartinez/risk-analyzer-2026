package processors

import (
	"context"
	"fmt"
	"log"
	"time"

	"risk-analyzer/internal/models"
	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"

	"github.com/google/uuid"
)

type DocumentUploadProcessor struct {
	BaseProcessor
}

func NewDocumentUploadProcessor(py services.PythonClientInterface, jr repositories.JobRepository, dr repositories.DocumentRepository, cbBase string) *DocumentUploadProcessor {
	bp := BaseProcessor{Py: py, JobRepo: jr, DocRepo: dr, CallbackBase: cbBase, CallbackPath: "/api/v1/documents/upload-callback"}
	return &DocumentUploadProcessor{BaseProcessor: bp}
}

func (p *DocumentUploadProcessor) StartProcessing(ctx context.Context, job *repositories.Job) error {
	// log job receipt
	log.Printf("Document Upload processor starting job with Id: %s", job.ID)

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
		// log that job failed
		log.Printf("Document Upload processor job %s failed to kickoff: %v", job.ID, err)
		_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, err.Error())
		return fmt.Errorf("python kickoff failed: %w", err)
	}

	// persist python id/status
	_ = p.JobRepo.UpdateJobResult(ctx, job.ID, map[string]interface{}{"python_job_id": pythonJobID, "python_status": status})
	// mark upload job as completed (processing will continue downstream via callbacks)
	_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusCompleted, 100, "Document upload processing started")
	return nil
}

// HandleCallback persists results from the Python upload step and schedules a parse job.
func (p *DocumentUploadProcessor) HandleCallback(ctx context.Context, job *repositories.Job, payload map[string]interface{}) error {
	log.Printf("Document Upload processor handling callback for job Id: %s", job.ID)
	// Idempotency: if already processed, no-op
	if job != nil && job.Result != nil {
		if processed, ok := job.Result["callback_processed"].(bool); ok && processed {
			return nil
		}
	}

	docID := fmt.Sprint(job.Payload["document_id"])

	// Persist any metadata present in payload
	if md, ok := payload["metadata"].(map[string]interface{}); ok && md != nil {
		_ = p.DocRepo.Update(ctx, docID, map[string]interface{}{"metadata": md})
	}

	// If the payload contains a parse_response (Python returned full parse), save it directly
	if result, ok := payload["result"].(map[string]interface{}); ok && result != nil {
		if pr, ok := result["parse_response"].(map[string]interface{}); ok && pr != nil {
			_ = p.DocRepo.Update(ctx, docID, map[string]interface{}{"parse_response": pr})
			// also persist nested metadata if present
			if md, ok := pr["metadata"].(map[string]interface{}); ok && md != nil {
				_ = p.DocRepo.Update(ctx, docID, map[string]interface{}{"metadata": md})
			}
		}
	}

	// Schedule a parse job if file_path is available (use the original upload job payload)
	filePath, _ := job.Payload["file_path"].(string)
	filename, _ := job.Payload["filename"].(string)
	collection := job.Payload["collection"]

	if filePath != "" {
		nextJob := &repositories.Job{
			ID:     uuid.New().String(),
			Type:   repositories.JobType(models.JobTypeDocumentParce),
			Status: repositories.JobStatusPending,
			Payload: map[string]interface{}{
				"document_id": docID,
				"filename":    filename,
				"file_path":   filePath,
				"collection":  collection,
			},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			MaxRetries: 3,
		}

		if err := p.JobRepo.CreateJob(ctx, nextJob); err != nil {
			// If create fails, bubble up error so it can be retried/inspected
			return fmt.Errorf("failed to create parse job: %w", err)
		}

		if err := p.JobRepo.EnqueueJob(ctx, nextJob); err != nil {
			return fmt.Errorf("failed to enqueue parse job: %w", err)
		}

		// Update document status to queued for parse
		_ = p.DocRepo.Update(ctx, docID, map[string]interface{}{"status": repositories.DocumentStatusPending})
	} else {
		log.Printf("Upload callback for job %s: no file_path available to schedule parse job", job.ID)
	}

	// Mark callback processed and persist python_job_id if present
	updateMap := map[string]interface{}{"callback_processed": true}
	if pj, ok := payload["python_job_id"].(string); ok && pj != "" {
		updateMap["python_job_id"] = pj
	}
	_ = p.JobRepo.UpdateJobResult(ctx, job.ID, updateMap)

	return nil
}
