package retry

import (
	"errors"
	"math/rand"
	"time"
)

// Policy defines retry behavior for failed jobs.
type Policy struct {
	// MaxAttempts is the maximum number of attempts (including first).
	MaxAttempts int

	// InitialBackoff is the delay before the first retry.
	InitialBackoff time.Duration

	// MaxBackoff caps the maximum delay between retries.
	MaxBackoff time.Duration

	// BackoffMultiplier is the factor by which backoff increases each retry.
	BackoffMultiplier float64

	// Jitter adds randomness to backoff delays to prevent thundering herd.
	Jitter bool

	// RetryableErrors limits retries to only these error types.
	// If nil, all errors are retryable.
	RetryableErrors []error

	// DiscardErrors causes immediate job discard on these errors.
	DiscardErrors []error
}

// DefaultPolicy returns a sensible default retry policy.
func DefaultPolicy() Policy {
	return Policy{
		MaxAttempts:       25,
		InitialBackoff:    time.Second,
		MaxBackoff:        24 * time.Hour,
		BackoffMultiplier: 2.0,
		Jitter:            true,
	}
}

// NextRetryAt calculates when the next retry should occur.
// Returns the retry time and whether a retry should happen.
func (p *Policy) NextRetryAt(attempt int, err error) (time.Time, bool) {
	// Check if error should cause immediate discard
	for _, discardErr := range p.DiscardErrors {
		if errors.Is(err, discardErr) {
			return time.Time{}, false
		}
	}

	// Check if we've exceeded max attempts
	if attempt >= p.MaxAttempts {
		return time.Time{}, false
	}

	// Check if only specific errors are retryable
	if len(p.RetryableErrors) > 0 {
		isRetryable := false
		for _, retryErr := range p.RetryableErrors {
			if errors.Is(err, retryErr) {
				isRetryable = true
				break
			}
		}
		if !isRetryable {
			return time.Time{}, false
		}
	}

	delay := p.calculateBackoff(attempt)
	return time.Now().Add(delay), true
}

func (p *Policy) calculateBackoff(attempt int) time.Duration {
	delay := p.InitialBackoff

	// Exponential backoff
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * p.BackoffMultiplier)
		if delay > p.MaxBackoff {
			delay = p.MaxBackoff
			break
		}
	}

	// Add jitter (up to 30% of delay)
	if p.Jitter && delay > 0 {
		jitter := time.Duration(rand.Float64() * 0.3 * float64(delay))
		delay += jitter
	}

	return delay
}

// ShouldRetry returns true if the error should be retried.
func (p *Policy) ShouldRetry(attempt int, err error) bool {
	_, shouldRetry := p.NextRetryAt(attempt, err)
	return shouldRetry
}
