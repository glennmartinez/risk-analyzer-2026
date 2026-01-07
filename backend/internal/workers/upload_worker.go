package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"risk-analyzer/internal/repositories"
)

// UploadWorker processes document upload jobs
type UploadWorker struct {
	*BaseWorker
	jobRepo      repositories.JobRepository
	documentRepo repositories.DocumentRepository
	vectorRepo   repositories.VectorRepository
	pythonClient PythonClient
	logger       Logger
}

// PythonClient defines the interface for calling Python backend
type PythonClient interface {
	ParseDocument(ctx context.Context, filename string, extractMetadata bool, numQuestions int, maxPages int) (ParseResult, error)
	ChunkText(ctx context.Context, text string, strategy string, chunkSize int, chunkOverlap int, extractMetadata bool, numQuestions int) (ChunkResult, error)
	GenerateEmbeddings(ctx context.Context, texts []string) (EmbeddingResult, error)
}

// Logger defines the interface for logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// ParseResult represents the result of document parsing
type ParseResult struct {
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ChunkResult represents the result of text chunking
type ChunkResult struct {
	Chunks   []string                 `json:"chunks"`
	Metadata []map[string]interface{} `json:"metadata,omitempty"`
}

// EmbeddingResult represents the result of embedding generation
type EmbeddingResult struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// UploadWorkerConfig holds configuration for upload worker
type UploadWorkerConfig struct {
	WorkerConfig WorkerConfig
	JobRepo      repositories.JobRepository
	DocumentRepo repositories.DocumentRepository
	VectorRepo   repositories.VectorRepository
	PythonClient PythonClient
	Logger       Logger
}

// NewUploadWorker creates a new upload worker
func NewUploadWorker(config UploadWorkerConfig) *UploadWorker {
	return &UploadWorker{
		BaseWorker:   NewBaseWorker(config.WorkerConfig),
		jobRepo:      config.JobRepo,
		documentRepo: config.DocumentRepo,
		vectorRepo:   config.VectorRepo,
		pythonClient: config.PythonClient,
		logger:       config.Logger,
	}
}

// Start begins processing upload jobs
func (w *UploadWorker) Start(ctx context.Context) error {
	if w.IsRunning() {
		return NewWorkerError(w.Name(), "start", nil, "worker already running")
	}

	w.setRunning(true)
	w.logger.Info("Starting upload worker: %s", w.Name())

	// Start worker goroutines
	for i := 0; i < w.config.Concurrency; i++ {
		go w.processJobs(ctx, i)
	}

	return nil
}

// Stop gracefully shuts down the worker
func (w *UploadWorker) Stop(ctx context.Context) error {
	if !w.IsRunning() {
		return nil
	}

	w.logger.Info("Stopping upload worker: %s", w.Name())

	// Create timeout context for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, w.config.ShutdownTimeout)
	defer cancel()

	// Wait for shutdown or timeout
	<-shutdownCtx.Done()

	w.setRunning(false)
	w.logger.Info("Upload worker stopped: %s", w.Name())

	return nil
}

// processJobs continuously processes jobs from the queue
func (w *UploadWorker) processJobs(ctx context.Context, workerID int) {
	workerName := fmt.Sprintf("%s-goroutine-%d", w.Name(), workerID)
	w.logger.Info("Worker goroutine started: %s", workerName)

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker goroutine stopping: %s", workerName)
			return

		case <-ticker.C:
			if !w.IsRunning() {
				return
			}

			// Try to dequeue a job
			job, err := w.jobRepo.DequeueJob(ctx, repositories.JobTypeDocumentUpload)
			if err != nil {
				w.logger.Error("Failed to dequeue job: %v", err)
				continue
			}

			if job == nil {
				// No jobs available
				continue
			}

			// Process the job
			w.processJob(ctx, job)
		}
	}
}

// processJob processes a single upload job
func (w *UploadWorker) processJob(ctx context.Context, job *repositories.Job) {
	startTime := w.recordJobStart()
	w.logger.Info("Processing job: %s (type: %s)", job.ID, job.Type)

	// Update job with worker ID
	job.WorkerID = w.Name()

	var err error
	if w.config.EnableRecovery {
		err = w.processJobWithRecovery(ctx, job)
	} else {
		err = w.processJobInternal(ctx, job)
	}

	if err != nil {
		w.handleJobFailure(ctx, job, err, startTime)
	} else {
		w.handleJobSuccess(ctx, job, startTime)
	}
}

