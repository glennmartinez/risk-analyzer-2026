package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"risk-analyzer/internal/repositories"
)

type JobState string
type JobEvent string

const (
	StatePending    JobState = "pending"
	StateQueued     JobState = "queued"
	StateProcessing JobState = "processing"
	StateSucceeded  JobState = "succeeded"
	StateFailed     JobState = "failed"
	StateRetrying   JobState = "retrying"
	StateCancelled  JobState = "cancelled"

	EventEnqueue         JobEvent = "enqueue"
	EventStart           JobEvent = "start"
	EventSuccess         JobEvent = "success"
	EventFailure         JobEvent = "failure"
	EventCallbackReceipt JobEvent = "callback_received"
	EventRetry           JobEvent = "retry"
	EventCancel          JobEvent = "cancel"
)

// JobProcessor is implemented per job type to perform actual work or schedule it
type JobProcessor interface {
	// StartProcessing is invoked when the state machine decides to process the job.
	// Implementation should perform processing or trigger external processing (eg call Python)
	// and return an error only if the immediate kickoff failed.
	StartProcessing(ctx context.Context, job *repositories.Job) error
}

type JobStateMachine struct {
	jobRepo    repositories.JobRepository
	processors map[repositories.JobType]JobProcessor
}

func NewJobStateMachine(jobRepo repositories.JobRepository) *JobStateMachine {
	return &JobStateMachine{
		jobRepo:    jobRepo,
		processors: make(map[repositories.JobType]JobProcessor),
	}
}

func (m *JobStateMachine) RegisterProcessor(jobType repositories.JobType, p JobProcessor) {
	m.processors[jobType] = p
}

// HandleEvent applies an event to a job and runs side-effects.
// This centralizes transition rules so tests and logic are consistent.
func (m *JobStateMachine) HandleEvent(ctx context.Context, jobID string, event JobEvent, payload map[string]interface{}) error {
	job, err := m.jobRepo.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	log.Printf("Handling event %s for job %s (current status: %s)", event, job.ID, job.Status)

	// State transition logic
	switch event {
	case EventEnqueue:
		job.Status = repositories.JobStatusQueued
		job.UpdatedAt = time.Now()
		if err := m.jobRepo.UpdateJob(ctx, job); err != nil {
			return err
		}
		return m.jobRepo.EnqueueJob(ctx, job)

	case EventStart:
		// mark processing and call processor
		job.Status = repositories.JobStatusProcessing
		job.UpdatedAt = time.Now()
		if err := m.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusProcessing, 0, "Processing started"); err != nil {
			return err
		}
		if p, ok := m.processors[job.Type]; ok {
			//log here that processing is starting
			log.Printf("Starting processing for job %s of type %s", job.ID, job.Type)
			return p.StartProcessing(ctx, job)
		}
		return fmt.Errorf("no processor registered for job type %s", job.Type)

	case EventSuccess:
		job.Status = repositories.JobStatusCompleted
		job.UpdatedAt = time.Now()
		if err := m.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusCompleted, 100, "Completed"); err != nil {
			return err
		}
		// persist result if provided
		if payload != nil {
			_ = m.jobRepo.UpdateJobResult(ctx, job.ID, payload)
		}
		return nil

	case EventFailure:
		// payload may include `permanent` bool
		isPermanent := false
		if payload != nil {
			if v, ok := payload["permanent"].(bool); ok && v {
				isPermanent = true
			}
		}
		if !isPermanent && job.RetryCount < job.MaxRetries {
			job.RetryCount++
			_ = m.jobRepo.UpdateJob(ctx, job)
			job.Status = repositories.JobStatusRetrying
			_ = m.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusRetrying, 0, "Retrying")
			// re-enqueue after simple backoff (could be scheduled via separate scheduler)
			time.Sleep(2 * time.Second)
			return m.jobRepo.EnqueueJob(ctx, job)
		}
		// permanent failure
		job.Status = repositories.JobStatusFailed
		_ = m.jobRepo.UpdateJobStatus(ctx, job.ID, repositories.JobStatusFailed, 0, "Failed")
		return nil

	case EventCallbackReceipt:
		// payload contains callback data; map to success/failure and update job/document accordingly
		statusStr, _ := payload["status"].(string)
		if statusStr == "completed" || statusStr == "success" {
			return m.HandleEvent(ctx, jobID, EventSuccess, payload)
		}
		return m.HandleEvent(ctx, jobID, EventFailure, payload)
	default:
		return fmt.Errorf("unsupported event: %s", event)
	}
}
