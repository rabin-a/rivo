package rivo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rabin-a/rivo/internal/db"
	"github.com/rabin-a/rivo/internal/executor"
	"github.com/rabin-a/rivo/internal/migration"
	"github.com/rabin-a/rivo/internal/workflow"
)

// Common errors.
var (
	ErrAlreadyStarted     = errors.New("client already started")
	ErrNotStarted         = errors.New("client not started")
	ErrInvalidJobKind     = errors.New("job kind is required")
	ErrHandlerNotFound    = errors.New("handler not found for job kind")
	ErrWorkflowNotFound   = errors.New("workflow not found")
	ErrWorkflowRunNotFound = errors.New("workflow run not found")
)

// Config configures the Rivo client.
type Config struct {
	// DatabaseURL is the Postgres connection string (required).
	DatabaseURL string

	// MaxConns is the maximum number of database connections.
	// Defaults to 25.
	MaxConns int32

	// Workers is the number of concurrent job workers.
	// Defaults to 10.
	Workers int

	// Queues to process. Defaults to ["default"].
	Queues []QueueConfig

	// Namespace isolates jobs. Defaults to "default".
	Namespace string

	// PollInterval is how often to poll for new jobs.
	// Defaults to 100ms.
	PollInterval time.Duration

	// JobTimeout is the default timeout for job execution.
	// Defaults to 5 minutes.
	JobTimeout time.Duration

	// AutoMigrate runs database migrations on startup.
	// Defaults to true.
	AutoMigrate bool
}

// QueueConfig configures a queue.
type QueueConfig struct {
	Name        string
	Priority    int
	Concurrency int
}

// Tx represents a database transaction for transactional enqueuing.
type Tx = pgx.Tx

// Client is the main entry point for Rivo.
type Client struct {
	config   Config
	db       *db.DB
	workerID string

	handlers map[string]*registeredHandler
	workflows map[string]*Workflow
	workflowEngine *workflow.Engine

	producer  *executor.Producer
	exec      *executor.Executor
	completer *executor.Completer

	mu       sync.RWMutex
	started  bool
	shutdown chan struct{}
	done     chan struct{}
}

// NewClient creates a new Rivo client.
func NewClient(ctx context.Context, config Config) (*Client, error) {
	if config.DatabaseURL == "" {
		return nil, errors.New("database URL is required")
	}

	// Apply defaults
	if config.MaxConns == 0 {
		config.MaxConns = 25
	}
	if config.Workers == 0 {
		config.Workers = 10
	}
	if config.Namespace == "" {
		config.Namespace = "default"
	}
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond
	}
	if config.JobTimeout == 0 {
		config.JobTimeout = 5 * time.Minute
	}
	if len(config.Queues) == 0 {
		config.Queues = []QueueConfig{{Name: "default"}}
	}

	// Connect to database
	database, err := db.New(ctx, config.DatabaseURL, config.MaxConns)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Run migrations if enabled
	if config.AutoMigrate {
		migrator := migration.New(database.Pool())
		if err := migrator.Migrate(ctx); err != nil {
			database.Close()
			return nil, fmt.Errorf("running migrations: %w", err)
		}
	}

	workerID := uuid.New().String()
	engine := workflow.NewEngine(database, config.Namespace)
	engine.SetWorkerID(workerID)

	client := &Client{
		config:         config,
		db:             database,
		workerID:       workerID,
		handlers:       make(map[string]*registeredHandler),
		workflows:      make(map[string]*Workflow),
		workflowEngine: engine,
		shutdown:       make(chan struct{}),
		done:           make(chan struct{}),
	}

	// Set up handler step executor
	engine.SetHandlerStepExecutor(func(ctx context.Context, handlerID string, input json.RawMessage) (json.RawMessage, error) {
		client.mu.RLock()
		handler, ok := client.handlers[handlerID]
		client.mu.RUnlock()

		if !ok {
			return nil, fmt.Errorf("handler not found: %s", handlerID)
		}

		// Create a synthetic job for the handler
		job := &Job{
			ID:          0, // Not a real job
			Kind:        handlerID,
			Queue:       handler.queue,
			Namespace:   config.Namespace,
			Payload:     input,
			Attempt:     1,
			MaxAttempts: 1,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Create job context
		jobCtx := NewJobContext(ctx, job)

		// Execute handler
		if err := handler.Handle(jobCtx, job); err != nil {
			return nil, err
		}

		// Return output (if handler set one via JobContext)
		return nil, nil
	})

	return client, nil
}