// processJobWithRecovery wraps job processing with panic recovery
func (w *UploadWorker) processJobWithRecovery(ctx context.Context, job *repositories.Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &WorkerPanicError{Panic: r}
			w.logger.Error("Panic in job processing: %v", r)
		}
	}()
	return w.processJobInternal(ctx, job)
}

// processJobInternal performs the actual job processing
func (w *UploadWorker) processJobInternal(ctx context.Context, job *repositories.Job) error {
	// Parse job payload
	payload, err := w.parsePayload(job.Payload)
	if err != nil {
		return fmt.Errorf("invalid job payload: %w", err)
	}

	// Validate file_path exists (permanently fail old jobs without it)
	if payload.FilePath == "" {
		w.logger.Error("Job missing file_path - permanently failing old job: %s", job.ID)
		// Mark job as failed permanently (no retry)
		_ = w.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 100, "Job missing file_path - created before async file saving was implemented")
		// Remove from queue so it doesn't retry
		_ = w.jobRepo.DeleteJob(ctx, job.ID)
		return &WorkerError{
			WorkerName: w.Name(),
			Operation:  "process_job",
			Err:        fmt.Errorf("job missing file_path field - cannot process"),
			Message:    "Old job format - permanently failed",
		}
	}

	// Create document record
	doc := &repositories.Document{
		ID:               payload.DocumentID,
		Filename:         payload.Filename,
		Collection:       payload.Collection,
		Status:           repositories.DocumentStatusProcessing,
		FileSize:         payload.FileSize,
		ChunkingStrategy: payload.ChunkingStrategy,
		ChunkSize:        payload.ChunkSize,
		ChunkOverlap:     payload.ChunkOverlap,
		ExtractMetadata:  payload.ExtractMetadata,
		NumQuestions:     payload.NumQuestions,
		MaxPages:         payload.MaxPages,
	}

	// Register document (skip if already exists - allow reprocessing)
	if err := w.documentRepo.Register(ctx, doc); err != nil {
		// Check if document already exists
		if existingDoc, getErr := w.documentRepo.Get(ctx, payload.DocumentID); getErr == nil && existingDoc != nil {
			w.logger.Info("Document already exists, updating status to processing: %s", payload.DocumentID)
			// Update existing document to processing status
			updates := map[string]interface{}{
				"status": repositories.DocumentStatusProcessing,
			}
			if updateErr := w.documentRepo.Update(ctx, payload.DocumentID, updates); updateErr != nil {
				w.logger.Warn("Failed to update existing document status: %v", updateErr)
			}
		} else {
			return fmt.Errorf("failed to register document: %w", err)
		}
	}

	// Update progress: Starting document processing
	w.updateProgress(ctx, job.ID, 5, "Starting document processing")

	// Debug: Log payload values
	w.logger.Info("DEBUG Payload - extract_metadata=%v, num_questions=%d, max_pages=%d, chunking_strategy=%s",
		payload.ExtractMetadata, payload.NumQuestions, payload.MaxPages, payload.ChunkingStrategy)

	// Step 1: Parse document (pass file path so adapter can read it)
	w.updateProgress(ctx, job.ID, 10, "Parsing document with Docling...")
	parseResult, err := w.pythonClient.ParseDocument(
		ctx,
		payload.FilePath,
		payload.ExtractMetadata,
		payload.NumQuestions,
		payload.MaxPages,
	)
	if err != nil {
		return fmt.Errorf("failed to parse document: %w", err)
	}
	w.updateProgress(ctx, job.ID, 20, fmt.Sprintf("Document parsed: %d characters extracted", len(parseResult.Text)))

	// Step 2: Chunk text
	w.logger.Info("DEBUG ChunkText - strategy=%s, size=%d, overlap=%d, extract_metadata=%v, num_questions=%d",
		payload.ChunkingStrategy, payload.ChunkSize, payload.ChunkOverlap, payload.ExtractMetadata, payload.NumQuestions)

	if payload.ExtractMetadata {
		w.updateProgress(ctx, job.ID, 25, "Chunking text and extracting metadata via LLM (this may take a while)...")
	} else {
		w.updateProgress(ctx, job.ID, 25, "Chunking text...")
	}

	chunkResult, err := w.pythonClient.ChunkText(
		ctx,
		parseResult.Text,
		payload.ChunkingStrategy,
		payload.ChunkSize,
		payload.ChunkOverlap,
		payload.ExtractMetadata,
		payload.NumQuestions,
	)
	if err != nil {
		return fmt.Errorf("failed to chunk text: %w", err)
	}

	if len(chunkResult.Chunks) == 0 {
		return fmt.Errorf("no chunks generated from document")
	}

	w.updateProgress(ctx, job.ID, 50, fmt.Sprintf("Created %d chunks, generating embeddings...", len(chunkResult.Chunks)))

	// Step 3: Generate embeddings
	embeddingResult, err := w.pythonClient.GenerateEmbeddings(ctx, chunkResult.Chunks)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}
	w.updateProgress(ctx, job.ID, 70, fmt.Sprintf("Generated %d embeddings, storing in vector database...", len(embeddingResult.Embeddings)))

	if len(embeddingResult.Embeddings) != len(chunkResult.Chunks) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddingResult.Embeddings), len(chunkResult.Chunks))
	}

	// Step 4: Store in vector database
	chunks := make([]*repositories.Chunk, len(chunkResult.Chunks))
	for i := range chunkResult.Chunks {
		// Build metadata including extracted metadata if available
		chunkMeta := map[string]interface{}{
			"document_id":  doc.ID,
			"filename":     doc.Filename,
			"chunk_index":  i,
			"total_chunks": len(chunkResult.Chunks),
		}

		// Merge extracted metadata (title, keywords, questions) if available
		if i < len(chunkResult.Metadata) && chunkResult.Metadata[i] != nil {
			for k, v := range chunkResult.Metadata[i] {
				chunkMeta[k] = v
			}
		}

		chunks[i] = &repositories.Chunk{
			ID:         fmt.Sprintf("%s-chunk-%d", doc.ID, i),
			DocumentID: doc.ID,
			Text:       chunkResult.Chunks[i],
			Embedding:  embeddingResult.Embeddings[i],
			ChunkIndex: i,
			Metadata:   chunkMeta,
		}
	}

	err = w.vectorRepo.StoreChunks(ctx, payload.Collection, chunks)
	if err != nil {
		return fmt.Errorf("failed to store chunks in vector database: %w", err)
	}

	w.updateProgress(ctx, job.ID, 85, fmt.Sprintf("Stored %d chunks in collection '%s'", len(chunks), payload.Collection))

	// Clean up uploaded file after successful storage
	if err := os.Remove(payload.FilePath); err != nil {
		w.logger.Warn("Failed to remove uploaded file %s: %v", payload.FilePath, err)
	}

	// Step 5: Update document record
	w.updateProgress(ctx, job.ID, 90, "Finalizing document record...")
	updates := map[string]interface{}{
		"chunk_count":         len(chunks),
		"status":              repositories.DocumentStatusCompleted,
		"stored_in_vector_db": true,
		"metadata":            parseResult.Metadata,
	}

	err = w.documentRepo.Update(ctx, doc.ID, updates)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	// Set job result
	result := map[string]interface{}{
		"document_id":        doc.ID,
		"chunk_count":        len(chunks),
		"collection":         payload.Collection,
		"processing_time_ms": time.Since(w.recordJobStart()).Milliseconds(),
		"success":            true,
	}

	err = w.jobRepo.UpdateJobResult(ctx, job.ID, result)
	if err != nil {
		w.logger.Warn("Failed to update job result: %v", err)
	}

	return nil
}

