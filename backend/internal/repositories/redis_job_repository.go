package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes for jobs
	jobKeyPrefix       = "job:"
	jobIndexKey        = "jobs:index"
	jobQueuePrefix     = "job:queue:"
	jobTypeIndexPrefix = "job:type:"
	jobStatusPrefix    = "job:status:"
	jobUserPrefix      = "job:user:"
)

// RedisJobRepository implements JobRepository using Redis
type RedisJobRepository struct {
	client *redis.Client
}

// NewRedisJobRepository creates a new Redis-based job repository
func NewRedisJobRepository(client *redis.Client) *RedisJobRepository {
	return &RedisJobRepository{
		client: client,
	}
}

// CreateJob creates a new job in the repository
func (r *RedisJobRepository) CreateJob(ctx context.Context, job *Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	// Check if job already exists
	exists, err := r.jobExists(ctx, job.ID)
	if err != nil {
		return NewJobRepositoryError("create_job", job.ID, err, "")
	}
	if exists {
		return JobAlreadyExistsError(job.ID)
	}

	// Set timestamps
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now

	// Default status if not set
	if job.Status == "" {
		job.Status = JobStatusPending
	}

	// Use transaction for atomicity
	pipe := r.client.TxPipeline()

	// Serialize job to JSON
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return NewJobRepositoryError("create_job", job.ID, err, "failed to marshal job")
	}

	// Store job
	jobKey := jobKeyPrefix + job.ID
	pipe.Set(ctx, jobKey, jobJSON, 0)

	// Add to global index
	pipe.SAdd(ctx, jobIndexKey, job.ID)

	// Add to type index
	typeKey := jobTypeIndexPrefix + string(job.Type)
	pipe.SAdd(ctx, typeKey, job.ID)

	// Add to status index
	statusKey := jobStatusPrefix + string(job.Status)
	pipe.SAdd(ctx, statusKey, job.ID)

	// Add to user index if user ID is provided
	if job.UserID != "" {
		userKey := jobUserPrefix + job.UserID
		pipe.SAdd(ctx, userKey, job.ID)
	}

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return NewJobRepositoryError("create_job", job.ID, err, "failed to execute transaction")
	}

	return nil
}

// GetJob retrieves a job by ID
// UpdateJob updates an entire job in the repository
func (r *RedisJobRepository) UpdateJob(ctx context.Context, job *Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	// Check if job exists
	exists, err := r.jobExists(ctx, job.ID)
	if err != nil {
		return err
	}
	if !exists {
		return JobNotFoundError(job.ID)
	}

	// Update timestamp
	job.UpdatedAt = time.Now()

	// Serialize job
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return NewJobRepositoryError("update_job", job.ID, err, "failed to marshal job")
	}

	// Save to Redis
	jobKey := jobKeyPrefix + job.ID
	if err := r.client.Set(ctx, jobKey, jobJSON, 0).Err(); err != nil {
		return NewJobRepositoryError("update_job", job.ID, err, "failed to save job")
	}

	return nil
}

func (r *RedisJobRepository) GetJob(ctx context.Context, jobID string) (*Job, error) {
	jobKey := jobKeyPrefix + jobID

	jobJSON, err := r.client.Get(ctx, jobKey).Result()
	if err == redis.Nil {
		return nil, JobNotFoundError(jobID)
	}
	if err != nil {
		return nil, NewJobRepositoryError("get_job", jobID, err, "")
	}

	var job Job
	if err := json.Unmarshal([]byte(jobJSON), &job); err != nil {
		return nil, NewJobRepositoryError("get_job", jobID, err, "failed to unmarshal job")
	}

	return &job, nil
}

