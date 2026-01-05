package repositories

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisJobRepository(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	repo := NewRedisJobRepository(client)
	assert.NotNil(t, repo)
	assert.Equal(t, client, repo.client)
}

func TestRedisJobRepository_CreateJob(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	t.Run("successful job creation", func(t *testing.T) {
		job := &Job{
			ID:         "job-1",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusPending,
			Priority:   1,
			Progress:   0,
			Message:    "Job created",
			MaxRetries: 3,
			Payload:    map[string]interface{}{"file": "test.pdf"},
		}

		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		// Verify job was stored
		retrieved, err := repo.GetJob(ctx, "job-1")
		require.NoError(t, err)
		assert.Equal(t, job.ID, retrieved.ID)
		assert.Equal(t, job.Type, retrieved.Type)
		assert.Equal(t, job.Status, retrieved.Status)
		assert.NotZero(t, retrieved.CreatedAt)
		assert.NotZero(t, retrieved.UpdatedAt)
	})

	t.Run("duplicate job creation fails", func(t *testing.T) {
		job := &Job{
			ID:         "job-dup",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusPending,
			MaxRetries: 3,
		}

		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		// Try to create again
		err = repo.CreateJob(ctx, job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("invalid job fails validation", func(t *testing.T) {
		job := &Job{
			ID:         "", // Invalid: empty ID
			Type:       JobTypeDocumentUpload,
			MaxRetries: 3,
		}

		err := repo.CreateJob(ctx, job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})
}

func TestRedisJobRepository_GetJob(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	t.Run("get existing job", func(t *testing.T) {
		job := &Job{
			ID:         "job-get-1",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusPending,
			Progress:   50,
			Message:    "Processing",
			MaxRetries: 3,
		}

		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		retrieved, err := repo.GetJob(ctx, "job-get-1")
		require.NoError(t, err)
		assert.Equal(t, job.ID, retrieved.ID)
		assert.Equal(t, job.Type, retrieved.Type)
		assert.Equal(t, job.Progress, retrieved.Progress)
	})

	t.Run("get non-existent job", func(t *testing.T) {
		_, err := repo.GetJob(ctx, "non-existent-job")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRedisJobRepository_UpdateJobStatus(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	t.Run("update job status", func(t *testing.T) {
		job := &Job{
			ID:         "job-update-1",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusPending,
			MaxRetries: 3,
		}

		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		// Update to processing
		err = repo.UpdateJobStatus(ctx, "job-update-1", JobStatusProcessing, 25, "Processing started")
		require.NoError(t, err)

		// Verify update
		updated, err := repo.GetJob(ctx, "job-update-1")
		require.NoError(t, err)
		assert.Equal(t, JobStatusProcessing, updated.Status)
		assert.Equal(t, 25, updated.Progress)
		assert.Equal(t, "Processing started", updated.Message)
		assert.NotNil(t, updated.StartedAt)
	})

	t.Run("update to completed sets timestamp", func(t *testing.T) {
		job := &Job{
			ID:         "job-update-2",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusProcessing,
			MaxRetries: 3,
		}

		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		// Update to completed
		err = repo.UpdateJobStatus(ctx, "job-update-2", JobStatusCompleted, 100, "Done")
		require.NoError(t, err)

		// Verify completed timestamp is set
		updated, err := repo.GetJob(ctx, "job-update-2")
		require.NoError(t, err)
		assert.Equal(t, JobStatusCompleted, updated.Status)
		assert.NotNil(t, updated.CompletedAt)
	})
}

func TestRedisJobRepository_UpdateJobResult(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	job := &Job{
		ID:         "job-result-1",
		Type:       JobTypeDocumentUpload,
		Status:     JobStatusCompleted,
		MaxRetries: 3,
	}

	err := repo.CreateJob(ctx, job)
	require.NoError(t, err)

	// Update result
	result := map[string]interface{}{
		"document_id": "doc-123",
		"chunks":      10,
		"success":     true,
	}

	err = repo.UpdateJobResult(ctx, "job-result-1", result)
	require.NoError(t, err)

	// Verify result
	updated, err := repo.GetJob(ctx, "job-result-1")
	require.NoError(t, err)
	assert.Equal(t, "doc-123", updated.Result["document_id"])
	assert.Equal(t, float64(10), updated.Result["chunks"])
	assert.True(t, updated.Result["success"].(bool))
}

func TestRedisJobRepository_DeleteJob(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	t.Run("delete existing job", func(t *testing.T) {
		job := &Job{
			ID:         "job-delete-1",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusCompleted,
			MaxRetries: 3,
		}

		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		// Delete
		err = repo.DeleteJob(ctx, "job-delete-1")
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetJob(ctx, "job-delete-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("delete non-existent job", func(t *testing.T) {
		err := repo.DeleteJob(ctx, "non-existent-job")
		assert.Error(t, err)
	})
}

func TestRedisJobRepository_ListJobsByStatus(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create jobs with different statuses
	jobs := []*Job{
		{ID: "job-status-1", Type: JobTypeDocumentUpload, Status: JobStatusPending, MaxRetries: 3},
		{ID: "job-status-2", Type: JobTypeDocumentUpload, Status: JobStatusPending, MaxRetries: 3},
		{ID: "job-status-3", Type: JobTypeDocumentUpload, Status: JobStatusCompleted, MaxRetries: 3},
	}

	for _, job := range jobs {
		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)
	}

	// List pending jobs
	pending, err := repo.ListJobsByStatus(ctx, JobStatusPending)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(pending), 2)

	// List completed jobs
	completed, err := repo.ListJobsByStatus(ctx, JobStatusCompleted)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(completed), 1)
}

func TestRedisJobRepository_ListJobsByType(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create jobs with different types
	jobs := []*Job{
		{ID: "job-type-1", Type: JobTypeDocumentUpload, Status: JobStatusPending, MaxRetries: 3},
		{ID: "job-type-2", Type: JobTypeDocumentUpload, Status: JobStatusPending, MaxRetries: 3},
		{ID: "job-type-3", Type: JobTypeDocumentDelete, Status: JobStatusPending, MaxRetries: 3},
	}

	for _, job := range jobs {
		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)
	}

	// List upload jobs
	uploadJobs, err := repo.ListJobsByType(ctx, JobTypeDocumentUpload)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(uploadJobs), 2)

	// List delete jobs
	deleteJobs, err := repo.ListJobsByType(ctx, JobTypeDocumentDelete)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(deleteJobs), 1)
}

func TestRedisJobRepository_GetActiveJobs(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create jobs with different statuses
	jobs := []*Job{
		{ID: "job-active-1", Type: JobTypeDocumentUpload, Status: JobStatusQueued, MaxRetries: 3},
		{ID: "job-active-2", Type: JobTypeDocumentUpload, Status: JobStatusProcessing, MaxRetries: 3},
		{ID: "job-active-3", Type: JobTypeDocumentUpload, Status: JobStatusCompleted, MaxRetries: 3},
		{ID: "job-active-4", Type: JobTypeDocumentUpload, Status: JobStatusFailed, MaxRetries: 3},
	}

	for _, job := range jobs {
		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)
	}

	// Get active jobs
	active, err := repo.GetActiveJobs(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(active), 2)

	// Verify only active statuses
	for _, job := range active {
		assert.True(t, job.Status.IsActive())
	}
}

func TestRedisJobRepository_EnqueueDequeue(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	t.Run("enqueue and dequeue job", func(t *testing.T) {
		job := &Job{
			ID:         "job-queue-1",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusPending,
			Priority:   10,
			MaxRetries: 3,
		}

		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		// Enqueue
		err = repo.EnqueueJob(ctx, job)
		require.NoError(t, err)

		// Verify status changed to queued
		queued, err := repo.GetJob(ctx, "job-queue-1")
		require.NoError(t, err)
		assert.Equal(t, JobStatusQueued, queued.Status)

		// Dequeue
		dequeued, err := repo.DequeueJob(ctx, JobTypeDocumentUpload)
		require.NoError(t, err)
		require.NotNil(t, dequeued)
		assert.Equal(t, "job-queue-1", dequeued.ID)
		assert.Equal(t, JobStatusProcessing, dequeued.Status)
	})

	t.Run("dequeue empty queue returns nil", func(t *testing.T) {
		job, err := repo.DequeueJob(ctx, JobTypeVectorReindex)
		require.NoError(t, err)
		assert.Nil(t, job)
	})

	t.Run("priority ordering", func(t *testing.T) {
		// Create jobs with different priorities
		jobs := []*Job{
			{ID: "job-prio-1", Type: JobTypeDocumentUpload, Status: JobStatusPending, Priority: 1, MaxRetries: 3},
			{ID: "job-prio-2", Type: JobTypeDocumentUpload, Status: JobStatusPending, Priority: 10, MaxRetries: 3},
			{ID: "job-prio-3", Type: JobTypeDocumentUpload, Status: JobStatusPending, Priority: 5, MaxRetries: 3},
		}

		for _, job := range jobs {
			err := repo.CreateJob(ctx, job)
			require.NoError(t, err)
			err = repo.EnqueueJob(ctx, job)
			require.NoError(t, err)
		}

		// Dequeue should return highest priority first (10)
		first, err := repo.DequeueJob(ctx, JobTypeDocumentUpload)
		require.NoError(t, err)
		assert.Equal(t, "job-prio-2", first.ID)

		// Next should be 5
		second, err := repo.DequeueJob(ctx, JobTypeDocumentUpload)
		require.NoError(t, err)
		assert.Equal(t, "job-prio-3", second.ID)

		// Last should be 1
		third, err := repo.DequeueJob(ctx, JobTypeDocumentUpload)
		require.NoError(t, err)
		assert.Equal(t, "job-prio-1", third.ID)
	})
}

func TestRedisJobRepository_SetGetProgress(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	job := &Job{
		ID:         "job-progress-1",
		Type:       JobTypeDocumentUpload,
		Status:     JobStatusProcessing,
		MaxRetries: 3,
	}

	err := repo.CreateJob(ctx, job)
	require.NoError(t, err)

	// Set progress
	err = repo.SetProgress(ctx, "job-progress-1", 75, "Almost done")
	require.NoError(t, err)

	// Get progress
	progress, err := repo.GetProgress(ctx, "job-progress-1")
	require.NoError(t, err)
	assert.Equal(t, 75, progress.Progress)
	assert.Equal(t, "Almost done", progress.Message)
	assert.Equal(t, JobStatusProcessing, progress.Status)
}

func TestRedisJobRepository_RequeueFailedJobs(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create failed jobs
	jobs := []*Job{
		{ID: "job-retry-1", Type: JobTypeDocumentUpload, Status: JobStatusFailed, RetryCount: 0, MaxRetries: 3},
		{ID: "job-retry-2", Type: JobTypeDocumentUpload, Status: JobStatusFailed, RetryCount: 2, MaxRetries: 3},
		{ID: "job-retry-3", Type: JobTypeDocumentUpload, Status: JobStatusFailed, RetryCount: 3, MaxRetries: 3}, // Exceeded retries
	}

	for _, job := range jobs {
		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)
	}

	// Requeue failed jobs
	count, err := repo.RequeueFailedJobs(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // Only 2 jobs should be requeued

	// Verify retry count incremented
	job1, err := repo.GetJob(ctx, "job-retry-1")
	require.NoError(t, err)
	assert.Equal(t, 1, job1.RetryCount)
	assert.Equal(t, JobStatusQueued, job1.Status)

	job2, err := repo.GetJob(ctx, "job-retry-2")
	require.NoError(t, err)
	assert.Equal(t, 3, job2.RetryCount)

	// Job 3 should still be failed
	job3, err := repo.GetJob(ctx, "job-retry-3")
	require.NoError(t, err)
	assert.Equal(t, JobStatusFailed, job3.Status)
}

func TestRedisJobRepository_CleanupCompletedJobs(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create old completed job
	oldTime := time.Now().Add(-48 * time.Hour)
	oldJob := &Job{
		ID:          "job-cleanup-old",
		Type:        JobTypeDocumentUpload,
		Status:      JobStatusCompleted,
		MaxRetries:  3,
		CompletedAt: &oldTime,
	}

	err := repo.CreateJob(ctx, oldJob)
	require.NoError(t, err)

	// Create recent completed job
	recentTime := time.Now().Add(-1 * time.Hour)
	recentJob := &Job{
		ID:          "job-cleanup-recent",
		Type:        JobTypeDocumentUpload,
		Status:      JobStatusCompleted,
		MaxRetries:  3,
		CompletedAt: &recentTime,
	}

	err = repo.CreateJob(ctx, recentJob)
	require.NoError(t, err)

	// Cleanup jobs older than 24 hours
	count, err := repo.CleanupCompletedJobs(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify old job is gone
	_, err = repo.GetJob(ctx, "job-cleanup-old")
	assert.Error(t, err)

	// Verify recent job still exists
	_, err = repo.GetJob(ctx, "job-cleanup-recent")
	assert.NoError(t, err)
}

func TestRedisJobRepository_CleanupFailedJobs(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create old failed job that exceeded retries
	oldJob := &Job{
		ID:         "job-cleanup-failed-old",
		Type:       JobTypeDocumentUpload,
		Status:     JobStatusFailed,
		RetryCount: 3,
		MaxRetries: 3,
	}
	err := repo.CreateJob(ctx, oldJob)
	require.NoError(t, err)

	// Manually set old timestamp
	oldJob.UpdatedAt = time.Now().Add(-48 * time.Hour)
	jobJSON, _ := json.Marshal(oldJob)
	err = client.Set(ctx, jobKeyPrefix+oldJob.ID, jobJSON, 0).Err()
	require.NoError(t, err)

	// Create recent failed job
	recentJob := &Job{
		ID:         "job-cleanup-failed-recent",
		Type:       JobTypeDocumentUpload,
		Status:     JobStatusFailed,
		RetryCount: 3,
		MaxRetries: 3,
	}
	err = repo.CreateJob(ctx, recentJob)
	require.NoError(t, err)

	// Cleanup
	count, err := repo.CleanupFailedJobs(ctx, 24*time.Hour, 3)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify old job is gone
	_, err = repo.GetJob(ctx, "job-cleanup-failed-old")
	assert.Error(t, err)

	// Verify recent job still exists
	_, err = repo.GetJob(ctx, "job-cleanup-failed-recent")
	assert.NoError(t, err)
}

func TestRedisJobRepository_ListJobs(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create test jobs
	jobs := []*Job{
		{ID: "job-list-1", Type: JobTypeDocumentUpload, Status: JobStatusPending, UserID: "user1", MaxRetries: 3},
		{ID: "job-list-2", Type: JobTypeDocumentUpload, Status: JobStatusCompleted, UserID: "user1", MaxRetries: 3},
		{ID: "job-list-3", Type: JobTypeDocumentDelete, Status: JobStatusPending, UserID: "user2", MaxRetries: 3},
	}

	for _, job := range jobs {
		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)
	}

	t.Run("list all jobs", func(t *testing.T) {
		allJobs, err := repo.ListJobs(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(allJobs), 3)
	})

	t.Run("filter by status", func(t *testing.T) {
		filter := &JobFilter{
			Statuses: []JobStatus{JobStatusPending},
		}
		filtered, err := repo.ListJobs(ctx, filter)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(filtered), 2)
	})

	t.Run("filter by type", func(t *testing.T) {
		filter := &JobFilter{
			Types: []JobType{JobTypeDocumentUpload},
		}
		filtered, err := repo.ListJobs(ctx, filter)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(filtered), 2)
	})

	t.Run("filter by user", func(t *testing.T) {
		filter := &JobFilter{
			UserID: "user1",
		}
		filtered, err := repo.ListJobs(ctx, filter)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(filtered), 2)
	})

	t.Run("pagination", func(t *testing.T) {
		filter := &JobFilter{
			Limit:  1,
			Offset: 0,
		}
		filtered, err := repo.ListJobs(ctx, filter)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(filtered), 1)
	})
}

