package rivo

import (
	"encoding/json"
	"time"
)

// JobState represents the current state of a job.
type JobState string

const (
	JobStateAvailable JobState = "available"
	JobStateScheduled JobState = "scheduled"
	JobStateRunning   JobState = "running"
	JobStateRetryable JobState = "retryable"
	JobStateCompleted JobState = "completed"
	JobStateCancelled JobState = "cancelled"
	JobStateDiscarded JobState = "discarded"
)

// Job represents a single unit of work.
type Job struct {
	ID            int64           `json:"id"`
	Kind          string          `json:"kind"`
	Queue         string          `json:"queue"`
	Namespace     string          `json:"namespace"`
	Priority      int16           `json:"priority"`
	State         JobState        `json:"state"`
	Payload       json.RawMessage `json:"payload"`
	Metadata      Metadata        `json:"metadata"`
	ScheduledAt   time.Time       `json:"scheduled_at"`
	Attempt       int16           `json:"attempt"`
	MaxAttempts   int16           `json:"max_attempts"`
	AttemptedAt   *time.Time      `json:"attempted_at,omitempty"`
	AttemptedBy   string          `json:"attempted_by,omitempty"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	CancelledAt   *time.Time      `json:"cancelled_at,omitempty"`
	DiscardedAt   *time.Time      `json:"discarded_at,omitempty"`
	Errors        []AttemptError  `json:"errors"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	WorkflowRunID *int64          `json:"workflow_run_id,omitempty"`
	StepID        string          `json:"step_id,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Metadata provides typed access to job metadata.
type Metadata map[string]any

// AttemptError records an error from a previous attempt.
type AttemptError struct {
	Attempt   int16     `json:"attempt"`
	At        time.Time `json:"at"`
	Error     string    `json:"error"`
	Traceback string    `json:"traceback,omitempty"`
}

// UnmarshalPayload decodes the job payload into the provided value.
func (j *Job) UnmarshalPayload(v any) error {
	return json.Unmarshal(j.Payload, v)
}

// IsTerminal returns true if the job is in a terminal state.
func (j *Job) IsTerminal() bool {
	switch j.State {
	case JobStateCompleted, JobStateCancelled, JobStateDiscarded:
		return true
	default:
		return false
	}
}
