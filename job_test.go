package rivo

import (
	"encoding/json"
	"testing"
)

func TestJob_UnmarshalPayload(t *testing.T) {
	type Payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	job := &Job{
		Payload: json.RawMessage(`{"name":"test","value":42}`),
	}

	var payload Payload
	err := job.UnmarshalPayload(&payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.Name != "test" {
		t.Errorf("expected name 'test', got %q", payload.Name)
	}
	if payload.Value != 42 {
		t.Errorf("expected value 42, got %d", payload.Value)
	}
}

func TestJob_IsTerminal(t *testing.T) {
	tests := []struct {
		state    JobState
		expected bool
	}{
		{JobStateAvailable, false},
		{JobStateScheduled, false},
		{JobStateRunning, false},
		{JobStateRetryable, false},
		{JobStateCompleted, true},
		{JobStateCancelled, true},
		{JobStateDiscarded, true},
	}

	for _, tt := range tests {
		job := &Job{State: tt.state}
		if got := job.IsTerminal(); got != tt.expected {
			t.Errorf("state %s: expected %v, got %v", tt.state, tt.expected, got)
		}
	}
}
