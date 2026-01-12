package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB provides database operations for Rivo.
type DB struct {
	pool *pgxpool.Pool
}

// New creates a new database connection pool.
func New(ctx context.Context, databaseURL string, maxConns int32) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	if maxConns > 0 {
		config.MaxConns = maxConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool.
func (d *DB) Close() {
	d.pool.Close()
}

// Pool returns the underlying connection pool.
func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// Begin starts a new transaction.
func (d *DB) Begin(ctx context.Context) (pgx.Tx, error) {
	return d.pool.Begin(ctx)
}

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

// JobRow represents a job row from the database.
type JobRow struct {
	ID             int64
	Kind           string
	Queue          string
	Namespace      string
	Priority       int16
	State          JobState
	Payload        json.RawMessage
	Metadata       json.RawMessage
	ScheduledAt    time.Time
	Attempt        int16
	MaxAttempts    int16
	AttemptedAt    *time.Time
	AttemptedBy    *string
	CompletedAt    *time.Time
	CancelledAt    *time.Time
	DiscardedAt    *time.Time
	Errors         json.RawMessage
	IdempotencyKey *string
	WorkflowRunID  *int64
	StepID         *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// InsertJobParams contains parameters for inserting a job.
type InsertJobParams struct {
	Kind           string
	Queue          string
	Namespace      string
	Priority       int16
	State          JobState
	Payload        json.RawMessage
	Metadata       json.RawMessage
	ScheduledAt    time.Time
	MaxAttempts    int16
	IdempotencyKey string
	WorkflowRunID  *int64
	StepID         string
}

// InsertJob inserts a new job into the database.
func (d *DB) InsertJob(ctx context.Context, tx pgx.Tx, params InsertJobParams) (*JobRow, bool, error) {
	var querier interface {
		QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	}
	if tx != nil {
		querier = tx
	} else {
		querier = d.pool
	}

	// Handle idempotency
	var idempotencyKey *string
	if params.IdempotencyKey != "" {
		idempotencyKey = &params.IdempotencyKey
	}

	var stepID *string
	if params.StepID != "" {
		stepID = &params.StepID
	}

	// If idempotency key is set, try to find existing job first
	if idempotencyKey != nil {
		var existing JobRow
		err := querier.QueryRow(ctx, `
			SELECT id, kind, queue, namespace, priority, state, payload, metadata,
				   scheduled_at, attempt, max_attempts, attempted_at, attempted_by,
				   completed_at, cancelled_at, discarded_at, errors, idempotency_key,
				   workflow_run_id, step_id, created_at, updated_at
			FROM rivo_jobs
			WHERE namespace = $1 AND kind = $2 AND idempotency_key = $3
			  AND state NOT IN ('completed', 'cancelled', 'discarded')
		`, params.Namespace, params.Kind, *idempotencyKey).Scan(
			&existing.ID, &existing.Kind, &existing.Queue, &existing.Namespace,
			&existing.Priority, &existing.State, &existing.Payload, &existing.Metadata,
			&existing.ScheduledAt, &existing.Attempt, &existing.MaxAttempts,
			&existing.AttemptedAt, &existing.AttemptedBy, &existing.CompletedAt,
			&existing.CancelledAt, &existing.DiscardedAt, &existing.Errors,
			&existing.IdempotencyKey, &existing.WorkflowRunID, &existing.StepID,
			&existing.CreatedAt, &existing.UpdatedAt,
		)
		if err == nil {
			return &existing, true, nil
		}
		if err != pgx.ErrNoRows {
			return nil, false, fmt.Errorf("checking idempotency: %w", err)
		}
	}

	var job JobRow
	err := querier.QueryRow(ctx, `
		INSERT INTO rivo_jobs (
			kind, queue, namespace, priority, state, payload, metadata,
			scheduled_at, max_attempts, idempotency_key, workflow_run_id, step_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, kind, queue, namespace, priority, state, payload, metadata,
				  scheduled_at, attempt, max_attempts, attempted_at, attempted_by,
				  completed_at, cancelled_at, discarded_at, errors, idempotency_key,
				  workflow_run_id, step_id, created_at, updated_at
	`,
		params.Kind, params.Queue, params.Namespace, params.Priority, params.State,
		params.Payload, params.Metadata, params.ScheduledAt, params.MaxAttempts,
		idempotencyKey, params.WorkflowRunID, stepID,
	).Scan(
		&job.ID, &job.Kind, &job.Queue, &job.Namespace, &job.Priority, &job.State,
		&job.Payload, &job.Metadata, &job.ScheduledAt, &job.Attempt, &job.MaxAttempts,
		&job.AttemptedAt, &job.AttemptedBy, &job.CompletedAt, &job.CancelledAt,
		&job.DiscardedAt, &job.Errors, &job.IdempotencyKey, &job.WorkflowRunID,
		&job.StepID, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("inserting job: %w", err)
	}

	return &job, false, nil
}

// FetchJobs fetches available jobs using SKIP LOCKED.
func (d *DB) FetchJobs(ctx context.Context, namespace string, queues []string, limit int, workerID string) ([]*JobRow, error) {
	rows, err := d.pool.Query(ctx, `
		WITH next_jobs AS (
			SELECT id
			FROM rivo_jobs
			WHERE namespace = $1
			  AND queue = ANY($2::text[])
			  AND state = 'available'
			  AND scheduled_at <= NOW()
			ORDER BY priority DESC, scheduled_at ASC, id ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		UPDATE rivo_jobs j
		SET state = 'running',
			attempt = attempt + 1,
			attempted_at = NOW(),
			attempted_by = $4,
			updated_at = NOW()
		FROM next_jobs
		WHERE j.id = next_jobs.id
		RETURNING j.id, j.kind, j.queue, j.namespace, j.priority, j.state,
				  j.payload, j.metadata, j.scheduled_at, j.attempt, j.max_attempts,
				  j.attempted_at, j.attempted_by, j.completed_at, j.cancelled_at,
				  j.discarded_at, j.errors, j.idempotency_key, j.workflow_run_id,
				  j.step_id, j.created_at, j.updated_at
	`, namespace, queues, limit, workerID)
	if err != nil {
		return nil, fmt.Errorf("fetching jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*JobRow
	for rows.Next() {
		var job JobRow
		err := rows.Scan(
			&job.ID, &job.Kind, &job.Queue, &job.Namespace, &job.Priority, &job.State,
			&job.Payload, &job.Metadata, &job.ScheduledAt, &job.Attempt, &job.MaxAttempts,
			&job.AttemptedAt, &job.AttemptedBy, &job.CompletedAt, &job.CancelledAt,
			&job.DiscardedAt, &job.Errors, &job.IdempotencyKey, &job.WorkflowRunID,
			&job.StepID, &job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning job: %w", err)
		}
		jobs = append(jobs, &job)
	}

	return jobs, rows.Err()
}

// CompleteJob marks a job as completed.
func (d *DB) CompleteJob(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_jobs
		SET state = 'completed',
			completed_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, id)
	return err
}

// FailJob marks a job as failed and schedules retry or discard.
func (d *DB) FailJob(ctx context.Context, id int64, errMsg string, retryAt *time.Time) error {
	var state JobState
	if retryAt != nil {
		state = JobStateRetryable
	} else {
		state = JobStateDiscarded
	}

	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_jobs
		SET state = $2,
			scheduled_at = COALESCE($3, scheduled_at),
			errors = errors || jsonb_build_array(jsonb_build_object(
				'attempt', attempt,
				'at', NOW(),
				'error', $4
			)),
			discarded_at = CASE WHEN $2 = 'discarded' THEN NOW() ELSE NULL END,
			updated_at = NOW()
		WHERE id = $1
	`, id, state, retryAt, errMsg)
	return err
}

// PromoteScheduledJobs moves scheduled jobs to available state.
func (d *DB) PromoteScheduledJobs(ctx context.Context, namespace string) (int64, error) {
	result, err := d.pool.Exec(ctx, `
		UPDATE rivo_jobs
		SET state = 'available',
			updated_at = NOW()
		WHERE namespace = $1
		  AND state IN ('scheduled', 'retryable')
		  AND scheduled_at <= NOW()
	`, namespace)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// GetJob retrieves a job by ID.
func (d *DB) GetJob(ctx context.Context, id int64) (*JobRow, error) {
	var job JobRow
	err := d.pool.QueryRow(ctx, `
		SELECT id, kind, queue, namespace, priority, state, payload, metadata,
			   scheduled_at, attempt, max_attempts, attempted_at, attempted_by,
			   completed_at, cancelled_at, discarded_at, errors, idempotency_key,
			   workflow_run_id, step_id, created_at, updated_at
		FROM rivo_jobs
		WHERE id = $1
	`, id).Scan(
		&job.ID, &job.Kind, &job.Queue, &job.Namespace, &job.Priority, &job.State,
		&job.Payload, &job.Metadata, &job.ScheduledAt, &job.Attempt, &job.MaxAttempts,
		&job.AttemptedAt, &job.AttemptedBy, &job.CompletedAt, &job.CancelledAt,
		&job.DiscardedAt, &job.Errors, &job.IdempotencyKey, &job.WorkflowRunID,
		&job.StepID, &job.CreatedAt, &job.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// CancelJob marks a job as cancelled.
func (d *DB) CancelJob(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_jobs
		SET state = 'cancelled',
			cancelled_at = NOW(),
			updated_at = NOW()
		WHERE id = $1 AND state NOT IN ('completed', 'cancelled', 'discarded')
	`, id)
	return err
}

// Worker represents an active worker.
type Worker struct {
	ID            string
	Namespace     string
	Queues        []string
	Concurrency   int
	Status        string
	StartedAt     time.Time
	LastHeartbeat time.Time
	JobsProcessed int64
	JobsFailed    int64
}

// RegisterWorker registers a new worker.
func (d *DB) RegisterWorker(ctx context.Context, w *Worker) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO rivo_workers (id, namespace, queues, concurrency, status, started_at, last_heartbeat)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (id) DO UPDATE SET
			status = 'active',
			last_heartbeat = NOW()
	`, w.ID, w.Namespace, w.Queues, w.Concurrency, "active", w.StartedAt)
	return err
}

// WorkerHeartbeat updates worker heartbeat.
func (d *DB) WorkerHeartbeat(ctx context.Context, workerID string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workers SET last_heartbeat = NOW() WHERE id = $1
	`, workerID)
	return err
}

