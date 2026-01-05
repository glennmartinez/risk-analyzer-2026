package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	ChunkText(ctx context.Context, text string, strategy string, chunkSize int, chunkOverlap int) (ChunkResult, error)
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
	Chunks []string `json:"chunks"`
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

	// Create document record
	doc := &repositories.Document{
		ID:               payload.DocumentID(),
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

	// Register document
	if err := w.documentRepo.Register(ctx, doc); err != nil {
		return fmt.Errorf("failed to register document: %w", err)
	}

	// Update progress: Parsing document
	w.updateProgress(ctx, job.ID, 10, "Parsing document")

	// Step 1: Parse document
	parseResult, err := w.pythonClient.ParseDocument(
		ctx,
		payload.Filename,
		payload.ExtractMetadata,
		payload.NumQuestions,
		payload.MaxPages,
	)
	if err != nil {
		return fmt.Errorf("failed to parse document: %w", err)
	}

	// Update progress: Chunking text
	w.updateProgress(ctx, job.ID, 30, "Chunking text")

	// Step 2: Chunk text
	chunkResult, err := w.pythonClient.ChunkText(
		ctx,
		parseResult.Text,
		payload.ChunkingStrategy,
		payload.ChunkSize,
		payload.ChunkOverlap,
	)
	if err != nil {
		return fmt.Errorf("failed to chunk text: %w", err)
	}

	if len(chunkResult.Chunks) == 0 {
		return fmt.Errorf("no chunks generated from document")
	}

	// Update progress: Generating embeddings
	w.updateProgress(ctx, job.ID, 50, fmt.Sprintf("Generating embeddings for %d chunks", len(chunkResult.Chunks)))

	// Step 3: Generate embeddings
	embeddingResult, err := w.pythonClient.GenerateEmbeddings(ctx, chunkResult.Chunks)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	if len(embeddingResult.Embeddings) != len(chunkResult.Chunks) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddingResult.Embeddings), len(chunkResult.Chunks))
	}

	// Update progress: Storing in vector database
	w.updateProgress(ctx, job.ID, 70, "Storing chunks in vector database")

	// Step 4: Store in vector database
	chunks := make([]*repositories.Chunk, len(chunkResult.Chunks))
	for i := range chunkResult.Chunks {
		chunks[i] = &repositories.Chunk{
			ID:         fmt.Sprintf("%s-chunk-%d", doc.ID, i),
			DocumentID: doc.ID,
			Text:       chunkResult.Chunks[i],
			Embedding:  embeddingResult.Embeddings[i],
			ChunkIndex: i,
			Metadata: map[string]interface{}{
				"document_id":  doc.ID,
				"filename":     doc.Filename,
				"chunk_index":  i,
				"total_chunks": len(chunkResult.Chunks),
			},
		}
	}

	err = w.vectorRepo.StoreChunks(ctx, payload.Collection, chunks)
	if err != nil {
		return fmt.Errorf("failed to store chunks in vector database: %w", err)
	}

	// Update progress: Finalizing
	w.updateProgress(ctx, job.ID, 90, "Finalizing document")

	// Step 5: Update document record
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

	// Update job error
	job.Error = jobErr.Error()
	job.RetryCount++

	// Check if we should retry
	if job.RetryCount < job.MaxRetries {
		// Retry
		w.logger.Warn("Job failed, will retry (%d/%d): %s - %v", job.RetryCount, job.MaxRetries, job.ID, jobErr)

		err := w.jobRepo.UpdateJobStatus(
			ctx,
			job.ID,
			repositories.JobStatusRetrying,
			0,
			fmt.Sprintf("Failed: %v. Retry %d/%d", jobErr, job.RetryCount, job.MaxRetries),
		)
		if err != nil {
			w.logger.Error("Failed to update job status to retrying: %v", err)
		}

		// Re-enqueue after delay
		time.Sleep(w.config.RetryDelay)
		if err := w.jobRepo.EnqueueJob(ctx, job); err != nil {
			w.logger.Error("Failed to re-enqueue job: %v", err)
		}
	} else {
		// Exceeded max retries
		w.logger.Error("Job failed permanently: %s - %v", job.ID, jobErr)

		err := w.jobRepo.UpdateJobStatus(
			ctx,
			job.ID,
			repositories.JobStatusFailed,
			0,
			fmt.Sprintf("Failed after %d retries: %v", job.MaxRetries, jobErr),
		)
		if err != nil {
			w.logger.Error("Failed to update job status to failed: %v", err)
		}

		// Update document status to failed
		if payload, err := w.parsePayload(job.Payload); err == nil {
			docUpdates := map[string]interface{}{
				"status": repositories.DocumentStatusFailed,
			}
			if err := w.documentRepo.Update(ctx, payload.DocumentID(), docUpdates); err != nil {
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
	Filename         string `json:"filename"`
	FileSize         int64  `json:"file_size"`
	Collection       string `json:"collection"`
	ChunkingStrategy string `json:"chunking_strategy"`
	ChunkSize        int    `json:"chunk_size"`
	ChunkOverlap     int    `json:"chunk_overlap"`
	ExtractMetadata  bool   `json:"extract_metadata"`
	NumQuestions     int    `json:"num_questions"`
	MaxPages         int    `json:"max_pages"`
}

// DocumentID generates a document ID from the payload
func (p *UploadJobPayload) DocumentID() string {
	// Generate a unique document ID based on filename and timestamp
	// In production, you might want to use a UUID or hash
	return fmt.Sprintf("doc-%s-%d", sanitizeFilename(p.Filename), time.Now().UnixNano())
}

// sanitizeFilename removes special characters from filename
func sanitizeFilename(filename string) string {
	// Simple sanitization - replace non-alphanumeric with dash
	result := ""
	for _, c := range filename {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result += string(c)
		} else {
			result += "-"
		}
	}
	return result
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
