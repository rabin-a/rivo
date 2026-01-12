package rivo

import (
	"context"
	"time"
)

// ScheduleParams configures a periodic job.
type ScheduleParams struct {
	// Name is a unique identifier for this schedule (required).
	Name string

	// Kind is the job type to enqueue (required).
	Kind string

	// CronExpr is the cron expression (required).
	// Supports standard 5-field cron syntax: minute hour day month weekday
	CronExpr string

	// Queue is the queue name. Defaults to "default".
	Queue string

	// Payload is the job data (will be JSON marshaled).
	Payload any

	// Timezone for cron evaluation. Defaults to "UTC".
	Timezone string

	// Enabled controls whether the schedule is active.
	Enabled bool
}

// PeriodicJob represents a scheduled recurring job.
type PeriodicJob struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Namespace  string    `json:"namespace"`
	Kind       string    `json:"kind"`
	Queue      string    `json:"queue"`
	CronExpr   string    `json:"cron_expr"`
	Timezone   string    `json:"timezone"`
	Enabled    bool      `json:"enabled"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	NextRunAt  time.Time  `json:"next_run_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Schedule creates or updates a periodic job schedule.
func (c *Client) Schedule(ctx context.Context, params ScheduleParams) (*PeriodicJob, error) {
	// This is a placeholder for the full scheduler implementation.
	// The full implementation would:
	// 1. Parse the cron expression
	// 2. Calculate the next run time
	// 3. Insert/update the rivo_periodic_jobs table
	// 4. Return the created/updated schedule

	// For now, we'll return an error indicating this needs full implementation
	return nil, ErrNotImplemented
}

// Unschedule removes a periodic job schedule.
func (c *Client) Unschedule(ctx context.Context, name string) error {
	return ErrNotImplemented
}

// GetSchedule retrieves a periodic job schedule by name.
func (c *Client) GetSchedule(ctx context.Context, name string) (*PeriodicJob, error) {
	return nil, ErrNotImplemented
}

// ListSchedules lists all periodic job schedules.
func (c *Client) ListSchedules(ctx context.Context) ([]*PeriodicJob, error) {
	return nil, ErrNotImplemented
}

// ErrNotImplemented indicates a feature is not yet implemented.
var ErrNotImplemented = errorString("not implemented")

type errorString string

func (e errorString) Error() string { return string(e) }
