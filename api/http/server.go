package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rabin-a/rivo"
)

// Server provides an HTTP API for Rivo.
type Server struct {
	client  *rivo.Client
	handler http.Handler
	server  *http.Server
}

// Config configures the HTTP server.
type Config struct {
	// Address to listen on (e.g., ":8080").
	Address string

	// Client is the Rivo client to use.
	Client *rivo.Client

	// AuthMiddleware is an optional authentication middleware.
	AuthMiddleware func(http.Handler) http.Handler
}

// NewServer creates a new HTTP API server.
func NewServer(cfg Config) *Server {
	s := &Server{
		client: cfg.Client,
	}

	mux := http.NewServeMux()

	// Job endpoints
	mux.HandleFunc("POST /api/v1/jobs", s.handleEnqueue)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/cancel", s.handleCancelJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/retry", s.handleRetryJob)

	// Workflow endpoints
	mux.HandleFunc("POST /api/v1/workflows/{name}/start", s.handleStartWorkflow)
	mux.HandleFunc("GET /api/v1/workflows/runs/{id}", s.handleGetWorkflowRun)
	mux.HandleFunc("POST /api/v1/workflows/runs/{id}/cancel", s.handleCancelWorkflow)
	mux.HandleFunc("GET /api/v1/workflows/runs/{id}/steps", s.handleGetWorkflowStepLogs)
	mux.HandleFunc("POST /api/v1/workflows/runs/{id}/steps/{stepId}/rerun", s.handleRerunWorkflowStep)

	// Health endpoint
	mux.HandleFunc("GET /health", s.handleHealth)

	var handler http.Handler = mux
	if cfg.AuthMiddleware != nil {
		handler = cfg.AuthMiddleware(handler)
	}

	s.handler = handler
	s.server = &http.Server{
		Addr:    cfg.Address,
		Handler: handler,
	}

	return s
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Handler returns the HTTP handler for embedding in other servers.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Request/Response types

type enqueueRequest struct {
	Kind           string          `json:"kind"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	Queue          string          `json:"queue,omitempty"`
	Priority       int16           `json:"priority,omitempty"`
	ScheduleAt     *time.Time      `json:"schedule_at,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	MaxAttempts    int16           `json:"max_attempts,omitempty"`
}

type jobResponse struct {
	ID          int64           `json:"id"`
	Kind        string          `json:"kind"`
	Queue       string          `json:"queue"`
	State       string          `json:"state"`
	Payload     json.RawMessage `json:"payload"`
	Priority    int16           `json:"priority"`
	Attempt     int16           `json:"attempt"`
	MaxAttempts int16           `json:"max_attempts"`
	ScheduledAt time.Time       `json:"scheduled_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Handlers

func (s *Server) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	var req enqueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	params := rivo.EnqueueParams{
		Kind:           req.Kind,
		Queue:          req.Queue,
		Priority:       req.Priority,
		IdempotencyKey: req.IdempotencyKey,
		MaxAttempts:    req.MaxAttempts,
	}

	if req.Payload != nil {
		params.Payload = req.Payload
	}
	if req.ScheduleAt != nil {
		params.ScheduleAt = *req.ScheduleAt
	}

	result, err := s.client.Enqueue(r.Context(), params)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to enqueue job", err)
		return
	}

	resp := s.jobToResponse(result.Job)

	status := http.StatusCreated
	if result.Duplicate {
		status = http.StatusOK
	}

	s.writeJSON(w, status, resp)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid job ID", err)
		return
	}

	job, err := s.client.GetJob(r.Context(), id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to get job", err)
		return
	}
	if job == nil {
		s.writeError(w, http.StatusNotFound, "job not found", nil)
		return
	}

	s.writeJSON(w, http.StatusOK, s.jobToResponse(job))
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid job ID", err)
		return
	}

	if err := s.client.CancelJob(r.Context(), id); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to cancel job", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	// Placeholder for retry implementation
	s.writeError(w, http.StatusNotImplemented, "not implemented", nil)
}

func (s *Server) handleStartWorkflow(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, http.StatusNotImplemented, "not implemented", nil)
}

func (s *Server) handleGetWorkflowRun(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, http.StatusNotImplemented, "not implemented", nil)
}

func (s *Server) handleCancelWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid workflow run ID", err)
		return
	}

	if err := s.client.CancelWorkflowRun(r.Context(), id); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to cancel workflow", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetWorkflowStepLogs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid workflow run ID", err)
		return
	}

	logs, err := s.client.GetWorkflowStepLogs(r.Context(), id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to get step logs", err)
		return
	}

	s.writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleRerunWorkflowStep(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid workflow run ID", err)
		return
	}

	stepID := r.PathValue("stepId")
	if stepID == "" {
		s.writeError(w, http.StatusBadRequest, "step ID is required", nil)
		return
	}

	if err := s.client.RerunWorkflowStep(r.Context(), id, stepID); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to rerun step", err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "rerun started"})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Helpers

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string, err error) {
	resp := errorResponse{
		Error:   http.StatusText(status),
		Message: message,
	}
	if err != nil {
		resp.Message = message + ": " + err.Error()
	}
	s.writeJSON(w, status, resp)
}

func (s *Server) jobToResponse(job *rivo.Job) jobResponse {
	return jobResponse{
		ID:          job.ID,
		Kind:        job.Kind,
		Queue:       job.Queue,
		State:       string(job.State),
		Payload:     job.Payload,
		Priority:    job.Priority,
		Attempt:     job.Attempt,
		MaxAttempts: job.MaxAttempts,
		ScheduledAt: job.ScheduledAt,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}
}