// RegisterHandler registers a handler for a job kind.
// By default, the job kind is used as the queue name.
// Use HandlerOptions.Queue to specify a different queue.
func (c *Client) RegisterHandler(kind string, fn HandlerFunc, opts ...HandlerOptions) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var options HandlerOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	// Default queue to kind name if not specified
	queue := options.Queue
	if queue == "" {
		queue = kind
	}

	c.handlers[kind] = &registeredHandler{
		kind:    kind,
		queue:   queue,
		fn:      fn,
		options: options,
	}
}

// Start begins processing jobs.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return ErrAlreadyStarted
	}
	c.started = true

	// Collect queues from registered handlers
	queueSet := make(map[string]bool)
	for _, h := range c.handlers {
		queueSet[h.queue] = true
	}

	// Add queues from config
	for _, q := range c.config.Queues {
		queueSet[q.Name] = true
	}

	// If no queues, use default
	if len(queueSet) == 0 {
		queueSet["default"] = true
	}

	queues := make([]string, 0, len(queueSet))
	for q := range queueSet {
		queues = append(queues, q)
	}
	c.mu.Unlock()

	// Register this worker
	c.db.RegisterWorker(ctx, &db.Worker{
		ID:          c.workerID,
		Namespace:   c.config.Namespace,
		Queues:      queues,
		Concurrency: c.config.Workers,
		Status:      "active",
		StartedAt:   time.Now(),
	})

	// Create producer
	c.producer = executor.NewProducer(executor.ProducerConfig{
		DB:           c.db,
		Namespace:    c.config.Namespace,
		Queues:       queues,
		WorkerID:     c.workerID,
		PollInterval: c.config.PollInterval,
		BatchSize:    100,
		BufferSize:   c.config.Workers * 10,
	})

	// Create executor
	c.exec = executor.NewExecutor(executor.ExecutorConfig{
		Workers: c.config.Workers,
		Handler: c.handleJob,
		Jobs:    c.producer.Jobs(),
		Timeout: c.config.JobTimeout,
	})

	// Create completer
	c.completer = executor.NewCompleter(executor.CompleterConfig{
		DB:          c.db,
		Results:     c.exec.Results(),
		RetryPolicy: executor.DefaultRetryPolicy(),
	})

	// Start all components
	c.producer.Start(ctx)
	c.exec.Start(ctx)
	c.completer.Start(ctx)

	// Start scheduler goroutine
	go c.runScheduler(ctx)

	// Start heartbeat goroutine
	go c.runHeartbeat(ctx)

	// Wait for shutdown
	go func() {
		<-c.shutdown
		c.producer.Stop()
		c.producer.Wait()
		c.exec.Stop()
		c.exec.Wait()
		c.completer.Stop()
		c.completer.Wait()
		// Deregister worker
		c.db.DeregisterWorker(context.Background(), c.workerID)
		close(c.done)
	}()

	return nil
}

// handleJob dispatches a job to its registered handler.
func (c *Client) handleJob(ctx context.Context, jobRow *db.JobRow) error {
	c.mu.RLock()
	handler, ok := c.handlers[jobRow.Kind]
	c.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrHandlerNotFound, jobRow.Kind)
	}

	// Convert db.JobRow to Job
	job := c.jobFromRow(jobRow)

	// Apply handler-specific timeout
	if handler.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, handler.options.Timeout)
		defer cancel()
	}

	// Create job context with logger
	jobCtx := NewJobContext(ctx, job)

	// Create job log entry
	logID, _ := c.db.CreateJobLog(ctx, job.ID, job.Attempt, c.workerID, job.Payload)

	// Execute the handler
	err := handler.Handle(jobCtx, job)

	// Save logs to database
	logs := jobCtx.Logger().JSON()
	if err != nil {
		c.db.FailJobLog(ctx, logID, err.Error(), "", logs)
	} else {
		c.db.CompleteJobLog(ctx, logID, nil, logs)
	}

	return err
}