// DeregisterWorker marks a worker as stopped.
func (d *DB) DeregisterWorker(ctx context.Context, workerID string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workers SET status = 'stopped' WHERE id = $1
	`, workerID)
	return err
}

// GetWorkers returns all active workers.
func (d *DB) GetWorkers(ctx context.Context, namespace string) ([]*Worker, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, namespace, queues, concurrency, status, started_at, last_heartbeat, jobs_processed, jobs_failed
		FROM rivo_workers
		WHERE namespace = $1 AND status = 'active' AND last_heartbeat > NOW() - INTERVAL '30 seconds'
		ORDER BY started_at
	`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workers []*Worker
	for rows.Next() {
		var w Worker
		if err := rows.Scan(&w.ID, &w.Namespace, &w.Queues, &w.Concurrency, &w.Status, &w.StartedAt, &w.LastHeartbeat, &w.JobsProcessed, &w.JobsFailed); err != nil {
			return nil, err
		}
		workers = append(workers, &w)
	}
	return workers, rows.Err()
}

// IncrementWorkerStats updates worker job counts.
func (d *DB) IncrementWorkerStats(ctx context.Context, workerID string, processed, failed int64) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workers
		SET jobs_processed = jobs_processed + $2, jobs_failed = jobs_failed + $3, last_heartbeat = NOW()
		WHERE id = $1
	`, workerID, processed, failed)
	return err
}

// JobLog represents a job execution log entry.
type JobLog struct {
	ID          int64
	JobID       int64
	Attempt     int16
	WorkerID    string
	StartedAt   time.Time
	CompletedAt *time.Time
	DurationMs  *int
	Input       json.RawMessage
	Output      json.RawMessage
	Status      string
	Error       string
	ErrorStack  string
	Logs        json.RawMessage
	CreatedAt   time.Time
}

// CreateJobLog creates a new job execution log.
func (d *DB) CreateJobLog(ctx context.Context, jobID int64, attempt int16, workerID string, input json.RawMessage) (int64, error) {
	var id int64
	err := d.pool.QueryRow(ctx, `
		INSERT INTO rivo_job_logs (job_id, attempt, worker_id, started_at, input, status)
		VALUES ($1, $2, $3, NOW(), $4, 'running')
		RETURNING id
	`, jobID, attempt, workerID, input).Scan(&id)
	return id, err
}

// CompleteJobLog marks a job log as completed.
func (d *DB) CompleteJobLog(ctx context.Context, logID int64, output json.RawMessage, logs json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_job_logs
		SET status = 'completed',
			completed_at = NOW(),
			duration_ms = EXTRACT(MILLISECONDS FROM NOW() - started_at)::INTEGER,
			output = $2,
			logs = $3
		WHERE id = $1
	`, logID, output, logs)
	return err
}

