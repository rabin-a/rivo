package rivo

import (
	"context"
)

// contextKey is a custom type for context keys.
type contextKey string

const (
	loggerKey contextKey = "rivo_logger"
	jobKey    contextKey = "rivo_job"
)

// JobContext wraps a context.Context and provides job-specific functionality.
type JobContext struct {
	context.Context
	logger *JobLogger
	job    *Job
}

// NewJobContext creates a new JobContext.
func NewJobContext(ctx context.Context, job *Job) *JobContext {
	logger := NewJobLogger()
	return &JobContext{
		Context: ctx,
		logger:  logger,
		job:     job,
	}
}

// Logger returns the job logger for structured logging during job execution.
func (c *JobContext) Logger() *JobLogger {
	return c.logger
}

// Job returns the current job.
func (c *JobContext) Job() *Job {
	return c.job
}

// Value returns the value associated with the key.
func (c *JobContext) Value(key any) any {
	switch key {
	case loggerKey:
		return c.logger
	case jobKey:
		return c.job
	}
	return c.Context.Value(key)
}

// LoggerFromContext extracts the JobLogger from a context.
func LoggerFromContext(ctx context.Context) *JobLogger {
	if jc, ok := ctx.(*JobContext); ok {
		return jc.logger
	}
	if logger, ok := ctx.Value(loggerKey).(*JobLogger); ok {
		return logger
	}
	return nil
}

// JobFromContext extracts the Job from a context.
func JobFromContext(ctx context.Context) *Job {
	if jc, ok := ctx.(*JobContext); ok {
		return jc.job
	}
	if job, ok := ctx.Value(jobKey).(*Job); ok {
		return job
	}
	return nil
}