func (c *Client) jobFromRow(row *db.JobRow) *Job {
	job := &Job{
		ID:          row.ID,
		Kind:        row.Kind,
		Queue:       row.Queue,
		Namespace:   row.Namespace,
		Priority:    row.Priority,
		State:       JobState(row.State),
		Payload:     row.Payload,
		ScheduledAt: row.ScheduledAt,
		Attempt:     row.Attempt,
		MaxAttempts: row.MaxAttempts,
		AttemptedAt: row.AttemptedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}

	if row.AttemptedBy != nil {
		job.AttemptedBy = *row.AttemptedBy
	}
	if row.CompletedAt != nil {
		job.CompletedAt = row.CompletedAt
	}
	if row.CancelledAt != nil {
		job.CancelledAt = row.CancelledAt
	}
	if row.DiscardedAt != nil {
		job.DiscardedAt = row.DiscardedAt
	}
	if row.IdempotencyKey != nil {
		job.IdempotencyKey = *row.IdempotencyKey
	}
	if row.WorkflowRunID != nil {
		job.WorkflowRunID = row.WorkflowRunID
	}
	if row.StepID != nil {
		job.StepID = *row.StepID
	}

	return job
}

// runScheduler promotes scheduled jobs to available state.
func (c *Client) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case <-ticker.C:
			c.db.PromoteScheduledJobs(ctx, c.config.Namespace)
		}
	}
}

// runHeartbeat sends periodic heartbeats to the database.
func (c *Client) runHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case <-ticker.C:
			c.db.WorkerHeartbeat(ctx, c.workerID)
		}
	}
}

// Shutdown gracefully stops the client.
func (c *Client) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	close(c.shutdown)

	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := c.Shutdown(ctx); err != nil {
		// Continue to close database even if shutdown fails
	}

	c.db.Close()
	return nil
}

// GetJob retrieves a job by ID.
func (c *Client) GetJob(ctx context.Context, id int64) (*Job, error) {
	row, err := c.db.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return c.jobFromRow(row), nil
}

// CancelJob cancels a pending or running job.
func (c *Client) CancelJob(ctx context.Context, id int64) error {
	return c.db.CancelJob(ctx, id)
}

// WorkerID returns the unique ID of this worker.
func (c *Client) WorkerID() string {
	return c.workerID
}

// DB returns the underlying database instance.
func (c *Client) DB() *db.DB {
	return c.db
}

// Namespace returns the client's namespace.
func (c *Client) Namespace() string {
	return c.config.Namespace
}

// HandlerInfo contains information about a registered handler.
type HandlerInfo struct {
	Kind        string
	Queue       string
	Concurrency int
	MaxAttempts int
}

// RegisteredHandlers returns information about all registered handlers.
func (c *Client) RegisteredHandlers() []HandlerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	handlers := make([]HandlerInfo, 0, len(c.handlers))
	for _, h := range c.handlers {
		maxAttempts := h.options.Retry.MaxAttempts
		if maxAttempts == 0 {
			maxAttempts = 25
		}
		handlers = append(handlers, HandlerInfo{
			Kind:        h.kind,
			Queue:       h.queue,
			Concurrency: h.options.Concurrency,
			MaxAttempts: maxAttempts,
		})
	}
	return handlers
}

// RegisterWorkflow registers a workflow and its step handlers.
func (c *Client) RegisterWorkflow(w *Workflow) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.workflows[w.name] = w

	// Register step handlers with the engine
	for _, step := range w.steps {
		c.registerWorkflowStep(w.name, step)
	}
}

