package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rabin-a/rivo"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configuration
	dbURL := getEnv("DATABASE_URL", "postgres://localhost:5432/rivo?sslmode=disable")
	port := getEnv("PORT", "8080")
	workers := getEnvInt("WORKERS", 10)

	log.Printf("Starting Rivo server on port %s", port)

	// Create Rivo client
	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL:  dbURL,
		AutoMigrate:  true,
		Workers:      workers,
		PollInterval: 100 * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Register example job handlers
	registerHandlers(client)

	// Start job processing
	if err := client.Start(ctx); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}
	log.Println("Job processing started")

	// Create HTTP server
	mux := http.NewServeMux()

	// API routes
	api := &APIHandler{client: client}
	mux.HandleFunc("GET /api/v1/stats", api.handleStats)
	mux.HandleFunc("GET /api/v1/jobs", api.handleListJobs)
	mux.HandleFunc("GET /api/v1/jobs/{id}", api.handleGetJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}/logs", api.handleGetJobLogs)
	mux.HandleFunc("POST /api/v1/jobs", api.handleEnqueueJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/cancel", api.handleCancelJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/retry", api.handleRetryJob)
	mux.HandleFunc("GET /api/v1/health", api.handleHealth)

	// Worker routes
	mux.HandleFunc("GET /api/v1/workers", api.handleListWorkers)
	mux.HandleFunc("GET /api/v1/workers/health", api.handleWorkersHealth)

	// Queue routes
	mux.HandleFunc("GET /api/v1/queues", api.handleListQueues)
	mux.HandleFunc("GET /api/v1/queues/stats", api.handleQueueStats)
	mux.HandleFunc("PUT /api/v1/queues/{name}", api.handleUpdateQueue)
	mux.HandleFunc("POST /api/v1/queues/{name}/pause", api.handlePauseQueue)
	mux.HandleFunc("POST /api/v1/queues/{name}/resume", api.handleResumeQueue)

	// Handler routes
	mux.HandleFunc("GET /api/v1/handlers", api.handleListHandlers)
	mux.HandleFunc("POST /api/v1/handlers/{kind}/run", api.handleRunHandler)

	// Workflow routes
	mux.HandleFunc("GET /api/v1/workflows", api.handleListWorkflows)
	mux.HandleFunc("POST /api/v1/workflows/{name}/start", api.handleStartWorkflow)
	mux.HandleFunc("GET /api/v1/workflow-runs", api.handleListWorkflowRuns)
	mux.HandleFunc("GET /api/v1/workflow-runs/{id}", api.handleGetWorkflowRun)
	mux.HandleFunc("POST /api/v1/workflow-runs/{id}/cancel", api.handleCancelWorkflowRun)
	mux.HandleFunc("GET /api/v1/workflow-runs/{id}/steps", api.handleGetWorkflowStepLogs)
	mux.HandleFunc("POST /api/v1/workflow-runs/{id}/steps/{stepId}/rerun", api.handleRerunWorkflowStep)

	// Serve a simple index page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Rivo API</title></head>