// UpdateJobStatus updates the status, progress, and message of a job
func (r *RedisJobRepository) UpdateJobStatus(ctx context.Context, jobID string, status JobStatus, progress int, message string) error {
	// Get existing job
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	oldStatus := job.Status

	// Update fields
	job.Status = status
	job.Progress = progress
	job.Message = message
	job.UpdatedAt = time.Now()

	// Update timestamps based on status
	now := time.Now()
	switch status {
	case JobStatusProcessing:
		if job.StartedAt == nil {
			job.StartedAt = &now
		}
	case JobStatusCompleted, JobStatusFailed, JobStatusCancelled:
		if job.CompletedAt == nil {
			job.CompletedAt = &now
		}
	}

	// Validate
	if err := job.Validate(); err != nil {
		return err
	}

	// Use transaction
	pipe := r.client.TxPipeline()

	// Serialize updated job
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return NewJobRepositoryError("update_job_status", jobID, err, "failed to marshal job")
	}

	// Update job
	jobKey := jobKeyPrefix + jobID
	pipe.Set(ctx, jobKey, jobJSON, 0)

	// Update status index if changed
	if oldStatus != status {
		oldStatusKey := jobStatusPrefix + string(oldStatus)
		newStatusKey := jobStatusPrefix + string(status)
		pipe.SRem(ctx, oldStatusKey, jobID)
		pipe.SAdd(ctx, newStatusKey, jobID)
	}

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return NewJobRepositoryError("update_job_status", jobID, err, "failed to execute transaction")
	}

	return nil
}

// UpdateJobResult updates the result field of a job
func (r *RedisJobRepository) UpdateJobResult(ctx context.Context, jobID string, result map[string]interface{}) error {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	job.Result = result
	job.UpdatedAt = time.Now()

	// Serialize and save
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return NewJobRepositoryError("update_job_result", jobID, err, "failed to marshal job")
	}

	jobKey := jobKeyPrefix + jobID
	err = r.client.Set(ctx, jobKey, jobJSON, 0).Err()
	if err != nil {
		return NewJobRepositoryError("update_job_result", jobID, err, "")
	}

	return nil
}

// DeleteJob removes a job from the repository
func (r *RedisJobRepository) DeleteJob(ctx context.Context, jobID string) error {
	// Get job to access metadata for index cleanup
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	// Use transaction
	pipe := r.client.TxPipeline()

	// Delete job
	jobKey := jobKeyPrefix + jobID
	pipe.Del(ctx, jobKey)

	// Remove from global index
	pipe.SRem(ctx, jobIndexKey, jobID)

	// Remove from type index
	typeKey := jobTypeIndexPrefix + string(job.Type)
	pipe.SRem(ctx, typeKey, jobID)

	// Remove from status index
	statusKey := jobStatusPrefix + string(job.Status)
	pipe.SRem(ctx, statusKey, jobID)

	// Remove from user index
	if job.UserID != "" {
		userKey := jobUserPrefix + job.UserID
		pipe.SRem(ctx, userKey, jobID)
	}

	// Remove from queue if queued
	if job.Status == JobStatusQueued {
		queueKey := jobQueuePrefix + string(job.Type)
		pipe.ZRem(ctx, queueKey, jobID)
	}

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return NewJobRepositoryError("delete_job", jobID, err, "failed to execute transaction")
	}

	return nil
}