func (c *Client) registerWorkflowStep(workflowName string, step workflowStep) {
	if step.stepType == stepTypeParallel {
		for _, child := range step.children {
			c.registerWorkflowStep(workflowName, child)
		}
		return
	}

	if step.handler != nil {
		c.workflowEngine.RegisterStep(workflowName, step.id, func(ctx *workflow.Context) error {
			// Create WorkflowContext from workflow.Context
			wfCtx := &WorkflowContext{
				Context:  ctx.Context,
				runID:    ctx.RunID(),
				stepID:   ctx.StepID(),
				workflow: c.workflows[workflowName],
				state:    make(map[string]json.RawMessage),
				logger:   NewJobLogger(),
			}

			// Copy input
			var input json.RawMessage
			ctx.Input(&input)
			wfCtx.input = input

			// Execute handler
			if err := step.handler(wfCtx); err != nil {
				return err
			}

			// Copy output back
			if output, ok := wfCtx.state[step.id]; ok {
				var v any
				json.Unmarshal(output, &v)
				ctx.SetOutput(v)
			}

			return nil
		})
	}

	if step.condition != nil {
		c.workflowEngine.RegisterCondition(workflowName, step.id, func(ctx *workflow.Context) bool {
			wfCtx := &WorkflowContext{
				Context:  ctx.Context,
				runID:    ctx.RunID(),
				stepID:   ctx.StepID(),
				workflow: c.workflows[workflowName],
				state:    make(map[string]json.RawMessage),
				logger:   NewJobLogger(),
			}
			return step.condition(wfCtx)
		})
	}

	// Register fan-out (Map) handler
	if step.fanOutConfig != nil {
		c.workflowEngine.RegisterFanOut(workflowName, step.id, workflow.FanOutHandler{
			ItemsFunc: func(ctx *workflow.Context) ([]any, error) {
				wfCtx := c.createWorkflowContext(ctx, workflowName)
				return step.fanOutConfig.ItemsFunc(wfCtx)
			},
			MapFunc: func(ctx *workflow.Context, item any, index int) (any, error) {
				wfCtx := c.createWorkflowContext(ctx, workflowName)
				return step.fanOutConfig.MapFunc(wfCtx, item, index)
			},
			MaxConcurrency: step.fanOutConfig.MaxConcurrency,
		})
	}

	// Register reduce handler
	if step.reduceConfig != nil {
		c.workflowEngine.RegisterReduce(workflowName, step.id, func(ctx *workflow.Context, results []any) (any, error) {
			wfCtx := c.createWorkflowContext(ctx, workflowName)
			return step.reduceConfig.ReduceFunc(wfCtx, results)
		})
	}
}

// createWorkflowContext creates a WorkflowContext from workflow.Context.
func (c *Client) createWorkflowContext(ctx *workflow.Context, workflowName string) *WorkflowContext {
	wfCtx := &WorkflowContext{
		Context:  ctx.Context,
		runID:    ctx.RunID(),
		stepID:   ctx.StepID(),
		workflow: c.workflows[workflowName],
		state:    make(map[string]json.RawMessage),
		logger:   NewJobLogger(),
	}

	// Copy input
	var input json.RawMessage
	ctx.Input(&input)
	wfCtx.input = input

	return wfCtx
}

// StartWorkflow starts a new workflow run.
func (c *Client) StartWorkflow(ctx context.Context, params StartWorkflowParams) (*StartWorkflowResult, error) {
	c.mu.RLock()
	w, ok := c.workflows[params.WorkflowName]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrWorkflowNotFound, params.WorkflowName)
	}

	// Serialize input
	inputJSON, err := json.Marshal(params.Input)
	if err != nil {
		return nil, fmt.Errorf("marshaling input: %w", err)
	}

	// Create workflow definition JSON
	defJSON, err := c.serializeWorkflowDefinition(w)
	if err != nil {
		return nil, fmt.Errorf("serializing workflow: %w", err)
	}

	// Insert or get workflow definition
	def, err := c.db.InsertWorkflowDefinition(ctx, w.name, c.config.Namespace, defJSON)
	if err != nil {
		return nil, fmt.Errorf("inserting workflow definition: %w", err)
	}

	// Initialize step states
	stepStates := make(map[string]StepRunState)
	for _, stepID := range w.Steps() {
		stepStates[stepID] = StepRunState{State: StepStatePending}
	}
	stepStatesJSON, _ := json.Marshal(stepStates)

	// Create workflow run
	runRow, err := c.db.CreateWorkflowRun(ctx, def.ID, c.config.Namespace, inputJSON, stepStatesJSON)
	if err != nil {
		return nil, fmt.Errorf("creating workflow run: %w", err)
	}

	run := c.workflowRunFromRow(runRow, w.name)

	// Execute workflow asynchronously
	go func() {
		logger := &workflowLogger{}
		c.workflowEngine.ExecuteRun(context.Background(), run.ID, logger)
	}()

	return &StartWorkflowResult{Run: run}, nil
}