// FailJobLog marks a job log as failed.
func (d *DB) FailJobLog(ctx context.Context, logID int64, errMsg, errStack string, logs json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_job_logs
		SET status = 'failed',
			completed_at = NOW(),
			duration_ms = EXTRACT(MILLISECONDS FROM NOW() - started_at)::INTEGER,
			error = $2,
			error_stack = $3,
			logs = $4
		WHERE id = $1
	`, logID, errMsg, errStack, logs)
	return err
}

// GetJobLogs returns all execution logs for a job.
func (d *DB) GetJobLogs(ctx context.Context, jobID int64) ([]*JobLog, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, job_id, attempt, worker_id, started_at, completed_at, duration_ms,
			   input, output, status, COALESCE(error, ''), COALESCE(error_stack, ''), logs, created_at
		FROM rivo_job_logs
		WHERE job_id = $1
		ORDER BY attempt DESC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*JobLog
	for rows.Next() {
		var l JobLog
		if err := rows.Scan(&l.ID, &l.JobID, &l.Attempt, &l.WorkerID, &l.StartedAt, &l.CompletedAt, &l.DurationMs,
			&l.Input, &l.Output, &l.Status, &l.Error, &l.ErrorStack, &l.Logs, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &l)
	}
	return logs, rows.Err()
}

// QueueStats represents queue statistics.
type QueueStats struct {
	Namespace string
	Queue     string
	Available int64
	Running   int64
	Completed int64
	Retryable int64
	Failed    int64
	Total     int64
}

// GetQueueStats returns statistics for all queues.
func (d *DB) GetQueueStats(ctx context.Context, namespace string) ([]*QueueStats, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT namespace, queue,
			   COALESCE(available, 0), COALESCE(running, 0), COALESCE(completed, 0),
			   COALESCE(retryable, 0), COALESCE(failed, 0), COALESCE(total, 0)
		FROM rivo_queue_stats
		WHERE namespace = $1
		ORDER BY queue
	`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*QueueStats
	for rows.Next() {
		var s QueueStats
		if err := rows.Scan(&s.Namespace, &s.Queue, &s.Available, &s.Running, &s.Completed, &s.Retryable, &s.Failed, &s.Total); err != nil {
			return nil, err
		}
		stats = append(stats, &s)
	}
	return stats, rows.Err()
}