<body>
<h1>Rivo API Server</h1>
<p>API is running. Available endpoints:</p>
<ul>
<li><a href="/api/v1/health">/api/v1/health</a> - Health check</li>
<li><a href="/api/v1/stats">/api/v1/stats</a> - Job statistics</li>
<li><a href="/api/v1/jobs">/api/v1/jobs</a> - List jobs</li>
</ul>
<p>Run <code>cd ui && npm install && npm run dev</code> for the full dashboard.</p>
</body>
</html>`))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: corsMiddleware(mux),
	}

	// Start server
	go func() {
		log.Printf("Server listening on http://localhost:%s", port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	server.Shutdown(shutdownCtx)
	client.Shutdown(shutdownCtx)
	log.Println("Goodbye!")
}

func registerHandlers(client *rivo.Client) {
	// Email handler
	client.RegisterHandler("send-email", func(ctx *rivo.JobContext, job *rivo.Job) error {
		ctx.Logger().Info("Starting send-email job %d", job.ID)

		var payload struct {
			To      string `json:"to"`
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		if err := job.UnmarshalPayload(&payload); err != nil {
			ctx.Logger().Error("Failed to unmarshal payload: %v", err)
			return err
		}

		ctx.Logger().Info("Sending email to %s: %s", payload.To, payload.Subject)
		time.Sleep(500 * time.Millisecond) // Simulate work
		ctx.Logger().Info("Email sent successfully")
		return nil
	})

	// Generic processing handler
	client.RegisterHandler("process", func(ctx *rivo.JobContext, job *rivo.Job) error {
		ctx.Logger().Info("Processing job %d", job.ID)
		ctx.Logger().Debug("Payload: %s", string(job.Payload))
		time.Sleep(200 * time.Millisecond)
		ctx.Logger().Info("Processing complete")
		return nil
	})

	// Failing handler for testing retries
	client.RegisterHandler("flaky", func(ctx *rivo.JobContext, job *rivo.Job) error {
		ctx.Logger().Info("Attempt %d of flaky job", job.Attempt)
		if job.Attempt < 3 {
			ctx.Logger().Warn("Simulating failure on attempt %d", job.Attempt)
			return fmt.Errorf("simulated failure")
		}
		ctx.Logger().Info("Success on attempt %d", job.Attempt)
		return nil
	}, rivo.HandlerOptions{
		Retry: rivo.RetryPolicy{
			MaxAttempts: 5,
			Backoff:     rivo.FixedBackoff(time.Second),
		},
	})

	// Register example workflow: Order Processing
	registerWorkflows(client)
}

func registerWorkflows(client *rivo.Client) {
	// Order Processing Workflow
	orderWorkflow := rivo.NewWorkflow("order-processing").
		Description("Process a customer order through validation, payment, and shipping").
		Step("validate-order", func(ctx *rivo.WorkflowContext) error {
			ctx.Logger().Info("Validating order...")

			var input struct {
				OrderID  string `json:"order_id"`
				Customer string `json:"customer"`
				Amount   float64 `json:"amount"`
			}
			if err := ctx.Input(&input); err != nil {
				return fmt.Errorf("invalid input: %w", err)
			}

			ctx.Logger().Info("Order %s validated for customer %s", input.OrderID, input.Customer)
			time.Sleep(100 * time.Millisecond)

			return ctx.SetOutput(map[string]any{
				"order_id":  input.OrderID,
				"validated": true,
			})
		}).
		Step("process-payment", func(ctx *rivo.WorkflowContext) error {
			ctx.Logger().Info("Processing payment...")

			var input struct {
				Amount float64 `json:"amount"`
			}
			ctx.Input(&input)

			time.Sleep(200 * time.Millisecond)
			ctx.Logger().Info("Payment of $%.2f processed", input.Amount)

			return ctx.SetOutput(map[string]any{
				"payment_id": fmt.Sprintf("PAY-%d", time.Now().UnixNano()),
				"status":     "completed",
			})
		}).
		Parallel(
			rivo.ParallelStep{
				ID:   "send-confirmation",
				Name: "Send Confirmation Email",
				Handler: func(ctx *rivo.WorkflowContext) error {
					ctx.Logger().Info("Sending confirmation email...")
					time.Sleep(100 * time.Millisecond)
					return ctx.SetOutput(map[string]any{"email_sent": true})
				},
			},
			rivo.ParallelStep{
				ID:   "update-inventory",
				Name: "Update Inventory",
				Handler: func(ctx *rivo.WorkflowContext) error {
					ctx.Logger().Info("Updating inventory...")
					time.Sleep(150 * time.Millisecond)
					return ctx.SetOutput(map[string]any{"inventory_updated": true})
				},
			},
		).
		Step("ship-order", func(ctx *rivo.WorkflowContext) error {
			ctx.Logger().Info("Shipping order...")
			time.Sleep(100 * time.Millisecond)

			return ctx.SetOutput(map[string]any{
				"tracking_number": fmt.Sprintf("TRACK-%d", time.Now().UnixNano()),
				"shipped":         true,
			})
		})

	client.RegisterWorkflow(orderWorkflow)

	// Simple Data Pipeline Workflow
	pipelineWorkflow := rivo.NewWorkflow("data-pipeline").
		Description("Extract, transform, and load data").
		Step("extract", func(ctx *rivo.WorkflowContext) error {
			ctx.Logger().Info("Extracting data...")
			time.Sleep(200 * time.Millisecond)
			return ctx.SetOutput(map[string]any{"records": 1000})
		}).
		Step("transform", func(ctx *rivo.WorkflowContext) error {
			ctx.Logger().Info("Transforming data...")

			var extractOutput struct {
				Records int `json:"records"`
			}
			ctx.GetStepOutput("extract", &extractOutput)

			time.Sleep(300 * time.Millisecond)
			ctx.Logger().Info("Transformed %d records", extractOutput.Records)

			return ctx.SetOutput(map[string]any{
				"records":     extractOutput.Records,
				"transformed": true,
			})
		}).
		Step("load", func(ctx *rivo.WorkflowContext) error {
			ctx.Logger().Info("Loading data...")
			time.Sleep(150 * time.Millisecond)
			return ctx.SetOutput(map[string]any{"loaded": true})
		})

	client.RegisterWorkflow(pipelineWorkflow)

	// Batch Processing Workflow - demonstrates fan-out/fan-in pattern
	batchWorkflow := rivo.NewWorkflow("batch-processing").
		Description("Process multiple items in parallel with aggregation").
		Step("get-items", func(ctx *rivo.WorkflowContext) error {
			ctx.Logger().Info("Getting items to process...")
			time.Sleep(100 * time.Millisecond)

			// Simulate getting a list of item IDs
			items := []string{"item-1", "item-2", "item-3", "item-4", "item-5"}
			ctx.Logger().Info("Found %d items to process", len(items))

			return ctx.SetOutput(map[string]any{
				"item_ids": items,
			})
		}).
		ForEach(
			"process-items",
			// Items function: get items from previous step
			func(ctx *rivo.WorkflowContext) ([]any, error) {
				var prevOutput struct {
					ItemIDs []string `json:"item_ids"`
				}
				if err := ctx.GetStepOutput("get-items", &prevOutput); err != nil {
					return nil, err
				}

				// Convert to []any for processing
				items := make([]any, len(prevOutput.ItemIDs))
				for i, id := range prevOutput.ItemIDs {
					items[i] = id
				}
				return items, nil
			},
			// Map function: process each item
			func(ctx *rivo.WorkflowContext, item any, index int) (any, error) {
				itemID := item.(string)
				ctx.Logger().Info("Processing item %d: %s", index, itemID)

				// Simulate processing
				time.Sleep(50 * time.Millisecond)

				return map[string]any{
					"item_id":   itemID,
					"processed": true,
					"result":    fmt.Sprintf("Result for %s", itemID),
				}, nil
			},
			// Reducer: aggregate results
			rivo.WithReducer(func(ctx *rivo.WorkflowContext, results []any) (any, error) {
				ctx.Logger().Info("Aggregating %d results", len(results))

				successful := 0
				for _, r := range results {
					if res, ok := r.(map[string]any); ok {
						if processed, _ := res["processed"].(bool); processed {
							successful++
						}
					}
				}

				return map[string]any{
					"total_processed": len(results),
					"successful":      successful,
					"results":         results,
				}, nil
			}),
			rivo.WithMaxConcurrency(3), // Process max 3 items at a time
		).
		Step("finalize", func(ctx *rivo.WorkflowContext) error {
			ctx.Logger().Info("Finalizing batch processing...")

			var processResults struct {
				TotalProcessed int `json:"total_processed"`
				Successful     int `json:"successful"`
			}
			ctx.GetStepOutput("process-items", &processResults)

			ctx.Logger().Info("Batch complete: %d/%d successful", processResults.Successful, processResults.TotalProcessed)

			return ctx.SetOutput(map[string]any{
				"status":  "complete",
				"summary": fmt.Sprintf("Processed %d items with %d successful", processResults.TotalProcessed, processResults.Successful),
			})
		})

	client.RegisterWorkflow(batchWorkflow)
}

// APIHandler handles API requests
type APIHandler struct {
	client *rivo.Client
}

func (h *APIHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	queueStats, err := h.client.DB().GetQueueStats(r.Context(), h.client.Namespace())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var total, available, running, completed, failed int64
	queues := make([]map[string]any, len(queueStats))
	for i, s := range queueStats {
		total += s.Total
		available += s.Available
		running += s.Running
		completed += s.Completed
		failed += s.Failed

		queues[i] = map[string]any{
			"name":      s.Queue,
			"available": s.Available,
			"running":   s.Running,
			"completed": s.Completed,
			"retryable": s.Retryable,
			"failed":    s.Failed,
			"total":     s.Total,
		}
	}

	stats := map[string]any{
		"total":     total,
		"available": available,
		"running":   running,
		"completed": completed,
		"failed":    failed,
		"queues":    queues,
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *APIHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	queue := r.URL.Query().Get("queue")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	jobs, err := h.client.DB().ListJobs(r.Context(), h.client.Namespace(), state, queue, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(jobs))
	for i, j := range jobs {
		result[i] = map[string]any{
			"id":           j.ID,
			"kind":         j.Kind,
			"queue":        j.Queue,
			"state":        j.State,
			"payload":      j.Payload,
			"priority":     j.Priority,
			"attempt":      j.Attempt,
			"max_attempts": j.MaxAttempts,
			"scheduled_at": j.ScheduledAt,
			"attempted_at": j.AttemptedAt,
			"completed_at": j.CompletedAt,
			"created_at":   j.CreatedAt,
			"updated_at":   j.UpdatedAt,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.client.GetJob(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	writeJSON(w, http.StatusOK, jobToJSON(job))
}

func (h *APIHandler) handleEnqueueJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind           string          `json:"kind"`
		Payload        json.RawMessage `json:"payload"`
		Queue          string          `json:"queue"`
		Priority       int16           `json:"priority"`
		ScheduleAt     *time.Time      `json:"schedule_at"`
		IdempotencyKey string          `json:"idempotency_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := rivo.EnqueueParams{
		Kind:           req.Kind,
		Queue:          req.Queue,
		Priority:       req.Priority,
		IdempotencyKey: req.IdempotencyKey,
	}

	if req.Payload != nil {
		params.Payload = req.Payload
	}
	if req.ScheduleAt != nil {
		params.ScheduleAt = *req.ScheduleAt
	}

	result, err := h.client.Enqueue(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	status := http.StatusCreated
	if result.Duplicate {
		status = http.StatusOK
	}

	writeJSON(w, status, jobToJSON(result.Job))
}