func TestRedisJobRepository_GetStats(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create test jobs
	jobs := []*Job{
		{ID: "job-stats-1", Type: JobTypeDocumentUpload, Status: JobStatusPending, MaxRetries: 3},
		{ID: "job-stats-2", Type: JobTypeDocumentUpload, Status: JobStatusCompleted, MaxRetries: 3},
		{ID: "job-stats-3", Type: JobTypeDocumentDelete, Status: JobStatusFailed, MaxRetries: 3},
	}

	for _, job := range jobs {
		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)
	}

	stats, err := repo.GetStats(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalJobs, 3)
	assert.NotEmpty(t, stats.JobsByStatus)
	assert.NotEmpty(t, stats.JobsByType)
}

func TestRedisJobRepository_GetQueueLength(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	// Create and enqueue jobs
	jobs := []*Job{
		{ID: "job-qlen-1", Type: JobTypeDocumentUpload, Status: JobStatusPending, MaxRetries: 3},
		{ID: "job-qlen-2", Type: JobTypeDocumentUpload, Status: JobStatusPending, MaxRetries: 3},
	}

	for _, job := range jobs {
		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)
		err = repo.EnqueueJob(ctx, job)
		require.NoError(t, err)
	}

	// Get queue length
	length, err := repo.GetQueueLength(ctx, JobTypeDocumentUpload)
	require.NoError(t, err)
	assert.Equal(t, int64(2), length)
}

