<p align="center">
  <img src="https://raw.githubusercontent.com/rabin-a/rivo/main/logo.svg" alt="Rivo" width="200"/>
</p>

<h1 align="center">Rivo</h1>

<p align="center">
  Postgres-native workflow and background job execution platform for Go
</p>

<p align="center">
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#workflows">Workflows</a> •
  <a href="#api">API</a>
</p>

---

## Features

| Feature | Description |
|---------|-------------|
| **Postgres-native** | No Redis or Kafka needed, just Postgres |
| **Durable jobs** | At-least-once execution with SKIP LOCKED |
| **Workflows** | DAG with parallel, conditional, fan-out steps |
| **Map/Reduce** | Process items in parallel with aggregation |
| **Step rerun** | Rerun failed workflow steps from any point |
| **Dashboard** | React UI for monitoring jobs and workflows |

## Installation

```bash
go get github.com/rabin-a/rivo
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "github.com/rabin-a/rivo"
)

func main() {
    ctx := context.Background()

    client, _ := rivo.NewClient(ctx, rivo.Config{
        DatabaseURL: "postgres://localhost:5432/rivo?sslmode=disable",
        AutoMigrate: true,
    })
    defer client.Close()

    // Register handler
    client.RegisterHandler("send-email", func(ctx context.Context, job *rivo.Job) error {
        log.Printf("Processing job %d", job.ID)
        return nil
    })

    // Start workers
    client.Start(ctx)

    // Enqueue job
    client.Enqueue(ctx, rivo.EnqueueParams{
        Kind:    "send-email",
        Payload: map[string]string{"to": "user@example.com"},
    })

    select {}
}
```

## Interface

```go
type Client interface {
    RegisterHandler(kind string, fn HandlerFunc, opts ...HandlerOptions)
    Enqueue(ctx context.Context, params EnqueueParams) (*Job, error)
    Start(ctx context.Context) error
    Close() error

    // Workflows
    RegisterWorkflow(w *Workflow)
    StartWorkflow(ctx context.Context, name string, input any) (*WorkflowRun, error)
    RerunWorkflowStep(ctx context.Context, runID int64, stepID string) error
}

type Workflow interface {
    Step(id string, fn WorkflowFunc) *Workflow
    Parallel(steps ...ParallelStep) *Workflow
    If(cond func(*WorkflowContext) bool, fn WorkflowFunc) *Workflow
    HandlerStep(handlerID, stepID string) *Workflow
    Map(id string, items ItemsFunc, mapper MapFunc) *Workflow
    Reduce(id, sourceStepID string, reducer ReduceFunc) *Workflow
}
```

## Jobs

```go
// Priority
client.Enqueue(ctx, rivo.EnqueueParams{
    Kind: "task", Queue: "critical", Priority: 10,
})

// Delayed
client.Enqueue(ctx, rivo.EnqueueParams{
    Kind: "reminder", ScheduleAt: time.Now().Add(24 * time.Hour),
})

// Idempotent
client.Enqueue(ctx, rivo.EnqueueParams{
    Kind: "process-order", IdempotencyKey: "order-123",
})

// Retry policy
client.RegisterHandler("flaky-api", handler, rivo.HandlerOptions{
    Retry: rivo.RetryPolicy{
        MaxAttempts: 5,
        Backoff:     rivo.ExponentialBackoff(time.Second, time.Minute),
    },
})
```

## Workflows

```go
workflow := rivo.NewWorkflow("order-processing").
    Step("validate", validateOrder).
    Step("charge", chargePayment).
    Parallel(
        rivo.NewWorkflow("").Step("notify-user", sendEmail),
        rivo.NewWorkflow("").Step("notify-warehouse", sendWebhook),
    ).
    Step("complete", markComplete).
    Build()

client.RegisterWorkflow(workflow)
client.StartWorkflow(ctx, "order-processing", orderData)
```

### HandlerStep

Execute registered job handlers within workflows:

```go
workflow := rivo.NewWorkflow("user-onboarding").
    Step("create-user", createUser).
    HandlerStep("send-email", "send-welcome-email").
    HandlerStep("send-email", "send-verification-email").
    Build()
```

### Map / Reduce

Process items in parallel, then aggregate:

```go
workflow := rivo.NewWorkflow("calculate-stats").
    Map("double-values",
        func(ctx *rivo.WorkflowContext) ([]any, error) {
            return []any{1, 2, 3, 4, 5}, nil
        },
        func(ctx *rivo.WorkflowContext, item any, idx int) (any, error) {
            return item.(int) * 2, nil  // [2, 4, 6, 8, 10]
        },
    ).
    Reduce("sum-all", "double-values",
        func(ctx *rivo.WorkflowContext, results []any) (any, error) {
            sum := 0
            for _, r := range results { sum += r.(int) }
            return sum, nil  // 30
        },
    ).Build()
```

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/jobs` | List jobs |
| POST | `/api/v1/jobs` | Enqueue job |
| POST | `/api/v1/jobs/:id/retry` | Retry failed job |
| GET | `/api/v1/workflows/runs` | List workflow runs |
| POST | `/api/v1/workflows/runs/:id/steps/:stepId/rerun` | Rerun step |

## Running

```bash
# Start Postgres
make db-start

# Run with UI
make ui-build && make example-fullstack

# Open http://localhost:8080
```

## Requirements

- Go 1.21+
- PostgreSQL 14+

## License

MIT