// handleJobSuccess handles a successfully completed job
func (w *UploadWorker) handleJobSuccess(ctx context.Context, job *repositories.Job, startTime time.Time) {
	w.recordJobSuccess(startTime)

	err := w.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusCompleted, 100, "Upload completed successfully")
	if err != nil {
		w.logger.Error("Failed to update job status to completed: %v", err)
	}

	w.logger.Info("Job completed successfully: %s (duration: %v)", job.ID, time.Since(startTime))
}

// handleJobFailure handles a failed job
func (w *UploadWorker) handleJobFailure(ctx context.Context, job *repositories.Job, jobErr error, startTime time.Time) {
	w.recordJobFailure(startTime)

	// Get fresh job from DB to get current retry count
	freshJob, err := w.jobRepo.GetJob(ctx, job.ID)
	if err != nil {
		w.logger.Error("Failed to get job for retry check: %v", err)
		return
	}

	// Increment retry count
	freshJob.RetryCount++
	freshJob.Error = jobErr.Error()

	// Save the updated retry count using UpdateJob
	if err := w.jobRepo.UpdateJob(ctx, freshJob); err != nil {
		w.logger.Error("Failed to update job with retry count: %v", err)
		return
	}

	// Check if we should retry
	if freshJob.RetryCount <= freshJob.MaxRetries {
		// Retry
		w.logger.Warn("Job failed, will retry (%d/%d): %s - %v", freshJob.RetryCount, freshJob.MaxRetries, freshJob.ID, jobErr)

		freshJob.Status = repositories.JobStatusQueued
		freshJob.Message = fmt.Sprintf("Failed: %v. Retry %d/%d", jobErr, freshJob.RetryCount, freshJob.MaxRetries)

		// Re-enqueue after delay
		time.Sleep(w.config.RetryDelay)
		if err := w.jobRepo.EnqueueJob(ctx, freshJob); err != nil {
			w.logger.Error("Failed to re-enqueue job: %v", err)
		}
	} else {
		// Exceeded max retries - permanently failed
		w.logger.Error("Job failed permanently after %d retries: %s - %v", freshJob.MaxRetries, freshJob.ID, jobErr)

		freshJob.Status = repositories.JobStatusFailed
		freshJob.Message = fmt.Sprintf("Failed permanently after %d retries: %v", freshJob.MaxRetries, jobErr)

		// Save final state
		if err := w.jobRepo.UpdateJob(ctx, freshJob); err != nil {
			w.logger.Error("Failed to update job to failed status: %v", err)
		}

		// Update document status to failed
		if payload, err := w.parsePayload(freshJob.Payload); err == nil {
			docUpdates := map[string]interface{}{
				"status": repositories.DocumentStatusFailed,
			}
			if err := w.documentRepo.Update(ctx, payload.DocumentID, docUpdates); err != nil {
				w.logger.Error("Failed to update document status: %v", err)
			}
		}
	}
}