// QueueConfig represents queue configuration.
type QueueConfig struct {
	Name        string
	Namespace   string
	Concurrency int
	Paused      bool
	Priority    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewQueueConfig creates a new QueueConfig.
func (d *DB) NewQueueConfig(name, namespace string, concurrency int, paused bool, priority int) *QueueConfig {
	return &QueueConfig{
		Name:        name,
		Namespace:   namespace,
		Concurrency: concurrency,
		Paused:      paused,
		Priority:    priority,
	}
}

// GetQueues returns all queue configurations.
func (d *DB) GetQueues(ctx context.Context, namespace string) ([]*QueueConfig, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT name, namespace, concurrency, paused, priority, created_at, updated_at
		FROM rivo_queues
		WHERE namespace = $1
		ORDER BY priority DESC, name
	`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queues []*QueueConfig
	for rows.Next() {
		var q QueueConfig
		if err := rows.Scan(&q.Name, &q.Namespace, &q.Concurrency, &q.Paused, &q.Priority, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		queues = append(queues, &q)
	}
	return queues, rows.Err()
}

// UpsertQueue creates or updates a queue configuration.
func (d *DB) UpsertQueue(ctx context.Context, q *QueueConfig) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO rivo_queues (name, namespace, concurrency, paused, priority)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (namespace, name) DO UPDATE SET
			concurrency = EXCLUDED.concurrency,
			paused = EXCLUDED.paused,
			priority = EXCLUDED.priority,
			updated_at = NOW()
	`, q.Name, q.Namespace, q.Concurrency, q.Paused, q.Priority)
	return err
}

// UpdateQueueConcurrency updates queue concurrency.
func (d *DB) UpdateQueueConcurrency(ctx context.Context, namespace, name string, concurrency int) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_queues SET concurrency = $3, updated_at = NOW()
		WHERE namespace = $1 AND name = $2
	`, namespace, name, concurrency)
	return err
}

// PauseQueue pauses or resumes a queue.
func (d *DB) PauseQueue(ctx context.Context, namespace, name string, paused bool) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_queues SET paused = $3, updated_at = NOW()
		WHERE namespace = $1 AND name = $2
	`, namespace, name, paused)
	return err
}