// ListJobs retrieves jobs based on filter criteria
func (r *RedisJobRepository) ListJobs(ctx context.Context, filter *JobFilter) ([]*Job, error) {
	var jobIDs []string
	var err error

	if filter == nil {
		// No filter, get all jobs
		jobIDs, err = r.client.SMembers(ctx, jobIndexKey).Result()
		if err != nil {
			return nil, NewJobRepositoryError("list_jobs", "", err, "")
		}
	} else {
		// Apply filters
		if len(filter.Statuses) > 0 {
			// Get jobs by status
			for _, status := range filter.Statuses {
				statusKey := jobStatusPrefix + string(status)
				ids, err := r.client.SMembers(ctx, statusKey).Result()
				if err != nil {
					return nil, NewJobRepositoryError("list_jobs", "", err, "")
				}
				jobIDs = append(jobIDs, ids...)
			}
		} else if len(filter.Types) > 0 {
			// Get jobs by type
			for _, jobType := range filter.Types {
				typeKey := jobTypeIndexPrefix + string(jobType)
				ids, err := r.client.SMembers(ctx, typeKey).Result()
				if err != nil {
					return nil, NewJobRepositoryError("list_jobs", "", err, "")
				}
				jobIDs = append(jobIDs, ids...)
			}
		} else if filter.UserID != "" {
			// Get jobs by user
			userKey := jobUserPrefix + filter.UserID
			jobIDs, err = r.client.SMembers(ctx, userKey).Result()
			if err != nil {
				return nil, NewJobRepositoryError("list_jobs", "", err, "")
			}
		} else {
			// No specific filter, get all
			jobIDs, err = r.client.SMembers(ctx, jobIndexKey).Result()
			if err != nil {
				return nil, NewJobRepositoryError("list_jobs", "", err, "")
			}
		}
	}

	if len(jobIDs) == 0 {
		return []*Job{}, nil
	}

	// Get all jobs
	jobs, err := r.getBatch(ctx, jobIDs)
	if err != nil {
		return nil, err
	}

	// Apply additional filters in memory
	if filter != nil {
		jobs = r.applyFilters(jobs, filter)
	}

	// Apply pagination
	if filter != nil && filter.Limit > 0 {
		offset := filter.Offset
		limit := filter.Limit
		if offset >= len(jobs) {
			return []*Job{}, nil
		}
		end := offset + limit
		if end > len(jobs) {
			end = len(jobs)
		}
		jobs = jobs[offset:end]
	}

	return jobs, nil
}

// ListJobsByStatus retrieves all jobs with a specific status
func (r *RedisJobRepository) ListJobsByStatus(ctx context.Context, status JobStatus) ([]*Job, error) {
	statusKey := jobStatusPrefix + string(status)
	jobIDs, err := r.client.SMembers(ctx, statusKey).Result()
	if err != nil {
		return nil, NewJobRepositoryError("list_jobs_by_status", "", err, "")
	}

	if len(jobIDs) == 0 {
		return []*Job{}, nil
	}

	return r.getBatch(ctx, jobIDs)
}

// ListJobsByType retrieves all jobs with a specific type
func (r *RedisJobRepository) ListJobsByType(ctx context.Context, jobType JobType) ([]*Job, error) {
	typeKey := jobTypeIndexPrefix + string(jobType)
	jobIDs, err := r.client.SMembers(ctx, typeKey).Result()
	if err != nil {
		return nil, NewJobRepositoryError("list_jobs_by_type", "", err, "")
	}

	if len(jobIDs) == 0 {
		return []*Job{}, nil
	}

	return r.getBatch(ctx, jobIDs)
}

// GetActiveJobs retrieves all jobs that are currently active
func (r *RedisJobRepository) GetActiveJobs(ctx context.Context) ([]*Job, error) {
	activeStatuses := []JobStatus{JobStatusQueued, JobStatusProcessing, JobStatusRetrying}
	var allJobIDs []string

	for _, status := range activeStatuses {
		statusKey := jobStatusPrefix + string(status)
		jobIDs, err := r.client.SMembers(ctx, statusKey).Result()
		if err != nil {
			return nil, NewJobRepositoryError("get_active_jobs", "", err, "")
		}
		allJobIDs = append(allJobIDs, jobIDs...)
	}

	if len(allJobIDs) == 0 {
		return []*Job{}, nil
	}

	return r.getBatch(ctx, allJobIDs)
}

// EnqueueJob adds a job to the processing queue
func (r *RedisJobRepository) EnqueueJob(ctx context.Context, job *Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	// Update job status to queued
	if err := r.UpdateJobStatus(ctx, job.ID, JobStatusQueued, 0, "Job queued for processing"); err != nil {
		return err
	}

	// Add to priority queue (sorted set by priority)
	queueKey := jobQueuePrefix + string(job.Type)
	score := float64(job.Priority)
	if score == 0 {
		score = float64(time.Now().Unix()) // Use timestamp if no priority
	}

	err := r.client.ZAdd(ctx, queueKey, redis.Z{
		Score:  score,
		Member: job.ID,
	}).Err()

	if err != nil {
		return NewJobRepositoryError("enqueue_job", job.ID, err, "failed to add to queue")
	}

	return nil
}

