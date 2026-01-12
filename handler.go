package rivo

import (
	"time"
)

// Handler processes jobs of a specific kind.
type Handler interface {
	Kind() string
	Handle(ctx *JobContext, job *Job) error
}

// HandlerFunc is a function adapter for creating handlers.
// The JobContext provides access to Logger() for structured logging.
type HandlerFunc func(ctx *JobContext, job *Job) error

// HandlerOptions configures handler behavior.
type HandlerOptions struct {
	// Queue specifies which queue this handler processes.
	// If empty, uses the job kind as the queue name.
	Queue string

	// Retry configuration for failed jobs.
	Retry RetryPolicy

	// Timeout for individual job execution. Zero means no timeout.
	Timeout time.Duration

	// Concurrency limit for this handler. Zero means use default.
	Concurrency int
}

// RetryPolicy defines retry behavior for failed jobs.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of attempts (including first).
	// Zero means use default (25).
	MaxAttempts int

	// Backoff calculates the delay before the next retry.
	// If nil, uses exponential backoff.
	Backoff BackoffFunc

	// Jitter adds randomness to backoff delays to prevent thundering herd.
	Jitter bool

	// RetryableErrors limits retries to only these error types.
	// If nil, all errors are retryable.
	RetryableErrors []error

	// DiscardErrors causes immediate job discard on these errors.
	DiscardErrors []error
}

// BackoffFunc calculates the delay before the next retry attempt.
type BackoffFunc func(attempt int) time.Duration

// FixedBackoff returns a backoff function with constant delay.
func FixedBackoff(d time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		return d
	}
}

// ExponentialBackoff returns a backoff function with exponential delay.
// The delay doubles each attempt, starting from initial, up to max.
func ExponentialBackoff(initial, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		delay := initial
		for i := 1; i < attempt; i++ {
			delay *= 2
			if delay > max {
				return max
			}
		}
		return delay
	}
}

// LinearBackoff returns a backoff function with linearly increasing delay.
func LinearBackoff(initial, increment time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		return initial + time.Duration(attempt-1)*increment
	}
}

// registeredHandler wraps a handler with its options.
type registeredHandler struct {
	kind    string
	queue   string
	fn      HandlerFunc
	options HandlerOptions
}

func (h *registeredHandler) Kind() string {
	return h.kind
}

func (h *registeredHandler) Queue() string {
	return h.queue
}

func (h *registeredHandler) Handle(ctx *JobContext, job *Job) error {
	return h.fn(ctx, job)
}