func (c *Client) serializeWorkflowDefinition(w *Workflow) (json.RawMessage, error) {
	def := struct {
		Name        string               `json:"name"`
		Description string               `json:"description,omitempty"`
		Steps       []workflowStepDef    `json:"steps"`
		Timeout     string               `json:"timeout,omitempty"`
	}{
		Name:        w.name,
		Description: w.description,
		Steps:       make([]workflowStepDef, len(w.steps)),
	}

	if w.timeout > 0 {
		def.Timeout = w.timeout.String()
	}

	for i, s := range w.steps {
		def.Steps[i] = c.serializeStep(s)
	}

	return json.Marshal(def)
}

type workflowStepDef struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Children     []workflowStepDef `json:"children,omitempty"`
	HasCondition bool              `json:"has_condition,omitempty"`
	Timeout      string            `json:"timeout,omitempty"`
	HandlerID    string            `json:"handler_id,omitempty"`
	SourceStepID string            `json:"source_step_id,omitempty"`
}

func (c *Client) serializeStep(s workflowStep) workflowStepDef {
	def := workflowStepDef{
		ID:           s.id,
		Name:         s.name,
		HasCondition: s.condition != nil,
	}

	switch s.stepType {
	case stepTypeSequential:
		def.Type = "sequential"
	case stepTypeParallel:
		def.Type = "parallel"
		def.Children = make([]workflowStepDef, len(s.children))
		for i, child := range s.children {
			def.Children[i] = c.serializeStep(child)
		}
	case stepTypeConditional:
		def.Type = "conditional"
	case stepTypeFanOut:
		def.Type = "fanout"
	case stepTypeReduce:
		def.Type = "reduce"
		if s.reduceConfig != nil {
			def.SourceStepID = s.reduceConfig.SourceStepID
		}
	case stepTypeHandler:
		def.Type = "handler"
		def.HandlerID = s.handlerID
	}

	if s.timeout > 0 {
		def.Timeout = s.timeout.String()
	}

	return def
}

func (c *Client) workflowRunFromRow(row *db.WorkflowRunRow, workflowName string) *WorkflowRun {
	run := &WorkflowRun{
		ID:           row.ID,
		WorkflowName: workflowName,
		Namespace:    row.Namespace,
		State:        WorkflowState(row.State),
		Input:        row.Input,
		Output:       row.Output,
		Error:        row.Error,
		StartedAt:    row.StartedAt,
		CompletedAt:  row.CompletedAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}

	// Parse step states
	if len(row.StepStates) > 0 {
		json.Unmarshal(row.StepStates, &run.StepStates)
	}

	return run
}

// GetWorkflowRun retrieves a workflow run by ID.
func (c *Client) GetWorkflowRun(ctx context.Context, id int64) (*WorkflowRun, error) {
	row, err := c.db.GetWorkflowRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	// Get workflow name from definition
	def, err := c.db.GetWorkflowDefinitionByID(ctx, row.DefinitionID)
	if err != nil {
		return nil, err
	}

	workflowName := ""
	if def != nil {
		workflowName = def.Name
	}

	return c.workflowRunFromRow(row, workflowName), nil
}

// ListWorkflowRuns returns workflow runs with optional filtering.
func (c *Client) ListWorkflowRuns(ctx context.Context, state string, limit int) ([]*WorkflowRun, error) {
	rows, err := c.db.ListWorkflowRuns(ctx, c.config.Namespace, state, limit)
	if err != nil {
		return nil, err
	}

	runs := make([]*WorkflowRun, len(rows))
	for i, row := range rows {
		def, _ := c.db.GetWorkflowDefinitionByID(ctx, row.DefinitionID)
		workflowName := ""
		if def != nil {
			workflowName = def.Name
		}
		runs[i] = c.workflowRunFromRow(row, workflowName)
	}

	return runs, nil
}

// CancelWorkflowRun cancels a running workflow.
func (c *Client) CancelWorkflowRun(ctx context.Context, id int64) error {
	return c.db.UpdateWorkflowRunState(ctx, id, string(WorkflowStateCancelled), "", "cancelled by user")
}

