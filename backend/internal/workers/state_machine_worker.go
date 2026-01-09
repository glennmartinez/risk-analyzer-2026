package workers

import (
	"context"
	"fmt"
	"time"

	"risk-analyzer/internal/repositories"
	"risk-analyzer/internal/services"
)

// StateMachineWorker processes jobs by delegating state transitions to JobStateMachine.
type StateMachineWorker struct {
	*BaseWorker
	jobRepo         repositories.JobRepository
	jobStateMachine *services.JobStateMachine
	logger          Logger
}

// StateMachineWorkerConfig holds configuration for the state-machine-backed worker.
type StateMachineWorkerConfig struct {
	WorkerConfig    WorkerConfig
	JobRepo         repositories.JobRepository
	JobStateMachine *services.JobStateMachine
	Logger          Logger
}

// NewStateMachineWorker creates a new StateMachineWorker.
func NewStateMachineWorker(cfg StateMachineWorkerConfig) *StateMachineWorker {
	return &StateMachineWorker{
		BaseWorker:      NewBaseWorker(cfg.WorkerConfig),
		jobRepo:         cfg.JobRepo,
		jobStateMachine: cfg.JobStateMachine,
		logger:          cfg.Logger,
	}
}

// Start begins the worker goroutines.
func (w *StateMachineWorker) Start(ctx context.Context) error {
	if w.IsRunning() {
		return NewWorkerError(w.Name(), "start", nil, "worker already running")
	}

	w.setRunning(true)
	w.logger.Info("Starting state-machine worker: %s", w.Name())

	for i := 0; i < w.config.Concurrency; i++ {
		go w.processJobs(ctx, i)
	}

	return nil
}

// Stop gracefully stops the worker.
func (w *StateMachineWorker) Stop(ctx context.Context) error {
	if !w.IsRunning() {
		return nil
	}
	w.logger.Info("Stopping state-machine worker: %s", w.Name())

	shutdownCtx, cancel := context.WithTimeout(ctx, w.config.ShutdownTimeout)
	defer cancel()

	<-shutdownCtx.Done()

	w.setRunning(false)
	w.logger.Info("State-machine worker stopped: %s", w.Name())
	return nil
}

// processJobs polls the queue and dispatches events to the JobStateMachine.
func (w *StateMachineWorker) processJobs(ctx context.Context, workerID int) {
	workerName := fmt.Sprintf("%s-goroutine-%d", w.Name(), workerID)
	w.logger.Info("State-machine worker goroutine started: %s", workerName)

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

			job, err := w.jobRepo.DequeueJob(ctx, repositories.JobTypeDocumentUpload)
			if err != nil {
				w.logger.Error("Failed to dequeue job: %v", err)
				continue
			}
			if job == nil {
				continue
			}

			startTime := w.recordJobStart()
			// Update job worker id (optional)
			job.WorkerID = w.Name()

			// Delegate to state machine
			if w.jobStateMachine == nil {
				// Fallback: no state machine configured
				w.logger.Error("No job state machine configured; cannot process job %s", job.ID)
				w.handleKickoffFailure(ctx, job, fmt.Errorf("no job state machine configured"), startTime)
				continue
			}

			if err := w.jobStateMachine.HandleEvent(ctx, job.ID, services.EventStart, nil); err != nil {
				w.logger.Error("State machine failed to start job %s: %v", job.ID, err)
				w.handleKickoffFailure(ctx, job, err, startTime)
				continue
			}

			// If kickoff succeeded, record success for stats.
			w.recordJobSuccess(startTime)
			w.logger.Info("State machine accepted job %s for processing", job.ID)
		}
	}
}

// handleKickoffFailure does simple retry behavior for kickoff errors (similar to UploadWorker.handleJobFailure).
func (w *StateMachineWorker) handleKickoffFailure(ctx context.Context, job *repositories.Job, jobErr error, startTime time.Time) {
	w.recordJobFailure(startTime)

	// Refresh job from repo
	freshJob, err := w.jobRepo.GetJob(ctx, job.ID)
	if err != nil {
		w.logger.Error("Failed to get job for retry handling: %v", err)
		return
	}

	freshJob.RetryCount++
	freshJob.Error = jobErr.Error()
	if err := w.jobRepo.UpdateJob(ctx, freshJob); err != nil {
		w.logger.Error("Failed to update job retry count: %v", err)
		return
	}

	if freshJob.RetryCount <= freshJob.MaxRetries {
		w.logger.Warn("Kickoff failed, will retry (%d/%d): %s - %v", freshJob.RetryCount, freshJob.MaxRetries, freshJob.ID, jobErr)
		freshJob.Status = repositories.JobStatusQueued
		freshJob.Message = fmt.Sprintf("Kickoff failed: %v. Retry %d/%d", jobErr, freshJob.RetryCount, freshJob.MaxRetries)
		time.Sleep(w.config.RetryDelay)
		if err := w.jobRepo.EnqueueJob(ctx, freshJob); err != nil {
			w.logger.Error("Failed to re-enqueue job: %v", err)
		}
	} else {
		w.logger.Error("Job kickoff failed permanently after %d retries: %s - %v", freshJob.MaxRetries, freshJob.ID, jobErr)
		freshJob.Status = repositories.JobStatusFailed
		freshJob.Message = fmt.Sprintf("Failed permanently after %d retries: %v", freshJob.MaxRetries, jobErr)
		if err := w.jobRepo.UpdateJob(ctx, freshJob); err != nil {
			w.logger.Error("Failed to update job to failed status: %v", err)
		}
	}
}
