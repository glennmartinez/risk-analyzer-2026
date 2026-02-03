package processors

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"risk-analyzer/internal/models"
	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"

	"github.com/google/uuid"
)

type DocumentParseProcessor struct {
	BaseProcessor
}

func NewDocumentParseProcessor(py services.PythonClientInterface, jr repositories.JobRepository, dr repositories.DocumentRepository, cbBase string) *DocumentParseProcessor {
	bp := BaseProcessor{Py: py, JobRepo: jr, DocRepo: dr, CallbackBase: cbBase, CallbackPath: "/api/v1/documents/upload-callback"}
	return &DocumentParseProcessor{BaseProcessor: bp}
}

func (p *DocumentParseProcessor) StartProcessing(ctx context.Context, job *repositories.Job) error {
	fp, _ := job.Payload["file_path"].(string)
	if fp == "" {
		return fmt.Errorf("missing file_path")
	}

	// Read file bytes from disk
	fileBytes, err := os.ReadFile(fp)
	if err != nil {
		return fmt.Errorf("failed to read file '%s': %w", fp, err)
	}

	fn, _ := job.Payload["filename"].(string)
	if fn == "" {
		return fmt.Errorf("missing filename")
	}

	// Kick off an asynchronous parse job in the Python service and provide
	// our callback URL + the Go job ID so Python can POST progress/result.
	pythonJobID, status, err := p.Py.CreateParseJobWithCallback(ctx, fileBytes, fn, true, 0, p.CallbackBase+p.CallbackPath, job.ID)
	if err != nil {
		// Persist failure and mark job failed so state machine can handle retries/errors.
		_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, err.Error())
		return fmt.Errorf("parse failed: %w", err)
	}

	// Persist the mapping between the Go job and the Python job so callbacks
	// can be correlated. Mark the job as processing locally and include the
	// Python job id/status in the job result for future lookup.
	_ = p.JobRepo.UpdateJobResult(ctx, job.ID, map[string]interface{}{
		"python_job_id": pythonJobID,
		"python_status": status,
	})
	_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusProcessing, 0, "Parse delegated to Python (async)")

	// Important: do NOT update document metadata here. The Python service will
	// POST metadata in the callback which will be handled by HandleCallback.
	return nil
}

func (p *DocumentParseProcessor) HandleCallback(ctx context.Context, job *repositories.Job, payload map[string]interface{}) error {
	log.Printf("DocumentParseProcessor HandleCallback called for job %s", job.ID)

	// Persist metadata if present
	if md, ok := payload["metadata"].(map[string]interface{}); ok && md != nil {
		if docID, ok2 := job.Payload["document_id"].(string); ok2 {
			_ = p.DocRepo.Update(ctx, docID, map[string]interface{}{"metadata": md})
		} else {
			// Fallback: stringify document_id if it's not a string
			_ = p.DocRepo.Update(ctx, fmt.Sprint(job.Payload["document_id"]), map[string]interface{}{"metadata": md})
		}
	}

	// If parse produced chunk_manifest_url -> create chunking job
	if result, ok := payload["result"].(map[string]interface{}); ok && result != nil {
		if manifestRaw, ok2 := result["chunk_manifest_url"]; ok2 {
			if manifest, ok3 := manifestRaw.(string); ok3 && manifest != "" {
				nextJob := &repositories.Job{
					ID:     uuid.New().String(),
					Type:   repositories.JobType(models.JobTypeDocumentChunking),
					Status: repositories.JobStatusPending,
					Payload: map[string]interface{}{
						"document_id":        job.Payload["document_id"],
						"chunk_manifest_url": manifest,
						"collection":         job.Payload["collection"],
					},
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
					MaxRetries: 3,
				}
				_ = p.JobRepo.CreateJob(ctx, nextJob)
				// Use repository/state machine enqueue to keep transitions consistent
				_ = p.JobRepo.EnqueueJob(ctx, nextJob)
			}
		}
	}

	return nil
}