// updateProgress updates job progress
func (w *UploadWorker) updateProgress(ctx context.Context, jobID string, progress int, message string) {
	err := w.jobRepo.SetProgress(ctx, jobID, progress, message)
	if err != nil {
		w.logger.Warn("Failed to update job progress: %v", err)
	}
}

// parsePayload parses the job payload into UploadJobPayload
func (w *UploadWorker) parsePayload(payload map[string]interface{}) (*UploadJobPayload, error) {
	// Convert to JSON and back to get proper types
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var uploadPayload UploadJobPayload
	if err := json.Unmarshal(jsonData, &uploadPayload); err != nil {
		return nil, err
	}

	return &uploadPayload, nil
}

// UploadJobPayload represents the payload for document upload jobs
type UploadJobPayload struct {
	DocumentID       string `json:"document_id"`
	Filename         string `json:"filename"`
	FilePath         string `json:"file_path"`
	FileSize         int64  `json:"file_size"`
	Collection       string `json:"collection"`
	ChunkingStrategy string `json:"chunking_strategy"`
	ChunkSize        int    `json:"chunk_size"`
	ChunkOverlap     int    `json:"chunk_overlap"`
	ExtractMetadata  bool   `json:"extract_metadata"`
	NumQuestions     int    `json:"num_questions"`
	MaxPages         int    `json:"max_pages"`
}

// DefaultLogger is a simple logger implementation
type DefaultLogger struct{}

func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] "+msg, args...)
}