func (h *APIHandler) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := h.client.CancelJob(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *APIHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	logs, err := h.client.DB().GetJobLogs(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(logs))
	for i, l := range logs {
		result[i] = map[string]any{
			"id":           l.ID,
			"job_id":       l.JobID,
			"attempt":      l.Attempt,
			"worker_id":    l.WorkerID,
			"started_at":   l.StartedAt,
			"completed_at": l.CompletedAt,
			"duration_ms":  l.DurationMs,
			"input":        l.Input,
			"output":       l.Output,
			"status":       l.Status,
			"error":        l.Error,
			"error_stack":  l.ErrorStack,
			"logs":         l.Logs,
			"created_at":   l.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	workers, err := h.client.DB().GetWorkers(r.Context(), h.client.Namespace())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(workers))
	for i, w := range workers {
		result[i] = map[string]any{
			"id":             w.ID,
			"namespace":      w.Namespace,
			"queues":         w.Queues,
			"concurrency":    w.Concurrency,
			"status":         w.Status,
			"started_at":     w.StartedAt,
			"last_heartbeat": w.LastHeartbeat,
			"jobs_processed": w.JobsProcessed,
			"jobs_failed":    w.JobsFailed,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleWorkersHealth(w http.ResponseWriter, r *http.Request) {
	health, err := h.client.DB().GetWorkerHealth(r.Context(), h.client.Namespace())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(health))
	for i, h := range health {
		result[i] = map[string]any{
			"id":             h.ID,
			"status":         h.Status,
			"is_healthy":     h.IsHealthy,
			"last_heartbeat": h.LastHeartbeat,
			"heartbeat_age":  h.HeartbeatAge.String(),
			"jobs_processed": h.JobsProcessed,
			"jobs_failed":    h.JobsFailed,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleListQueues(w http.ResponseWriter, r *http.Request) {
	queues, err := h.client.DB().GetQueues(r.Context(), h.client.Namespace())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(queues))
	for i, q := range queues {
		result[i] = map[string]any{
			"name":        q.Name,
			"namespace":   q.Namespace,
			"concurrency": q.Concurrency,
			"paused":      q.Paused,
			"priority":    q.Priority,
			"created_at":  q.CreatedAt,
			"updated_at":  q.UpdatedAt,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleQueueStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.client.DB().GetQueueStats(r.Context(), h.client.Namespace())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(stats))
	for i, s := range stats {
		result[i] = map[string]any{
			"namespace": s.Namespace,
			"queue":     s.Queue,
			"available": s.Available,
			"running":   s.Running,
			"completed": s.Completed,
			"retryable": s.Retryable,
			"failed":    s.Failed,
			"total":     s.Total,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleUpdateQueue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		Concurrency int  `json:"concurrency"`
		Priority    int  `json:"priority"`
		Paused      bool `json:"paused"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dbClient := h.client.DB()
	queueConfig := dbClient.NewQueueConfig(name, h.client.Namespace(), req.Concurrency, req.Paused, req.Priority)
	err := dbClient.UpsertQueue(r.Context(), queueConfig)

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handlePauseQueue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if err := h.client.DB().PauseQueue(r.Context(), h.client.Namespace(), name, true); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (h *APIHandler) handleResumeQueue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if err := h.client.DB().PauseQueue(r.Context(), h.client.Namespace(), name, false); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

func (h *APIHandler) handleListHandlers(w http.ResponseWriter, r *http.Request) {
	handlers := h.client.RegisteredHandlers()

	result := make([]map[string]any, len(handlers))
	for i, handler := range handlers {
		result[i] = map[string]any{
			"kind":         handler.Kind,
			"queue":        handler.Queue,
			"concurrency":  handler.Concurrency,
			"max_attempts": handler.MaxAttempts,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleRunHandler(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")

	// Find the handler to get its queue
	handlers := h.client.RegisteredHandlers()
	var queue string
	for _, handler := range handlers {
		if handler.Kind == kind {
			queue = handler.Queue
			break
		}
	}

	if queue == "" {
		writeError(w, http.StatusNotFound, "handler not found")
		return
	}

	// Parse optional payload
	var payload json.RawMessage
	if r.ContentLength > 0 {
		var req struct {
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Payload != nil {
			payload = req.Payload
		}
	}

	if payload == nil {
		payload = json.RawMessage(`{}`)
	}

	// Enqueue the job
	result, err := h.client.Enqueue(r.Context(), rivo.EnqueueParams{
		Kind:    kind,
		Queue:   queue,
		Payload: payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"job_id":  result.Job.ID,
		"kind":    result.Job.Kind,
		"queue":   result.Job.Queue,
		"state":   result.Job.State,
		"message": "Job enqueued successfully",
	})
}

func (h *APIHandler) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows := h.client.RegisteredWorkflows()
	writeJSON(w, http.StatusOK, workflows)
}

func (h *APIHandler) handleStartWorkflow(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Parse optional input
	var input any
	if r.ContentLength > 0 {
		var req struct {
			Input json.RawMessage `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Input != nil {
			input = req.Input
		}
	}

	if input == nil {
		input = map[string]any{}
	}

	result, err := h.client.StartWorkflow(r.Context(), rivo.StartWorkflowParams{
		WorkflowName: name,
		Input:        input,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, workflowRunToJSON(result.Run))
}

func (h *APIHandler) handleListWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	runs, err := h.client.ListWorkflowRuns(r.Context(), state, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(runs))
	for i, run := range runs {
		result[i] = workflowRunToJSON(run)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleGetWorkflowRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow run ID")
		return
	}

	run, err := h.client.GetWorkflowRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "workflow run not found")
		return
	}

	writeJSON(w, http.StatusOK, workflowRunToJSON(run))
}

func (h *APIHandler) handleCancelWorkflowRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow run ID")
		return
	}

	if err := h.client.CancelWorkflowRun(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) handleGetWorkflowStepLogs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow run ID")
		return
	}

	logs, err := h.client.GetWorkflowStepLogs(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(logs))
	for i, l := range logs {
		result[i] = map[string]any{
			"id":              l.ID,
			"workflow_run_id": l.WorkflowRunID,
			"step_id":         l.StepID,
			"attempt":         l.Attempt,
			"worker_id":       l.WorkerID,
			"started_at":      l.StartedAt,
			"completed_at":    l.CompletedAt,
			"duration_ms":     l.DurationMs,
			"input":           l.Input,
			"output":          l.Output,
			"status":          l.Status,
			"error":           l.Error,
			"error_stack":     l.ErrorStack,
			"logs":            l.Logs,
			"created_at":      l.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleRerunWorkflowStep(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workflow run ID")
		return
	}

	stepID := r.PathValue("stepId")
	if stepID == "" {
		writeError(w, http.StatusBadRequest, "step ID is required")
		return
	}

	if err := h.client.RerunWorkflowStep(r.Context(), id, stepID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "rerun started"})
}

func workflowRunToJSON(run *rivo.WorkflowRun) map[string]any {
	return map[string]any{
		"id":            run.ID,
		"workflow_name": run.WorkflowName,
		"namespace":     run.Namespace,
		"state":         run.State,
		"input":         run.Input,
		"output":        run.Output,
		"step_states":   run.StepStates,
		"error":         run.Error,
		"started_at":    run.StartedAt,
		"completed_at":  run.CompletedAt,
		"created_at":    run.CreatedAt,
		"updated_at":    run.UpdatedAt,
	}
}

// Helpers

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func jobToJSON(job *rivo.Job) map[string]any {
	return map[string]any{
		"id":           job.ID,
		"kind":         job.Kind,
		"queue":        job.Queue,
		"state":        job.State,
		"payload":      job.Payload,
		"priority":     job.Priority,
		"attempt":      job.Attempt,
		"max_attempts": job.MaxAttempts,
		"scheduled_at": job.ScheduledAt,
		"created_at":   job.CreatedAt,
		"updated_at":   job.UpdatedAt,
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
