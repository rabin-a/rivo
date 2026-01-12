# Rivo

A modern, Postgres-native workflow and background job execution platform for Go.

## Features

- **Postgres-native** - No Redis, Kafka, or external dependencies required
- **Library-first** - Embed directly into your Go applications
- **Durable jobs** - At-least-once execution with optional idempotency
- **Workflow engine** - Build complex workflows with fan-out, conditionals, and sub-workflows
- **High concurrency** - Safe concurrent workers with row-level locking
- **Dashboard UI** - React-based UI for monitoring and managing jobs

## Project Structure

```
rivo/
├── *.go                    # Go library (import as github.com/rabin-a/rivo)
├── internal/               # Internal packages
│   ├── db/                 # Database layer
│   ├── executor/           # Job execution engine
│   ├── migration/          # SQL migrations
│   └── retry/              # Retry policies
├── api/http/               # HTTP API server
├── ui/                     # React dashboard (Vite + TypeScript)
└── _examples/
    ├── basic/              # Simple job processing example
    └── fullstack/          # Full platform with UI
```

## Installation

```bash
go get github.com/rabin-a/rivo
```

## Quick Start

### As a Library

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/rabin-a/rivo"
)

func main() {
    ctx := context.Background()

    // Create client
    client, err := rivo.NewClient(ctx, rivo.Config{
        DatabaseURL: "postgres://localhost:5432/rivo?sslmode=disable",
        AutoMigrate: true,
        Workers:     10,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Register handler
    client.RegisterHandler("send-email", func(ctx context.Context, job *rivo.Job) error {
        var payload struct {
            To      string `json:"to"`
            Subject string `json:"subject"`
        }
        job.UnmarshalPayload(&payload)
        log.Printf("Sending email to %s: %s", payload.To, payload.Subject)
        return nil
    })

    // Start processing
    client.Start(ctx)

    // Enqueue jobs
    client.Enqueue(ctx, rivo.EnqueueParams{
        Kind:    "send-email",
        Payload: map[string]string{"to": "user@example.com", "subject": "Hello!"},
    })

    // Wait...
    select {}
}
```

### Full Platform with UI

```bash
# Start Postgres
make db-start

# Install UI dependencies
make ui-install

# Build UI
make ui-build

# Run fullstack example
make example-fullstack

# Open http://localhost:8080
```

## Development

```bash
# Run API server (port 8080)
make example-fullstack

# In another terminal, run UI dev server (port 3000)
make ui-dev

# Open http://localhost:3000
```

## Job Features

### Priority Queues

```go
client.Enqueue(ctx, rivo.EnqueueParams{
    Kind:     "urgent-task",
    Queue:    "critical",
    Priority: 10,
})
```

### Delayed Jobs

```go
client.Enqueue(ctx, rivo.EnqueueParams{
    Kind:       "reminder",
    ScheduleAt: time.Now().Add(24 * time.Hour),
})
```

### Idempotency

```go
// Second call returns the same job (no duplicate)
client.Enqueue(ctx, rivo.EnqueueParams{
    Kind:           "process-order",
    IdempotencyKey: "order-123",
})
```

### Retry Policies

```go
client.RegisterHandler("flaky-api", handler, rivo.HandlerOptions{
    Retry: rivo.RetryPolicy{
        MaxAttempts: 5,
        Backoff:     rivo.ExponentialBackoff(time.Second, time.Minute),
        Jitter:      true,
    },
})
```

## Workflows (Pro)

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

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/stats` | Job statistics |
| GET | `/api/v1/jobs` | List jobs |
| GET | `/api/v1/jobs/:id` | Get job |
| POST | `/api/v1/jobs` | Enqueue job |
| POST | `/api/v1/jobs/:id/cancel` | Cancel job |
| POST | `/api/v1/jobs/:id/retry` | Retry job |

## Configuration

```go
rivo.Config{
    // Database
    DatabaseURL: "postgres://localhost:5432/rivo",
    MaxConns:    25,

    // Workers
    Workers:      10,
    PollInterval: 100 * time.Millisecond,
    JobTimeout:   5 * time.Minute,

    // Queues
    Queues: []rivo.QueueConfig{
        {Name: "critical", Priority: 10},
        {Name: "default", Priority: 1},
    },

    // Options
    Namespace:   "default",
    AutoMigrate: true,
}
```

## UI Dashboard

The dashboard provides:

- **Dashboard** - Job statistics and queue depths
- **Jobs** - List, filter, cancel, and retry jobs
- **Workflows** - Visual workflow monitoring (Pro)
- **Schedules** - Cron job management

## Makefile Commands

```bash
make build              # Build Go library
make test               # Run unit tests
make test-integration   # Run integration tests (Docker required)

make ui-install         # Install UI dependencies
make ui-dev             # Run UI dev server
make ui-build           # Build UI for production

make example-basic      # Run basic example
make example-fullstack  # Run fullstack platform

make db-start           # Start Postgres with Docker
make db-stop            # Stop Postgres

make clean              # Clean build artifacts
```

## Requirements

- Go 1.21+
- PostgreSQL 14+
- Node.js 18+ (for UI development)

## License

MIT
