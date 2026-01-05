package workers

import (
	"context"
	"sync"
	"time"
)

// Worker defines the interface for background workers
type Worker interface {
	// Start begins processing jobs
	Start(ctx context.Context) error

	// Stop gracefully shuts down the worker
	Stop(ctx context.Context) error

	// Name returns the worker's name
	Name() string

	// IsRunning returns whether the worker is currently running
	IsRunning() bool

	// Stats returns worker statistics
	Stats() WorkerStats
}

// WorkerStats represents statistics about a worker
type WorkerStats struct {
	WorkerName         string        `json:"worker_name"`
	JobsProcessed      int64         `json:"jobs_processed"`
	JobsSucceeded      int64         `json:"jobs_succeeded"`
	JobsFailed         int64         `json:"jobs_failed"`
	AverageProcessTime time.Duration `json:"average_process_time"`
	LastJobTime        time.Time     `json:"last_job_time,omitempty"`
	Uptime             time.Duration `json:"uptime"`
	IsRunning          bool          `json:"is_running"`
}

// WorkerConfig holds configuration for workers
type WorkerConfig struct {
	// WorkerName is a unique identifier for this worker instance
	WorkerName string

	// Concurrency is the number of jobs to process concurrently
	Concurrency int

	// PollInterval is how often to check for new jobs
	PollInterval time.Duration

	// ShutdownTimeout is how long to wait for graceful shutdown
	ShutdownTimeout time.Duration

	// MaxRetries is the maximum number of retries for failed jobs
	MaxRetries int

	// RetryDelay is the delay between retries
	RetryDelay time.Duration

	// EnableRecovery enables panic recovery
	EnableRecovery bool
}

// DefaultWorkerConfig returns a worker configuration with sensible defaults
func DefaultWorkerConfig(workerName string) WorkerConfig {
	return WorkerConfig{
		WorkerName:      workerName,
		Concurrency:     3,
		PollInterval:    2 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		MaxRetries:      3,
		RetryDelay:      5 * time.Second,
		EnableRecovery:  true,
	}
}

// BaseWorker provides common functionality for workers
type BaseWorker struct {
	config  WorkerConfig
	running bool
	mu      sync.RWMutex

	// Stats tracking
	jobsProcessed    int64
	jobsSucceeded    int64
	jobsFailed       int64
	totalProcessTime time.Duration
	startTime        time.Time
	lastJobTime      time.Time
	statsMu          sync.RWMutex
}

// NewBaseWorker creates a new base worker
func NewBaseWorker(config WorkerConfig) *BaseWorker {
	return &BaseWorker{
		config: config,
	}
}

// Name returns the worker's name
func (w *BaseWorker) Name() string {
	return w.config.WorkerName
}

// IsRunning returns whether the worker is currently running
func (w *BaseWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// setRunning sets the running state
func (w *BaseWorker) setRunning(running bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.running = running
	if running {
		w.startTime = time.Now()
	}
}

// Stats returns worker statistics
func (w *BaseWorker) Stats() WorkerStats {
	w.statsMu.RLock()
	defer w.statsMu.RUnlock()

	var avgProcessTime time.Duration
	if w.jobsProcessed > 0 {
		avgProcessTime = w.totalProcessTime / time.Duration(w.jobsProcessed)
	}

	var uptime time.Duration
	if !w.startTime.IsZero() {
		uptime = time.Since(w.startTime)
	}

	return WorkerStats{
		WorkerName:         w.config.WorkerName,
		JobsProcessed:      w.jobsProcessed,
		JobsSucceeded:      w.jobsSucceeded,
		JobsFailed:         w.jobsFailed,
		AverageProcessTime: avgProcessTime,
		LastJobTime:        w.lastJobTime,
		Uptime:             uptime,
		IsRunning:          w.IsRunning(),
	}
}

// recordJobStart records the start of job processing
func (w *BaseWorker) recordJobStart() time.Time {
	return time.Now()
}

// recordJobSuccess records a successful job completion
func (w *BaseWorker) recordJobSuccess(startTime time.Time) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()

	duration := time.Since(startTime)
	w.jobsProcessed++
	w.jobsSucceeded++
	w.totalProcessTime += duration
	w.lastJobTime = time.Now()
}