// GetQueueConcurrency returns queue concurrency (default if not configured).
func (d *DB) GetQueueConcurrency(ctx context.Context, namespace, name string) (int, bool, error) {
	var concurrency int
	var paused bool
	err := d.pool.QueryRow(ctx, `
		SELECT concurrency, paused FROM rivo_queues
		WHERE namespace = $1 AND name = $2
	`, namespace, name).Scan(&concurrency, &paused)
	if err == pgx.ErrNoRows {
		return 10, false, nil // default concurrency
	}
	if err != nil {
		return 0, false, err
	}
	return concurrency, paused, nil
}

// WorkerHealth represents worker health status.
type WorkerHealth struct {
	ID            string
	Status        string
	IsHealthy     bool
	LastHeartbeat time.Time
	HeartbeatAge  time.Duration
	JobsProcessed int64
	JobsFailed    int64
}

// GetWorkerHealth returns health status for all workers.
func (d *DB) GetWorkerHealth(ctx context.Context, namespace string) ([]*WorkerHealth, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT
			id,
			status,
			last_heartbeat,
			NOW() - last_heartbeat as heartbeat_age,
			last_heartbeat > NOW() - INTERVAL '30 seconds' as is_healthy,
			jobs_processed,
			jobs_failed
		FROM rivo_workers
		WHERE namespace = $1 AND last_heartbeat > NOW() - INTERVAL '5 minutes'
		ORDER BY started_at
	`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var health []*WorkerHealth
	for rows.Next() {
		var h WorkerHealth
		if err := rows.Scan(&h.ID, &h.Status, &h.LastHeartbeat, &h.HeartbeatAge, &h.IsHealthy, &h.JobsProcessed, &h.JobsFailed); err != nil {
			return nil, err
		}
		health = append(health, &h)
	}
	return health, rows.Err()
}

// CleanupStaleWorkers marks stale workers as stopped.
func (d *DB) CleanupStaleWorkers(ctx context.Context, threshold time.Duration) (int64, error) {
	result, err := d.pool.Exec(ctx, `
		UPDATE rivo_workers
		SET status = 'stopped'
		WHERE status = 'active' AND last_heartbeat < NOW() - $1::INTERVAL
	`, threshold.String())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// ListJobs returns jobs with optional filtering.
func (d *DB) ListJobs(ctx context.Context, namespace string, state string, queue string, limit int) ([]*JobRow, error) {
	query := `
		SELECT id, kind, queue, namespace, priority, state, payload, metadata,
			   scheduled_at, attempt, max_attempts, attempted_at, attempted_by,
			   completed_at, cancelled_at, discarded_at, errors, idempotency_key,
			   workflow_run_id, step_id, created_at, updated_at
		FROM rivo_jobs
		WHERE namespace = $1
	`
	args := []any{namespace}
	argNum := 2

	if state != "" {
		query += fmt.Sprintf(" AND state = $%d", argNum)
		args = append(args, state)
		argNum++
	}
	if queue != "" {
		query += fmt.Sprintf(" AND queue = $%d", argNum)
		args = append(args, queue)
		argNum++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argNum)
	args = append(args, limit)

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*JobRow
	for rows.Next() {
		var job JobRow
		err := rows.Scan(
			&job.ID, &job.Kind, &job.Queue, &job.Namespace, &job.Priority, &job.State,
			&job.Payload, &job.Metadata, &job.ScheduledAt, &job.Attempt, &job.MaxAttempts,
			&job.AttemptedAt, &job.AttemptedBy, &job.CompletedAt, &job.CancelledAt,
			&job.DiscardedAt, &job.Errors, &job.IdempotencyKey, &job.WorkflowRunID,
			&job.StepID, &job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}
	return jobs, rows.Err()
}

// WorkflowDefinitionRow represents a workflow definition in the database.
type WorkflowDefinitionRow struct {
	ID         int64
	Name       string
	Version    int
	Namespace  string
	Definition json.RawMessage
	CreatedAt  time.Time
}

// WorkflowRunRow represents a workflow run in the database.
type WorkflowRunRow struct {
	ID           int64
	DefinitionID int64
	Namespace    string
	State        string
	Input        json.RawMessage
	Output       json.RawMessage
	StepStates   json.RawMessage
	CurrentStep  string
	Error        string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// InsertWorkflowDefinition inserts a new workflow definition.
func (d *DB) InsertWorkflowDefinition(ctx context.Context, name, namespace string, definition json.RawMessage) (*WorkflowDefinitionRow, error) {
	var row WorkflowDefinitionRow
	err := d.pool.QueryRow(ctx, `
		INSERT INTO rivo_workflow_definitions (name, namespace, definition)
		VALUES ($1, $2, $3)
		ON CONFLICT (namespace, name, version) DO UPDATE SET
			definition = EXCLUDED.definition
		RETURNING id, name, version, namespace, definition, created_at
	`, name, namespace, definition).Scan(
		&row.ID, &row.Name, &row.Version, &row.Namespace, &row.Definition, &row.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting workflow definition: %w", err)
	}
	return &row, nil
}

// GetWorkflowDefinition retrieves a workflow definition by name.
func (d *DB) GetWorkflowDefinition(ctx context.Context, namespace, name string) (*WorkflowDefinitionRow, error) {
	var row WorkflowDefinitionRow
	err := d.pool.QueryRow(ctx, `
		SELECT id, name, version, namespace, definition, created_at
		FROM rivo_workflow_definitions
		WHERE namespace = $1 AND name = $2
		ORDER BY version DESC
		LIMIT 1
	`, namespace, name).Scan(
		&row.ID, &row.Name, &row.Version, &row.Namespace, &row.Definition, &row.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting workflow definition: %w", err)
	}
	return &row, nil
}

// CreateWorkflowRun creates a new workflow run.
func (d *DB) CreateWorkflowRun(ctx context.Context, definitionID int64, namespace string, input json.RawMessage, stepStates json.RawMessage) (*WorkflowRunRow, error) {
	var row WorkflowRunRow
	err := d.pool.QueryRow(ctx, `
		INSERT INTO rivo_workflow_runs (definition_id, namespace, state, input, step_states)
		VALUES ($1, $2, 'pending', $3, $4)
		RETURNING id, definition_id, namespace, state, input, output, step_states,
		          COALESCE(error, ''), started_at, completed_at, created_at, updated_at
	`, definitionID, namespace, input, stepStates).Scan(
		&row.ID, &row.DefinitionID, &row.Namespace, &row.State, &row.Input, &row.Output,
		&row.StepStates, &row.Error, &row.StartedAt, &row.CompletedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating workflow run: %w", err)
	}
	return &row, nil
}

// UpdateWorkflowRunState updates the state of a workflow run.
func (d *DB) UpdateWorkflowRunState(ctx context.Context, id int64, state string, currentStep string, errMsg string) error {
	var errorVal *string
	if errMsg != "" {
		errorVal = &errMsg
	}

	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workflow_runs
		SET state = $2,
		    started_at = CASE WHEN $2 = 'running' AND started_at IS NULL THEN NOW() ELSE started_at END,
		    completed_at = CASE WHEN $2 IN ('completed', 'failed', 'cancelled') THEN NOW() ELSE completed_at END,
		    error = COALESCE($3, error),
		    updated_at = NOW()
		WHERE id = $1
	`, id, state, errorVal)
	return err
}

