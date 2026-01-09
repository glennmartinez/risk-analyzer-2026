package models

import (
	"time"
)

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

	// process Jobs
	JobTypeDocumentParce    JobType = "document_parse"
	JobTypeDocumentChunking JobType = "document_chunking"
	JobTypeDocumentEmbed    JobType = "document_embed"
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

// JobDTO represents the API view of a job
type JobDTO struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Priority    int                    `json:"priority"`
	Progress    int                    `json:"progress"`
	Message     string                 `json:"message"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
	CreatedAt   string                 `json:"created_at"`
	StartedAt   string                 `json:"started_at,omitempty"`
	CompletedAt string                 `json:"completed_at,omitempty"`
	UpdatedAt   string                 `json:"updated_at"`
	UserID      string                 `json:"user_id,omitempty"`
	WorkerID    string                 `json:"worker_id,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
}

// ToDTO converts Job domain model to DTO
func (j *Job) ToDTO() JobDTO {
	dto := JobDTO{
		ID:         j.ID,
		Type:       string(j.Type),
		Status:     string(j.Status),
		Priority:   j.Priority,
		Progress:   j.Progress,
		Message:    j.Message,
		Payload:    j.Payload,
		Result:     j.Result,
		Error:      j.Error,
		RetryCount: j.RetryCount,
		MaxRetries: j.MaxRetries,
		CreatedAt:  j.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  j.UpdatedAt.Format(time.RFC3339),
		UserID:     j.UserID,
		WorkerID:   j.WorkerID,
		Tags:       j.Tags,
	}

	if j.StartedAt != nil {
		dto.StartedAt = j.StartedAt.Format(time.RFC3339)
	}
	if j.CompletedAt != nil {
		dto.CompletedAt = j.CompletedAt.Format(time.RFC3339)
	}

	// Calculate duration
	duration := j.Duration()
	if duration > 0 {
		dto.Duration = duration.String()
	}

	return dto
}

// JobFromDTO converts JobDTO to Job domain model
func JobFromDTO(dto JobDTO) (*Job, error) {
	createdAt, err := time.Parse(time.RFC3339, dto.CreatedAt)
	if err != nil {
		createdAt = time.Now()
	}

	updatedAt, err := time.Parse(time.RFC3339, dto.UpdatedAt)
	if err != nil {
		updatedAt = time.Now()
	}

	job := &Job{
		ID:         dto.ID,
		Type:       JobType(dto.Type),
		Status:     JobStatus(dto.Status),
		Priority:   dto.Priority,
		Progress:   dto.Progress,
		Message:    dto.Message,
		Payload:    dto.Payload,
		Result:     dto.Result,
		Error:      dto.Error,
		RetryCount: dto.RetryCount,
		MaxRetries: dto.MaxRetries,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		UserID:     dto.UserID,
		WorkerID:   dto.WorkerID,
		Tags:       dto.Tags,
	}

	if dto.StartedAt != "" {
		if startedAt, err := time.Parse(time.RFC3339, dto.StartedAt); err == nil {
			job.StartedAt = &startedAt
		}
	}

	if dto.CompletedAt != "" {
		if completedAt, err := time.Parse(time.RFC3339, dto.CompletedAt); err == nil {
			job.CompletedAt = &completedAt
		}
	}

	return job, nil
}

// Validate checks if job is valid
func (j *Job) Validate() error {
	if j.ID == "" {
		return &ValidationError{Field: "id", Message: "job ID is required"}
	}
	if j.Type == "" {
		return &ValidationError{Field: "type", Message: "job type is required"}
	}
	if !j.Type.IsValid() {
		return &ValidationError{Field: "type", Message: "invalid job type: " + string(j.Type)}
	}
	if !j.Status.IsValid() {
		return &ValidationError{Field: "status", Message: "invalid job status: " + string(j.Status)}
	}
	if j.Progress < 0 || j.Progress > 100 {
		return &ValidationError{Field: "progress", Message: "progress must be between 0 and 100"}
	}
	if j.MaxRetries < 0 {
		return &ValidationError{Field: "max_retries", Message: "max retries cannot be negative"}
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

// JobProgress represents the progress of a job
type JobProgress struct {
	JobID     string    `json:"job_id"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	Status    JobStatus `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

// JobProgressDTO represents the API view of job progress
type JobProgressDTO struct {
	JobID     string `json:"job_id"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

// ToDTO converts JobProgress to DTO
func (jp *JobProgress) ToDTO() JobProgressDTO {
	return JobProgressDTO{
		JobID:     jp.JobID,
		Progress:  jp.Progress,
		Message:   jp.Message,
		Status:    string(jp.Status),
		UpdatedAt: jp.UpdatedAt.Format(time.RFC3339),
	}
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

// JobStatsDTO represents the API view of job statistics
type JobStatsDTO struct {
	TotalJobs     int            `json:"total_jobs"`
	JobsByStatus  map[string]int `json:"jobs_by_status"`
	JobsByType    map[string]int `json:"jobs_by_type"`
	AverageTime   string         `json:"average_time"`
	SuccessRate   float64        `json:"success_rate"`
	ActiveWorkers int            `json:"active_workers"`
}

// ToDTO converts JobStats to DTO
func (js *JobStats) ToDTO() JobStatsDTO {
	statusMap := make(map[string]int)
	for status, count := range js.JobsByStatus {
		statusMap[string(status)] = count
	}

	typeMap := make(map[string]int)
	for jobType, count := range js.JobsByType {
		typeMap[string(jobType)] = count
	}

	return JobStatsDTO{
		TotalJobs:     js.TotalJobs,
		JobsByStatus:  statusMap,
		JobsByType:    typeMap,
		AverageTime:   js.AverageTime.String(),
		SuccessRate:   js.SuccessRate,
		ActiveWorkers: js.ActiveWorkers,
	}
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

// Validate validates the upload job payload
func (p *UploadJobPayload) Validate() error {
	if p.Filename == "" {
		return &ValidationError{Field: "filename", Message: "filename is required"}
	}
	if p.Collection == "" {
		return &ValidationError{Field: "collection", Message: "collection is required"}
	}
	if p.ChunkSize <= 0 {
		p.ChunkSize = 512 // Default
	}
	if p.ChunkOverlap < 0 {
		return &ValidationError{Field: "chunk_overlap", Message: "chunk overlap cannot be negative"}
	}
	return nil
}

// UploadJobResult represents the result of a document upload job
type UploadJobResult struct {
	DocumentID       string  `json:"document_id"`
	ChunkCount       int     `json:"chunk_count"`
	ProcessingTimeMs float64 `json:"processing_time_ms"`
	Collection       string  `json:"collection"`
	Success          bool    `json:"success"`
}