// recordJobFailure records a failed job
func (w *BaseWorker) recordJobFailure(startTime time.Time) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()

	duration := time.Since(startTime)
	w.jobsProcessed++
	w.jobsFailed++
	w.totalProcessTime += duration
	w.lastJobTime = time.Now()
}

// resetStats resets worker statistics
func (w *BaseWorker) resetStats() {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()

	w.jobsProcessed = 0
	w.jobsSucceeded = 0
	w.jobsFailed = 0
	w.totalProcessTime = 0
	w.lastJobTime = time.Time{}
}

// Config returns the worker configuration
func (w *BaseWorker) Config() WorkerConfig {
	return w.config
}

// WorkerPool manages multiple workers
type WorkerPool struct {
	workers []Worker
	mu      sync.RWMutex
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool() *WorkerPool {
	return &WorkerPool{
		workers: make([]Worker, 0),
	}
}

// AddWorker adds a worker to the pool
func (p *WorkerPool) AddWorker(worker Worker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.workers = append(p.workers, worker)
}

// StartAll starts all workers in the pool
func (p *WorkerPool) StartAll(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, worker := range p.workers {
		if err := worker.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all workers in the pool
func (p *WorkerPool) StopAll(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(p.workers))

	for _, worker := range p.workers {
		wg.Add(1)
		go func(w Worker) {
			defer wg.Done()
			if err := w.Stop(ctx); err != nil {
				errChan <- err
			}
		}(worker)
	}

	wg.Wait()
	close(errChan)

	// Return first error if any
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// GetWorker returns a worker by name
func (p *WorkerPool) GetWorker(name string) Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, worker := range p.workers {
		if worker.Name() == name {
			return worker
		}
	}
	return nil
}

// ListWorkers returns all workers
func (p *WorkerPool) ListWorkers() []Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := make([]Worker, len(p.workers))
	copy(workers, p.workers)
	return workers
}

// GetAllStats returns statistics for all workers
func (p *WorkerPool) GetAllStats() []WorkerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make([]WorkerStats, 0, len(p.workers))
	for _, worker := range p.workers {
		stats = append(stats, worker.Stats())
	}
	return stats
}

// Count returns the number of workers in the pool
func (p *WorkerPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// JobProcessor defines a function that processes a job
type JobProcessor func(ctx context.Context, job interface{}) error

// RecoverableJobProcessor wraps a job processor with panic recovery
func RecoverableJobProcessor(processor JobProcessor) JobProcessor {
	return func(ctx context.Context, job interface{}) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = &WorkerPanicError{
					Panic: r,
				}
			}
		}()
		return processor(ctx, job)
	}
}

// WorkerError represents a worker-specific error
type WorkerError struct {
	WorkerName string
	Operation  string
	Err        error
	Message    string
}

func (e *WorkerError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	prefix := e.WorkerName + ":" + e.Operation
	if e.Err != nil {
		return prefix + ": " + e.Err.Error()
	}
	return prefix + ": unknown error"
}

func (e *WorkerError) Unwrap() error {
	return e.Err
}

// NewWorkerError creates a new worker error
func NewWorkerError(workerName, operation string, err error, message string) *WorkerError {
	return &WorkerError{
		WorkerName: workerName,
		Operation:  operation,
		Err:        err,
		Message:    message,
	}
}

// WorkerPanicError represents a panic that occurred during job processing
type WorkerPanicError struct {
	Panic interface{}
}

func (e *WorkerPanicError) Error() string {
	return "worker panic: " + formatPanic(e.Panic)
}

func formatPanic(p interface{}) string {
	switch v := p.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		return "unknown panic"
	}
}