// UpdateWorkflowRunStepStates updates the step states of a workflow run.
func (d *DB) UpdateWorkflowRunStepStates(ctx context.Context, id int64, stepStates json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workflow_runs
		SET step_states = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, id, stepStates)
	return err
}

// UpdateWorkflowRunOutput updates the output of a completed workflow run.
func (d *DB) UpdateWorkflowRunOutput(ctx context.Context, id int64, output json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workflow_runs
		SET output = $2, updated_at = NOW()
		WHERE id = $1
	`, id, output)
	return err
}

// GetWorkflowRun retrieves a workflow run by ID.
func (d *DB) GetWorkflowRun(ctx context.Context, id int64) (*WorkflowRunRow, error) {
	var row WorkflowRunRow
	err := d.pool.QueryRow(ctx, `
		SELECT id, definition_id, namespace, state, input, output, step_states,
		       COALESCE(error, ''), started_at, completed_at, created_at, updated_at
		FROM rivo_workflow_runs
		WHERE id = $1
	`, id).Scan(
		&row.ID, &row.DefinitionID, &row.Namespace, &row.State, &row.Input, &row.Output,
		&row.StepStates, &row.Error, &row.StartedAt, &row.CompletedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting workflow run: %w", err)
	}
	return &row, nil
}

// ListWorkflowRuns returns workflow runs with optional filtering.
func (d *DB) ListWorkflowRuns(ctx context.Context, namespace string, state string, limit int) ([]*WorkflowRunRow, error) {
	query := `
		SELECT id, definition_id, namespace, state, input, output, step_states,
		       COALESCE(error, ''), started_at, completed_at, created_at, updated_at
		FROM rivo_workflow_runs
		WHERE namespace = $1
	`
	args := []any{namespace}
	argNum := 2

	if state != "" {
		query += fmt.Sprintf(" AND state = $%d", argNum)
		args = append(args, state)
		argNum++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argNum)
	args = append(args, limit)

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*WorkflowRunRow
	for rows.Next() {
		var row WorkflowRunRow
		err := rows.Scan(
			&row.ID, &row.DefinitionID, &row.Namespace, &row.State, &row.Input, &row.Output,
			&row.StepStates, &row.Error, &row.StartedAt, &row.CompletedAt, &row.CreatedAt, &row.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		runs = append(runs, &row)
	}
	return runs, rows.Err()
}

// GetWorkflowDefinitionByID retrieves a workflow definition by ID.
func (d *DB) GetWorkflowDefinitionByID(ctx context.Context, id int64) (*WorkflowDefinitionRow, error) {
	var row WorkflowDefinitionRow
	err := d.pool.QueryRow(ctx, `
		SELECT id, name, version, namespace, definition, created_at
		FROM rivo_workflow_definitions
		WHERE id = $1
	`, id).Scan(
		&row.ID, &row.Name, &row.Version, &row.Namespace, &row.Definition, &row.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting workflow definition by ID: %w", err)
	}
	return &row, nil
}

// WorkflowStepLog represents a workflow step execution log entry.
type WorkflowStepLog struct {
	ID            int64
	WorkflowRunID int64
	StepID        string
	Attempt       int
	WorkerID      string
	StartedAt     time.Time
	CompletedAt   *time.Time
	DurationMs    *int
	Input         json.RawMessage
	Output        json.RawMessage
	Status        string
	Error         string
	ErrorStack    string
	Logs          json.RawMessage
	CreatedAt     time.Time
}

// CreateWorkflowStepLog creates a new workflow step execution log.
func (d *DB) CreateWorkflowStepLog(ctx context.Context, runID int64, stepID string, attempt int, workerID string, input json.RawMessage) (int64, error) {
	var id int64
	err := d.pool.QueryRow(ctx, `
		INSERT INTO rivo_workflow_step_logs (workflow_run_id, step_id, attempt, worker_id, started_at, input, status)
		VALUES ($1, $2, $3, $4, NOW(), $5, 'running')
		RETURNING id
	`, runID, stepID, attempt, workerID, input).Scan(&id)
	return id, err
}

// CompleteWorkflowStepLog marks a workflow step log as completed.
func (d *DB) CompleteWorkflowStepLog(ctx context.Context, logID int64, output json.RawMessage, logs json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workflow_step_logs
		SET status = 'completed',
			completed_at = NOW(),
			duration_ms = EXTRACT(MILLISECONDS FROM NOW() - started_at)::INTEGER,
			output = $2,
			logs = $3
		WHERE id = $1
	`, logID, output, logs)
	return err
}