// DequeueJob retrieves and removes the next job from the queue
func (r *RedisJobRepository) DequeueJob(ctx context.Context, jobType JobType) (*Job, error) {
	queueKey := jobQueuePrefix + string(jobType)

	// Get highest priority job (ZPOPMAX gets highest score)
	result, err := r.client.ZPopMax(ctx, queueKey, 1).Result()
	if err != nil {
		return nil, NewJobRepositoryError("dequeue_job", "", err, "")
	}

	if len(result) == 0 {
		return nil, nil // No jobs in queue
	}

	jobID, ok := result[0].Member.(string)
	if !ok {
		return nil, NewJobRepositoryError("dequeue_job", "", nil, "invalid job ID in queue")
	}

	// Get the job
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Update status to processing
	err = r.UpdateJobStatus(ctx, jobID, JobStatusProcessing, 0, "Processing started")
	if err != nil {
		return nil, err
	}

	return job, nil
}

// RequeueFailedJobs re-queues failed jobs that haven't exceeded max retries
func (r *RedisJobRepository) RequeueFailedJobs(ctx context.Context, maxRetries int) (int, error) {
	failedJobs, err := r.ListJobsByStatus(ctx, JobStatusFailed)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, job := range failedJobs {
		if job.RetryCount < maxRetries {
			job.RetryCount++
			job.Status = JobStatusRetrying
			job.UpdatedAt = time.Now()

			// Update job
			if err := r.UpdateJobStatus(ctx, job.ID, JobStatusRetrying, 0, fmt.Sprintf("Retry attempt %d/%d", job.RetryCount, maxRetries)); err != nil {
				continue
			}

			// Re-enqueue
			if err := r.EnqueueJob(ctx, job); err != nil {
				continue
			}

			count++
		}
	}

	return count, nil
}

// SetProgress updates the progress of a job
func (r *RedisJobRepository) SetProgress(ctx context.Context, jobID string, progress int, message string) error {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	job.Progress = progress
	job.Message = message
	job.UpdatedAt = time.Now()

	// Validate progress
	if progress < 0 || progress > 100 {
		return InvalidJobError(jobID, "progress must be between 0 and 100")
	}

	// Serialize and save
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return NewJobRepositoryError("set_progress", jobID, err, "failed to marshal job")
	}

	jobKey := jobKeyPrefix + jobID
	err = r.client.Set(ctx, jobKey, jobJSON, 0).Err()
	if err != nil {
		return NewJobRepositoryError("set_progress", jobID, err, "")
	}

	return nil
}

// GetProgress retrieves the progress of a job
func (r *RedisJobRepository) GetProgress(ctx context.Context, jobID string) (*JobProgress, error) {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	return &JobProgress{
		JobID:     job.ID,
		Progress:  job.Progress,
		Message:   job.Message,
		Status:    job.Status,
		UpdatedAt: job.UpdatedAt,
	}, nil
}

// CleanupCompletedJobs removes completed jobs older than the specified duration
func (r *RedisJobRepository) CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) (int, error) {
	completedJobs, err := r.ListJobsByStatus(ctx, JobStatusCompleted)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	count := 0

	for _, job := range completedJobs {
		if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
			if err := r.DeleteJob(ctx, job.ID); err != nil {
				continue
			}
			count++
		}
	}

	return count, nil
}

// CleanupFailedJobs removes failed jobs older than the specified duration
func (r *RedisJobRepository) CleanupFailedJobs(ctx context.Context, olderThan time.Duration, maxRetries int) (int, error) {
	failedJobs, err := r.ListJobsByStatus(ctx, JobStatusFailed)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	count := 0

	for _, job := range failedJobs {
		// Only cleanup if exceeded max retries and old enough
		if job.RetryCount >= maxRetries && job.UpdatedAt.Before(cutoff) {
			if err := r.DeleteJob(ctx, job.ID); err != nil {
				continue
			}
			count++
		}
	}

	return count, nil
}

