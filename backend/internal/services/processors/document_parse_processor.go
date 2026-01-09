package processors

import (
	"context"
	"fmt"
	"os"

	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"
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

	resp, err := p.Py.ParseDocument(ctx, fileBytes, fn, true, 0)
	if err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}

	_ = p.JobRepo.UpdateJobResult(ctx, job.ID, map[string]interface{}{"parsed_chars": len(resp.Text)})
	_ = p.JobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusCompleted, 100, "Parse completed")
	_ = p.DocRepo.Update(ctx, fmt.Sprint(job.Payload["document_id"]), map[string]interface{}{"metadata": resp.Metadata})
	return nil
}
