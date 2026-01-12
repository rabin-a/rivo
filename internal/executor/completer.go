package executor

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/rabin-a/rivo/internal/db"
)

// RetryPolicy defines how to handle job failures.
type RetryPolicy struct {
	MaxAttempts     int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffExponent float64
	Jitter          bool
	RetryableErrors []error
	DiscardErrors   []error
}

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:     25,
		InitialBackoff:  time.Second,
		MaxBackoff:      24 * time.Hour,
		BackoffExponent: 2.0,
		Jitter:          true,
	}
}

// Completer handles job completion and failure.
type Completer struct {
	db          *db.DB
	results     <-chan JobResult
	retryPolicy RetryPolicy

	shutdown chan struct{}
	wg       sync.WaitGroup
}

// CompleterConfig configures the completer.
type CompleterConfig struct {
	DB          *db.DB
	Results     <-chan JobResult
	RetryPolicy RetryPolicy
}

// NewCompleter creates a new completer.
func NewCompleter(cfg CompleterConfig) *Completer {
	if cfg.RetryPolicy.MaxAttempts == 0 {
		cfg.RetryPolicy = DefaultRetryPolicy()
	}

	return &Completer{
		db:          cfg.DB,
		results:     cfg.Results,
		retryPolicy: cfg.RetryPolicy,
		shutdown:    make(chan struct{}),
	}
}

// Start begins processing job results.
func (c *Completer) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.run(ctx)
}

// Stop signals the completer to stop.
func (c *Completer) Stop() {
	close(c.shutdown)
}

// Wait waits for the completer to finish.
func (c *Completer) Wait() {
	c.wg.Wait()
}

func (c *Completer) run(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case result, ok := <-c.results:
			if !ok {
				return
			}
			c.handleResult(ctx, result)
		}
	}
}

func (c *Completer) handleResult(ctx context.Context, result JobResult) {
	if result.Error == nil {
		// Job completed successfully
		if err := c.db.CompleteJob(ctx, result.Job.ID); err != nil {
			// Log error in production
		}
		return
	}

	// Job failed - determine retry or discard
	retryAt := c.shouldRetry(result.Job, result.Error)

	errMsg := result.Error.Error()
	if err := c.db.FailJob(ctx, result.Job.ID, errMsg, retryAt); err != nil {
		// Log error in production
	}
}

func (c *Completer) shouldRetry(job *db.JobRow, err error) *time.Time {
	// Check if error should cause immediate discard
	for _, discardErr := range c.retryPolicy.DiscardErrors {
		if errors.Is(err, discardErr) {
			return nil
		}
	}

	// Check if we've exceeded max attempts
	if int(job.Attempt) >= c.retryPolicy.MaxAttempts {
		return nil
	}

	// Check if only specific errors are retryable
	if len(c.retryPolicy.RetryableErrors) > 0 {
		isRetryable := false
		for _, retryErr := range c.retryPolicy.RetryableErrors {
			if errors.Is(err, retryErr) {
				isRetryable = true
				break
			}
		}
		if !isRetryable {
			return nil
		}
	}

	// Calculate backoff delay
	delay := c.calculateBackoff(int(job.Attempt))
	retryAt := time.Now().Add(delay)
	return &retryAt
}

func (c *Completer) calculateBackoff(attempt int) time.Duration {
	delay := c.retryPolicy.InitialBackoff

	// Exponential backoff
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * c.retryPolicy.BackoffExponent)
		if delay > c.retryPolicy.MaxBackoff {
			delay = c.retryPolicy.MaxBackoff
			break
		}
	}

	// Add jitter
	if c.retryPolicy.Jitter {
		jitter := time.Duration(rand.Float64() * 0.3 * float64(delay))
		delay += jitter
	}

	return delay
}