// FailWorkflowStepLog marks a workflow step log as failed.
func (d *DB) FailWorkflowStepLog(ctx context.Context, logID int64, errMsg, errStack string, logs json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workflow_step_logs
		SET status = 'failed',
			completed_at = NOW(),
			duration_ms = EXTRACT(MILLISECONDS FROM NOW() - started_at)::INTEGER,
			error = $2,
			error_stack = $3,
			logs = $4
		WHERE id = $1
	`, logID, errMsg, errStack, logs)
	return err
}

// GetWorkflowStepLogs returns all execution logs for a workflow run.
func (d *DB) GetWorkflowStepLogs(ctx context.Context, runID int64) ([]*WorkflowStepLog, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, workflow_run_id, step_id, attempt, COALESCE(worker_id, ''), started_at, completed_at, duration_ms,
			   input, output, status, COALESCE(error, ''), COALESCE(error_stack, ''), logs, created_at
		FROM rivo_workflow_step_logs
		WHERE workflow_run_id = $1
		ORDER BY started_at DESC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*WorkflowStepLog
	for rows.Next() {
		var l WorkflowStepLog
		if err := rows.Scan(&l.ID, &l.WorkflowRunID, &l.StepID, &l.Attempt, &l.WorkerID, &l.StartedAt, &l.CompletedAt, &l.DurationMs,
			&l.Input, &l.Output, &l.Status, &l.Error, &l.ErrorStack, &l.Logs, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &l)
	}
	return logs, rows.Err()
}

// GetWorkflowStepLogsByStep returns execution logs for a specific step.
func (d *DB) GetWorkflowStepLogsByStep(ctx context.Context, runID int64, stepID string) ([]*WorkflowStepLog, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, workflow_run_id, step_id, attempt, COALESCE(worker_id, ''), started_at, completed_at, duration_ms,
			   input, output, status, COALESCE(error, ''), COALESCE(error_stack, ''), logs, created_at
		FROM rivo_workflow_step_logs
		WHERE workflow_run_id = $1 AND step_id = $2
		ORDER BY attempt DESC
	`, runID, stepID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*WorkflowStepLog
	for rows.Next() {
		var l WorkflowStepLog
		if err := rows.Scan(&l.ID, &l.WorkflowRunID, &l.StepID, &l.Attempt, &l.WorkerID, &l.StartedAt, &l.CompletedAt, &l.DurationMs,
			&l.Input, &l.Output, &l.Status, &l.Error, &l.ErrorStack, &l.Logs, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &l)
	}
	return logs, rows.Err()
}

