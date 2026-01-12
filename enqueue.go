package rivo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rabin-a/rivo/internal/db"
)

// EnqueueParams configures job enqueuing.
type EnqueueParams struct {
	// Kind identifies the job type (required).
	Kind string

	// Payload is the job data (will be JSON marshaled).
	Payload any

	// Queue is the queue name. Defaults to "default".
	Queue string

	// Priority affects job ordering. Higher values are processed first.
	Priority int16

	// ScheduleAt delays execution until this time.
	ScheduleAt time.Time

	// IdempotencyKey prevents duplicate job creation.
	IdempotencyKey string

	// Metadata stores additional job metadata.
	Metadata Metadata

	// MaxAttempts overrides the default maximum attempts.
	MaxAttempts int16

	// WorkflowRunID associates this job with a workflow run.
	WorkflowRunID *int64

	// StepID identifies this job within a workflow.
	StepID string
}

// EnqueueResult contains the result of enqueuing a job.
type EnqueueResult struct {
	// Job is the created or found job.
	Job *Job

	// Duplicate is true if an existing job matched the idempotency key.
	Duplicate bool
}

// Enqueue adds a job to the queue.
func (c *Client) Enqueue(ctx context.Context, params EnqueueParams) (*EnqueueResult, error) {
	return c.enqueueJob(ctx, nil, params)
}

// EnqueueTx enqueues a job within an existing transaction.
func (c *Client) EnqueueTx(ctx context.Context, tx Tx, params EnqueueParams) (*EnqueueResult, error) {
	return c.enqueueJob(ctx, tx, params)
}

// EnqueueMany inserts multiple jobs efficiently.
func (c *Client) EnqueueMany(ctx context.Context, params []EnqueueParams) ([]*EnqueueResult, error) {
	results := make([]*EnqueueResult, len(params))
	for i, p := range params {
		result, err := c.Enqueue(ctx, p)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

func (c *Client) enqueueJob(ctx context.Context, tx Tx, params EnqueueParams) (*EnqueueResult, error) {
	if params.Kind == "" {
		return nil, ErrInvalidJobKind
	}

	queue := params.Queue
	if queue == "" {
		queue = "default"
	}

	scheduledAt := params.ScheduleAt
	if scheduledAt.IsZero() {
		scheduledAt = time.Now()
	}

	state := JobStateAvailable
	if scheduledAt.After(time.Now()) {
		state = JobStateScheduled
	}

	maxAttempts := params.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 25
	}

	var payload json.RawMessage
	if params.Payload != nil {
		var err error
		payload, err = json.Marshal(params.Payload)
		if err != nil {
			return nil, err
		}
	} else {
		payload = json.RawMessage("{}")
	}

	metadata := params.Metadata
	if metadata == nil {
		metadata = Metadata{}
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	jobRow, duplicate, err := c.db.InsertJob(ctx, tx, db.InsertJobParams{
		Kind:           params.Kind,
		Queue:          queue,
		Namespace:      c.config.Namespace,
		Priority:       params.Priority,
		State:          db.JobState(state),
		Payload:        payload,
		Metadata:       metadataJSON,
		ScheduledAt:    scheduledAt,
		MaxAttempts:    maxAttempts,
		IdempotencyKey: params.IdempotencyKey,
		WorkflowRunID:  params.WorkflowRunID,
		StepID:         params.StepID,
	})
	if err != nil {
		return nil, err
	}

	return &EnqueueResult{
		Job:       c.jobFromRow(jobRow),
		Duplicate: duplicate,
	}, nil
}
