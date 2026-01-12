package rivo

import (
	"testing"
	"time"
)

func TestFixedBackoff(t *testing.T) {
	backoff := FixedBackoff(5 * time.Second)

	for i := 1; i <= 5; i++ {
		delay := backoff(i)
		if delay != 5*time.Second {
			t.Errorf("attempt %d: expected 5s, got %v", i, delay)
		}
	}
}

func TestExponentialBackoff(t *testing.T) {
	backoff := ExponentialBackoff(time.Second, time.Minute)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 32 * time.Second},
		{7, time.Minute}, // capped at max
		{8, time.Minute}, // capped at max
	}

	for _, tt := range tests {
		delay := backoff(tt.attempt)
		if delay != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, delay)
		}
	}
}

func TestLinearBackoff(t *testing.T) {
	backoff := LinearBackoff(time.Second, 2*time.Second)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 3 * time.Second},
		{3, 5 * time.Second},
		{4, 7 * time.Second},
	}

	for _, tt := range tests {
		delay := backoff(tt.attempt)
		if delay != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, delay)
		}
	}
}