// RegisteredWorkflows returns the names of all registered workflows.
func (c *Client) RegisteredWorkflows() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.workflows))
	for name := range c.workflows {
		names = append(names, name)
	}
	return names
}

// WorkflowStepLog represents a workflow step execution log.
type WorkflowStepLog struct {
	ID            int64           `json:"id"`
	WorkflowRunID int64           `json:"workflow_run_id"`
	StepID        string          `json:"step_id"`
	Attempt       int             `json:"attempt"`
	WorkerID      string          `json:"worker_id,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	DurationMs    *int            `json:"duration_ms,omitempty"`
	Input         json.RawMessage `json:"input,omitempty"`
	Output        json.RawMessage `json:"output,omitempty"`
	Status        string          `json:"status"`
	Error         string          `json:"error,omitempty"`
	ErrorStack    string          `json:"error_stack,omitempty"`
	Logs          json.RawMessage `json:"logs,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// GetWorkflowStepLogs returns execution logs for a workflow run.
func (c *Client) GetWorkflowStepLogs(ctx context.Context, runID int64) ([]*WorkflowStepLog, error) {
	rows, err := c.db.GetWorkflowStepLogs(ctx, runID)
	if err != nil {
		return nil, err
	}

	logs := make([]*WorkflowStepLog, len(rows))
	for i, row := range rows {
		logs[i] = &WorkflowStepLog{
			ID:            row.ID,
			WorkflowRunID: row.WorkflowRunID,
			StepID:        row.StepID,
			Attempt:       row.Attempt,
			WorkerID:      row.WorkerID,
			StartedAt:     row.StartedAt,
			CompletedAt:   row.CompletedAt,
			DurationMs:    row.DurationMs,
			Input:         row.Input,
			Output:        row.Output,
			Status:        row.Status,
			Error:         row.Error,
			ErrorStack:    row.ErrorStack,
			Logs:          row.Logs,
			CreatedAt:     row.CreatedAt,
		}
	}
	return logs, nil
}

// RerunWorkflowStep reruns a failed step and all subsequent steps.
func (c *Client) RerunWorkflowStep(ctx context.Context, runID int64, stepID string) error {
	// Get workflow run
	run, err := c.GetWorkflowRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("getting workflow run: %w", err)
	}
	if run == nil {
		return ErrWorkflowRunNotFound
	}

	// Check if the workflow is in a state that allows rerun
	if run.State != WorkflowStateFailed && run.State != WorkflowStateCancelled {
		return fmt.Errorf("workflow must be in failed or cancelled state to rerun steps")
	}

	// Get the workflow definition
	c.mu.RLock()
	w, ok := c.workflows[run.WorkflowName]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrWorkflowNotFound, run.WorkflowName)
	}

	// Find the step index
	stepIDs := w.Steps()
	stepIndex := -1
	for i, id := range stepIDs {
		if id == stepID {
			stepIndex = i
			break
		}
	}
	if stepIndex == -1 {
		return fmt.Errorf("step not found: %s", stepID)
	}

	// Reset step states from the target step onwards
	newStepStates := make(map[string]StepRunState)
	for id, state := range run.StepStates {
		found := false
		for i, sid := range stepIDs {
			if sid == id && i >= stepIndex {
				found = true
				break
			}
		}
		if found {
			// Reset this step to pending
			newStepStates[id] = StepRunState{
				State:   StepStatePending,
				Attempt: state.Attempt, // Keep attempt count for logging
			}
		} else {
			newStepStates[id] = state
		}
	}

	// Update workflow run
	stepStatesJSON, _ := json.Marshal(newStepStates)
	if err := c.db.ResetWorkflowStepForRerun(ctx, runID, stepStatesJSON); err != nil {
		return fmt.Errorf("resetting workflow step: %w", err)
	}

	// Re-execute workflow asynchronously
	go func() {
		logger := &workflowLogger{}
		c.workflowEngine.ExecuteRun(context.Background(), runID, logger)
	}()

	return nil
}

// workflowLogger implements workflow.Logger.
type workflowLogger struct{}

func (l *workflowLogger) Info(msg string, args ...any)  {}
func (l *workflowLogger) Error(msg string, args ...any) {}
func (l *workflowLogger) Debug(msg string, args ...any) {}