// ResetWorkflowStepForRerun resets a step and all subsequent steps for rerunning.
func (d *DB) ResetWorkflowStepForRerun(ctx context.Context, runID int64, stepStates json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE rivo_workflow_runs
		SET step_states = $2,
		    state = 'running',
		    completed_at = NULL,
		    error = '',
		    updated_at = NOW()
		WHERE id = $1
	`, runID, stepStates)
	return err
}

// ScriptHandlerRow represents a script handler from the database.
type ScriptHandlerRow struct {
	Kind        string
	Namespace   string
	HandlerType string
	Code        string
	Config      json.RawMessage
	Version     int
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GetScriptHandlers returns all enabled script handlers for a namespace.
func (d *DB) GetScriptHandlers(ctx context.Context, namespace string) ([]*ScriptHandlerRow, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT kind, namespace, handler_type, COALESCE(code, ''), config, version, enabled, created_at, updated_at
		FROM rivo_handlers
		WHERE namespace = $1 AND enabled = true
		ORDER BY kind
	`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var handlers []*ScriptHandlerRow
	for rows.Next() {
		var h ScriptHandlerRow
		if err := rows.Scan(&h.Kind, &h.Namespace, &h.HandlerType, &h.Code, &h.Config, &h.Version, &h.Enabled, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		handlers = append(handlers, &h)
	}
	return handlers, rows.Err()
}

// GetScriptHandler returns a single script handler by kind.
func (d *DB) GetScriptHandler(ctx context.Context, namespace, kind string) (*ScriptHandlerRow, error) {
	var h ScriptHandlerRow
	err := d.pool.QueryRow(ctx, `
		SELECT kind, namespace, handler_type, COALESCE(code, ''), config, version, enabled, created_at, updated_at
		FROM rivo_handlers
		WHERE namespace = $1 AND kind = $2
	`, namespace, kind).Scan(&h.Kind, &h.Namespace, &h.HandlerType, &h.Code, &h.Config, &h.Version, &h.Enabled, &h.CreatedAt, &h.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// UpsertScriptHandler creates or updates a script handler.
func (d *DB) UpsertScriptHandler(ctx context.Context, h *ScriptHandlerRow) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO rivo_handlers (kind, namespace, handler_type, code, config, version, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (kind) DO UPDATE SET
			handler_type = EXCLUDED.handler_type,
			code = EXCLUDED.code,
			config = EXCLUDED.config,
			version = rivo_handlers.version + 1,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()
	`, h.Kind, h.Namespace, h.HandlerType, h.Code, h.Config, h.Version, h.Enabled)
	return err
}

// DeleteScriptHandler deletes a script handler.
func (d *DB) DeleteScriptHandler(ctx context.Context, namespace, kind string) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM rivo_handlers WHERE namespace = $1 AND kind = $2`, namespace, kind)
	return err
}
