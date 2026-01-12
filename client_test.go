//go:build integration

package rivo_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rabin-a/rivo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) string {
	// Check for DATABASE_URL env var first (for CI or local postgres)
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("rivo_test"),
		postgres.WithUsername("rivo"),
		postgres.WithPassword("rivo"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, container.Terminate(ctx))
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	return connStr
}

func TestClient_NewClient(t *testing.T) {
	connStr := setupTestDB(t)
	ctx := context.Background()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL: connStr,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer client.Close()

	assert.NotEmpty(t, client.WorkerID())
}

func TestClient_Enqueue(t *testing.T) {
	connStr := setupTestDB(t)
	ctx := context.Background()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL: connStr,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer client.Close()

	result, err := client.Enqueue(ctx, rivo.EnqueueParams{
		Kind:    "test-job",
		Payload: map[string]string{"key": "value"},
	})
	require.NoError(t, err)
	assert.NotNil(t, result.Job)
	assert.NotZero(t, result.Job.ID)
	assert.Equal(t, "test-job", result.Job.Kind)
	assert.Equal(t, "default", result.Job.Queue)
	assert.Equal(t, rivo.JobStateAvailable, result.Job.State)
	assert.False(t, result.Duplicate)
}

func TestClient_EnqueueWithPriority(t *testing.T) {
	connStr := setupTestDB(t)
	ctx := context.Background()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL: connStr,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer client.Close()

	result, err := client.Enqueue(ctx, rivo.EnqueueParams{
		Kind:     "priority-job",
		Priority: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int16(10), result.Job.Priority)
}

func TestClient_EnqueueScheduled(t *testing.T) {
	connStr := setupTestDB(t)
	ctx := context.Background()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL: connStr,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer client.Close()

	futureTime := time.Now().Add(1 * time.Hour)
	result, err := client.Enqueue(ctx, rivo.EnqueueParams{
		Kind:       "scheduled-job",
		ScheduleAt: futureTime,
	})
	require.NoError(t, err)
	assert.Equal(t, rivo.JobStateScheduled, result.Job.State)
}

func TestClient_EnqueueIdempotency(t *testing.T) {
	connStr := setupTestDB(t)
	ctx := context.Background()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL: connStr,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// First enqueue
	result1, err := client.Enqueue(ctx, rivo.EnqueueParams{
		Kind:           "idempotent-job",
		IdempotencyKey: "unique-key-123",
	})
	require.NoError(t, err)
	assert.False(t, result1.Duplicate)

	// Second enqueue with same key
	result2, err := client.Enqueue(ctx, rivo.EnqueueParams{
		Kind:           "idempotent-job",
		IdempotencyKey: "unique-key-123",
	})
	require.NoError(t, err)
	assert.True(t, result2.Duplicate)
	assert.Equal(t, result1.Job.ID, result2.Job.ID)
}

func TestClient_GetJob(t *testing.T) {
	connStr := setupTestDB(t)
	ctx := context.Background()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL: connStr,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer client.Close()

	result, err := client.Enqueue(ctx, rivo.EnqueueParams{
		Kind: "get-job-test",
	})
	require.NoError(t, err)

	job, err := client.GetJob(ctx, result.Job.ID)
	require.NoError(t, err)
	assert.Equal(t, result.Job.ID, job.ID)
	assert.Equal(t, "get-job-test", job.Kind)
}

func TestClient_CancelJob(t *testing.T) {
	connStr := setupTestDB(t)
	ctx := context.Background()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL: connStr,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer client.Close()

	result, err := client.Enqueue(ctx, rivo.EnqueueParams{
		Kind: "cancel-test",
	})
	require.NoError(t, err)

	err = client.CancelJob(ctx, result.Job.ID)
	require.NoError(t, err)

	job, err := client.GetJob(ctx, result.Job.ID)
	require.NoError(t, err)
	assert.Equal(t, rivo.JobStateCancelled, job.State)
}

func TestClient_ProcessJob(t *testing.T) {
	connStr := setupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL:  connStr,
		AutoMigrate:  true,
		Workers:      2,
		PollInterval: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	defer client.Close()

	var processed atomic.Bool
	done := make(chan struct{})

	client.RegisterHandler("process-test", func(ctx context.Context, job *rivo.Job) error {
		processed.Store(true)
		close(done)
		return nil
	})

	err = client.Start(ctx)
	require.NoError(t, err)

	_, err = client.Enqueue(ctx, rivo.EnqueueParams{
		Kind: "process-test",
	})
	require.NoError(t, err)

	select {
	case <-done:
		assert.True(t, processed.Load())
	case <-ctx.Done():
		t.Fatal("timeout waiting for job to be processed")
	}
}

func TestClient_ProcessMultipleJobs(t *testing.T) {
	connStr := setupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL:  connStr,
		AutoMigrate:  true,
		Workers:      5,
		PollInterval: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	defer client.Close()

	var count atomic.Int32
	jobCount := 10

	client.RegisterHandler("multi-test", func(ctx context.Context, job *rivo.Job) error {
		count.Add(1)
		return nil
	})

	err = client.Start(ctx)
	require.NoError(t, err)

	for i := 0; i < jobCount; i++ {
		_, err = client.Enqueue(ctx, rivo.EnqueueParams{
			Kind: "multi-test",
		})
		require.NoError(t, err)
	}

	// Wait for all jobs to be processed
	require.Eventually(t, func() bool {
		return count.Load() == int32(jobCount)
	}, 10*time.Second, 100*time.Millisecond)

	assert.Equal(t, int32(jobCount), count.Load())
}

func TestClient_JobPayload(t *testing.T) {
	connStr := setupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL:  connStr,
		AutoMigrate:  true,
		Workers:      1,
		PollInterval: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	defer client.Close()

	type TestPayload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	var receivedPayload TestPayload
	done := make(chan struct{})

	client.RegisterHandler("payload-test", func(ctx context.Context, job *rivo.Job) error {
		err := job.UnmarshalPayload(&receivedPayload)
		close(done)
		return err
	})

	err = client.Start(ctx)
	require.NoError(t, err)

	_, err = client.Enqueue(ctx, rivo.EnqueueParams{
		Kind:    "payload-test",
		Payload: TestPayload{Name: "test", Value: 42},
	})
	require.NoError(t, err)

	select {
	case <-done:
		assert.Equal(t, "test", receivedPayload.Name)
		assert.Equal(t, 42, receivedPayload.Value)
	case <-ctx.Done():
		t.Fatal("timeout waiting for job to be processed")
	}
}