func TestRedisJobRepository_Ping(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	repo := NewRedisJobRepository(client)
	ctx := context.Background()

	err := repo.Ping(ctx)
	assert.NoError(t, err)
}

func TestJob_Validation(t *testing.T) {
	t.Run("valid job", func(t *testing.T) {
		job := &Job{
			ID:         "valid-job",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusPending,
			Progress:   50,
			MaxRetries: 3,
		}
		err := job.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid progress", func(t *testing.T) {
		job := &Job{
			ID:         "invalid-job",
			Type:       JobTypeDocumentUpload,
			Status:     JobStatusPending,
			Progress:   150, // Invalid
			MaxRetries: 3,
		}
		err := job.Validate()
		assert.Error(t, err)
	})
}

func TestJob_HelperMethods(t *testing.T) {
	t.Run("CanRetry", func(t *testing.T) {
		job := &Job{
			Status:     JobStatusFailed,
			RetryCount: 1,
			MaxRetries: 3,
		}
		assert.True(t, job.CanRetry())

		job.RetryCount = 3
		assert.False(t, job.CanRetry())
	})

	t.Run("IsComplete", func(t *testing.T) {
		job := &Job{Status: JobStatusCompleted}
		assert.True(t, job.IsComplete())

		job.Status = JobStatusProcessing
		assert.False(t, job.IsComplete())
	})

	t.Run("Duration", func(t *testing.T) {
		start := time.Now().Add(-1 * time.Hour)
		end := time.Now()
		job := &Job{
			StartedAt:   &start,
			CompletedAt: &end,
		}
		duration := job.Duration()
		assert.Greater(t, duration, 50*time.Minute)
	})
}

func TestJobStatus_Methods(t *testing.T) {
	t.Run("IsValid", func(t *testing.T) {
		assert.True(t, JobStatusPending.IsValid())
		assert.True(t, JobStatusCompleted.IsValid())
		assert.False(t, JobStatus("invalid").IsValid())
	})

	t.Run("IsTerminal", func(t *testing.T) {
		assert.True(t, JobStatusCompleted.IsTerminal())
		assert.True(t, JobStatusFailed.IsTerminal())
		assert.False(t, JobStatusProcessing.IsTerminal())
	})

	t.Run("IsActive", func(t *testing.T) {
		assert.True(t, JobStatusProcessing.IsActive())
		assert.True(t, JobStatusQueued.IsActive())
		assert.False(t, JobStatusCompleted.IsActive())
	})
}
