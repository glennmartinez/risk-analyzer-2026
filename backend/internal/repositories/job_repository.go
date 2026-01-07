package repositories

import (
	"context"
	"time"
)

// JobRepository defines the interface for job queue operations
// This manages background job processing for long-running tasks like document upload
type JobRepository interface {
	// Job Management
	CreateJob(ctx context.Context, job *Job) error
	GetJob(ctx context.Context, jobID string) (*Job, error)
	UpdateJob(ctx context.Context, job *Job) error
	UpdateJobStatus(ctx context.Context, jobID string, status JobStatus, progress int, message string) error
	UpdateJobResult(ctx context.Context, jobID string, result map[string]interface{}) error
	DeleteJob(ctx context.Context, jobID string) error

	// Job Queries
	ListJobs(ctx context.Context, filter *JobFilter) ([]*Job, error)
	ListJobsByStatus(ctx context.Context, status JobStatus) ([]*Job, error)
	ListJobsByType(ctx context.Context, jobType JobType) ([]*Job, error)
	GetActiveJobs(ctx context.Context) ([]*Job, error)

	// Job Queue Operations
	EnqueueJob(ctx context.Context, job *Job) error
	DequeueJob(ctx context.Context, jobType JobType) (*Job, error)
	RequeueFailedJobs(ctx context.Context, maxRetries int) (int, error)

	// Job Progress
	SetProgress(ctx context.Context, jobID string, progress int, message string) error
	GetProgress(ctx context.Context, jobID string) (*JobProgress, error)

	// Cleanup
	CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) (int, error)
	CleanupFailedJobs(ctx context.Context, olderThan time.Duration, maxRetries int) (int, error)

	// Health
	Ping(ctx context.Context) error
	Close() error
}