// Ping checks if Redis connection is alive
func (r *RedisJobRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisJobRepository) Close() error {
	return r.client.Close()
}

// Helper methods

// jobExists checks if a job exists
func (r *RedisJobRepository) jobExists(ctx context.Context, jobID string) (bool, error) {
	jobKey := jobKeyPrefix + jobID
	exists, err := r.client.Exists(ctx, jobKey).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// getBatch retrieves multiple jobs by IDs
func (r *RedisJobRepository) getBatch(ctx context.Context, jobIDs []string) ([]*Job, error) {
	if len(jobIDs) == 0 {
		return []*Job{}, nil
	}

	// Build keys
	keys := make([]string, len(jobIDs))
	for i, id := range jobIDs {
		keys[i] = jobKeyPrefix + id
	}

	// Use pipeline for batch get
	pipe := r.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, NewJobRepositoryError("get_batch", "", err, "failed to execute batch get")
	}

	// Parse results
	jobs := make([]*Job, 0, len(jobIDs))
	for i, cmd := range cmds {
		jobJSON, err := cmd.Result()
		if err == redis.Nil {
			// Skip missing jobs
			continue
		}
		if err != nil {
			return nil, NewJobRepositoryError("get_batch", jobIDs[i], err, "")
		}

		var job Job
		if err := json.Unmarshal([]byte(jobJSON), &job); err != nil {
			return nil, NewJobRepositoryError("get_batch", jobIDs[i], err, "failed to unmarshal job")
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// applyFilters applies additional filters to jobs
func (r *RedisJobRepository) applyFilters(jobs []*Job, filter *JobFilter) []*Job {
	filtered := make([]*Job, 0, len(jobs))

	for _, job := range jobs {
		// Check types
		if len(filter.Types) > 0 {
			matched := false
			for _, t := range filter.Types {
				if job.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Check statuses
		if len(filter.Statuses) > 0 {
			matched := false
			for _, s := range filter.Statuses {
				if job.Status == s {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Check user ID
		if filter.UserID != "" && job.UserID != filter.UserID {
			continue
		}

		// Check tags
		if len(filter.Tags) > 0 {
			matched := false
			for _, filterTag := range filter.Tags {
				for _, jobTag := range job.Tags {
					if filterTag == jobTag {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Check created after
		if filter.CreatedAfter != nil && job.CreatedAt.Before(*filter.CreatedAfter) {
			continue
		}

		// Check created before
		if filter.CreatedBefore != nil && job.CreatedAt.After(*filter.CreatedBefore) {
			continue
		}

		filtered = append(filtered, job)
	}

	return filtered
}

// GetStats returns statistics about jobs (helper method)
func (r *RedisJobRepository) GetStats(ctx context.Context) (*JobStats, error) {
	allJobs, err := r.ListJobs(ctx, nil)
	if err != nil {
		return nil, err
	}

	stats := &JobStats{
		TotalJobs:    len(allJobs),
		JobsByStatus: make(map[JobStatus]int),
		JobsByType:   make(map[JobType]int),
	}

	var totalDuration time.Duration
	successCount := 0
	activeWorkers := make(map[string]bool)

	for _, job := range allJobs {
		stats.JobsByStatus[job.Status]++
		stats.JobsByType[job.Type]++

		if job.Status == JobStatusCompleted {
			successCount++
			totalDuration += job.Duration()
		}

		if job.Status == JobStatusProcessing && job.WorkerID != "" {
			activeWorkers[job.WorkerID] = true
		}
	}

	if successCount > 0 {
		stats.AverageTime = totalDuration / time.Duration(successCount)
	}

	if len(allJobs) > 0 {
		stats.SuccessRate = float64(successCount) / float64(len(allJobs))
	}

	stats.ActiveWorkers = len(activeWorkers)

	return stats, nil
}

// GetQueueLength returns the number of jobs in a specific queue
func (r *RedisJobRepository) GetQueueLength(ctx context.Context, jobType JobType) (int64, error) {
	queueKey := jobQueuePrefix + string(jobType)
	return r.client.ZCard(ctx, queueKey).Result()
}