// Job represents a background job in the queue
type Job struct {
	ID          string                 `json:"id"`
	Type        JobType                `json:"type"`
	Status      JobStatus              `json:"status"`
	Priority    int                    `json:"priority"` // Higher = more important
	Progress    int                    `json:"progress"` // 0-100
	Message     string                 `json:"message"`
	Payload     map[string]interface{} `json:"payload"` // Input data
	Result      map[string]interface{} `json:"result"`  // Output data
	Error       string                 `json:"error,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`

	// Metadata
	UserID   string   `json:"user_id,omitempty"`
	WorkerID string   `json:"worker_id,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// JobType represents the type of job
type JobType string

const (
	JobTypeDocumentUpload   JobType = "document_upload"
	JobTypeDocumentDelete   JobType = "document_delete"
	JobTypeCollectionDelete JobType = "collection_delete"
	JobTypeVectorReindex    JobType = "vector_reindex"
	JobTypeMetadataExtract  JobType = "metadata_extract"
	JobTypeBulkImport       JobType = "bulk_import"
)

// JobStatus represents the current status of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusQueued     JobStatus = "queued"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
	JobStatusRetrying   JobStatus = "retrying"
)

// JobFilter represents filter criteria for job queries
type JobFilter struct {
	Types         []JobType
	Statuses      []JobStatus
	UserID        string
	Tags          []string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Limit         int
	Offset        int
}

// JobProgress represents the progress of a job
type JobProgress struct {
	JobID     string    `json:"job_id"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	Status    JobStatus `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

// JobStats represents statistics about jobs
type JobStats struct {
	TotalJobs     int               `json:"total_jobs"`
	JobsByStatus  map[JobStatus]int `json:"jobs_by_status"`
	JobsByType    map[JobType]int   `json:"jobs_by_type"`
	AverageTime   time.Duration     `json:"average_time"`
	SuccessRate   float64           `json:"success_rate"`
	ActiveWorkers int               `json:"active_workers"`
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

// UploadJobResult represents the result of a document upload job
type UploadJobResult struct {
	DocumentID       string  `json:"document_id"`
	ChunkCount       int     `json:"chunk_count"`
	ProcessingTimeMs float64 `json:"processing_time_ms"`
	Collection       string  `json:"collection"`
	Success          bool    `json:"success"`
}

// JobRepositoryError represents errors from the job repository
type JobRepositoryError struct {
	Operation string
	JobID     string
	Err       error
	Message   string
}

func (e *JobRepositoryError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	prefix := e.Operation
	if e.JobID != "" {
		prefix += " (job: " + e.JobID + ")"
	}
	if e.Err != nil {
		return prefix + ": " + e.Err.Error()
	}
	return prefix + ": unknown error"
}

func (e *JobRepositoryError) Unwrap() error {
	return e.Err
}

// NewJobRepositoryError creates a new job repository error
func NewJobRepositoryError(operation string, jobID string, err error, message string) *JobRepositoryError {
	return &JobRepositoryError{
		Operation: operation,
		JobID:     jobID,
		Err:       err,
		Message:   message,
	}
}

// Common error constructors
func JobNotFoundError(jobID string) error {
	return NewJobRepositoryError(
		"get_job",
		jobID,
		nil,
		"job not found: "+jobID,
	)
}

func JobAlreadyExistsError(jobID string) error {
	return NewJobRepositoryError(
		"create_job",
		jobID,
		nil,
		"job already exists: "+jobID,
	)
}

func InvalidJobError(jobID string, reason string) error {
	return NewJobRepositoryError(
		"validate_job",
		jobID,
		nil,
		"invalid job: "+reason,
	)
}

// Validation helpers

// Validate checks if job is valid
func (j *Job) Validate() error {
	if j.ID == "" {
		return InvalidJobError("", "job ID is required")
	}
	if j.Type == "" {
		return InvalidJobError(j.ID, "job type is required")
	}
	if !j.Type.IsValid() {
		return InvalidJobError(j.ID, "invalid job type: "+string(j.Type))
	}
	if !j.Status.IsValid() {
		return InvalidJobError(j.ID, "invalid job status: "+string(j.Status))
	}
	if j.Progress < 0 || j.Progress > 100 {
		return InvalidJobError(j.ID, "progress must be between 0 and 100")
	}
	if j.MaxRetries < 0 {
		return InvalidJobError(j.ID, "max retries cannot be negative")
	}
	return nil
}

// IsValid checks if job type is valid
func (t JobType) IsValid() bool {
	switch t {
	case JobTypeDocumentUpload, JobTypeDocumentDelete, JobTypeCollectionDelete,
		JobTypeVectorReindex, JobTypeMetadataExtract, JobTypeBulkImport:
		return true
	default:
		return false
	}
}

// String returns the string representation of job type
func (t JobType) String() string {
	return string(t)
}

// IsValid checks if job status is valid
func (s JobStatus) IsValid() bool {
	switch s {
	case JobStatusPending, JobStatusQueued, JobStatusProcessing,
		JobStatusCompleted, JobStatusFailed, JobStatusCancelled, JobStatusRetrying:
		return true
	default:
		return false
	}
}

// String returns the string representation of job status
func (s JobStatus) String() string {
	return string(s)
}

// IsTerminal returns true if the status is a terminal state
func (s JobStatus) IsTerminal() bool {
	return s == JobStatusCompleted || s == JobStatusFailed || s == JobStatusCancelled
}

// IsActive returns true if the job is currently active
func (s JobStatus) IsActive() bool {
	return s == JobStatusQueued || s == JobStatusProcessing || s == JobStatusRetrying
}

// CanRetry returns true if the job can be retried
func (j *Job) CanRetry() bool {
	return j.Status == JobStatusFailed && j.RetryCount < j.MaxRetries
}

// IsComplete returns true if the job is in a terminal state
func (j *Job) IsComplete() bool {
	return j.Status.IsTerminal()
}

// Duration returns the time taken to complete the job
func (j *Job) Duration() time.Duration {
	if j.StartedAt == nil {
		return 0
	}
	if j.CompletedAt == nil {
		return time.Since(*j.StartedAt)
	}
	return j.CompletedAt.Sub(*j.StartedAt)
}
