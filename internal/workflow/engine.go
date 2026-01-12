package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rabin-a/rivo/internal/db"
)

// StepState represents the state of a workflow step.
type StepState string

const (
	StepStatePending   StepState = "pending"
	StepStateRunning   StepState = "running"
	StepStateCompleted StepState = "completed"
	StepStateFailed    StepState = "failed"
	StepStateSkipped   StepState = "skipped"
)

// WorkflowState represents the state of a workflow run.
type WorkflowState string

const (
	WorkflowStatePending   WorkflowState = "pending"
	WorkflowStateRunning   WorkflowState = "running"
	WorkflowStateCompleted WorkflowState = "completed"
	WorkflowStateFailed    WorkflowState = "failed"
	WorkflowStateCancelled WorkflowState = "cancelled"
)

// StepType represents the type of workflow step.
type StepType int

const (
	StepTypeSequential StepType = iota
	StepTypeParallel
	StepTypeConditional
	StepTypeFanOut
	StepTypeHandler
)

// UnmarshalJSON implements json.Unmarshaler for StepType.
func (s *StepType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		// Try as int for backward compatibility
		var i int
		if err := json.Unmarshal(data, &i); err != nil {
			return err
		}
		*s = StepType(i)
		return nil
	}
	switch str {
	case "sequential":
		*s = StepTypeSequential
	case "parallel":
		*s = StepTypeParallel
	case "conditional":
		*s = StepTypeConditional
	case "fanout":
		*s = StepTypeFanOut
	case "handler":
		*s = StepTypeHandler
	default:
		*s = StepTypeSequential
	}
	return nil
}

// MarshalJSON implements json.Marshaler for StepType.
func (s StepType) MarshalJSON() ([]byte, error) {
	var str string
	switch s {
	case StepTypeSequential:
		str = "sequential"
	case StepTypeParallel:
		str = "parallel"
	case StepTypeConditional:
		str = "conditional"
	case StepTypeFanOut:
		str = "fanout"
	case StepTypeHandler:
		str = "handler"
	default:
		str = "sequential"
	}
	return json.Marshal(str)
}

// StepDefinition represents a step in the workflow definition.
type StepDefinition struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Type         StepType         `json:"type"`
	Children     []StepDefinition `json:"children,omitempty"`
	HasCondition bool             `json:"has_condition,omitempty"`
	Timeout      time.Duration    `json:"timeout,omitempty"`
	HandlerID    string           `json:"handler_id,omitempty"` // For handler steps
}

// WorkflowDefinition represents the serializable workflow definition.
type WorkflowDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Steps       []StepDefinition `json:"steps"`
	Timeout     time.Duration    `json:"timeout,omitempty"`
}

// StepRunState represents the runtime state of a step.
type StepRunState struct {
	State       StepState       `json:"state"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	Attempt     int             `json:"attempt"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// StepHandler is a function that executes a workflow step.
type StepHandler func(ctx *Context) error

// ConditionFunc evaluates a condition for conditional steps.
type ConditionFunc func(ctx *Context) bool

// Context provides context for workflow step execution.
type Context struct {
	context.Context
	runID  int64
	stepID string
	input  json.RawMessage
	state  map[string]json.RawMessage
	logger Logger
}

// Logger interface for workflow logging.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// NewContext creates a new workflow context.
func NewContext(ctx context.Context, runID int64, stepID string, input json.RawMessage, state map[string]json.RawMessage, logger Logger) *Context {
	return &Context{
		Context: ctx,
		runID:   runID,
		stepID:  stepID,
		input:   input,
		state:   state,
		logger:  logger,
	}
}

// RunID returns the workflow run ID.
func (c *Context) RunID() int64 {
	return c.runID
}

// StepID returns the current step ID.
func (c *Context) StepID() string {
	return c.stepID
}

// Input unmarshals the workflow input into v.
func (c *Context) Input(v any) error {
	return json.Unmarshal(c.input, v)
}

// GetStepOutput retrieves output from a previous step.
func (c *Context) GetStepOutput(stepID string, v any) error {
	data, ok := c.state[stepID]
	if !ok {
		return fmt.Errorf("step output not found: %s", stepID)
	}
	return json.Unmarshal(data, v)
}

// SetOutput sets the output for the current step.
func (c *Context) SetOutput(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.state[c.stepID] = data
	return nil
}

// Log returns the workflow logger.
func (c *Context) Log() Logger {
	return c.logger
}

// HandlerStepExecutor executes a job handler as a workflow step.
type HandlerStepExecutor func(ctx context.Context, handlerID string, input json.RawMessage) (json.RawMessage, error)

// Engine executes workflows.
type Engine struct {
	db                   *db.DB
	namespace            string
	workerID             string
	handlers             map[string]map[string]StepHandler   // workflow -> step -> handler
	conditions           map[string]map[string]ConditionFunc // workflow -> step -> condition
	fanOutHandlers       map[string]map[string]FanOutHandler // workflow -> step -> fanout handler
	handlerStepExecutor  HandlerStepExecutor                 // Executes job handlers as steps
	mu                   sync.RWMutex
}

// NewEngine creates a new workflow engine.
func NewEngine(database *db.DB, namespace string) *Engine {
	return &Engine{
		db:             database,
		namespace:      namespace,
		handlers:       make(map[string]map[string]StepHandler),
		conditions:     make(map[string]map[string]ConditionFunc),
		fanOutHandlers: make(map[string]map[string]FanOutHandler),
	}
}

// SetWorkerID sets the worker ID for step logging.
func (e *Engine) SetWorkerID(workerID string) {
	e.workerID = workerID
}

// SetHandlerStepExecutor sets the executor for handler steps.
func (e *Engine) SetHandlerStepExecutor(executor HandlerStepExecutor) {
	e.handlerStepExecutor = executor
}

// RegisterStep registers a handler for a workflow step.
func (e *Engine) RegisterStep(workflowName, stepID string, handler StepHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.handlers[workflowName] == nil {
		e.handlers[workflowName] = make(map[string]StepHandler)
	}
	e.handlers[workflowName][stepID] = handler
}

// RegisterCondition registers a condition function for a conditional step.
func (e *Engine) RegisterCondition(workflowName, stepID string, condition ConditionFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.conditions[workflowName] == nil {
		e.conditions[workflowName] = make(map[string]ConditionFunc)
	}
	e.conditions[workflowName][stepID] = condition
}

// ExecuteRun executes a workflow run.
func (e *Engine) ExecuteRun(ctx context.Context, runID int64, logger Logger) error {
	// Get workflow run
	run, err := e.db.GetWorkflowRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("getting workflow run: %w", err)
	}
	if run == nil {
		return fmt.Errorf("workflow run not found: %d", runID)
	}

	// Get workflow definition
	def, err := e.db.GetWorkflowDefinitionByID(ctx, run.DefinitionID)
	if err != nil {
		return fmt.Errorf("getting workflow definition: %w", err)
	}
	if def == nil {
		return fmt.Errorf("workflow definition not found: %d", run.DefinitionID)
	}

	// Parse definition
	var workflowDef WorkflowDefinition
	if err := json.Unmarshal(def.Definition, &workflowDef); err != nil {
		return fmt.Errorf("parsing workflow definition: %w", err)
	}

	// Parse step states
	stepStates := make(map[string]StepRunState)
	if len(run.StepStates) > 0 {
		if err := json.Unmarshal(run.StepStates, &stepStates); err != nil {
			return fmt.Errorf("parsing step states: %w", err)
		}
	}

	// Parse output state
	state := make(map[string]json.RawMessage)
	for stepID, stepState := range stepStates {
		if stepState.Output != nil {
			state[stepID] = stepState.Output
		}
	}

	// Update run state to running
	if err := e.db.UpdateWorkflowRunState(ctx, runID, string(WorkflowStateRunning), "", ""); err != nil {
		return fmt.Errorf("updating workflow run state: %w", err)
	}

	// Execute steps
	for _, step := range workflowDef.Steps {
		if err := e.executeStep(ctx, runID, def.Name, step, run.Input, state, stepStates, logger); err != nil {
			// Update run as failed
			e.db.UpdateWorkflowRunState(ctx, runID, string(WorkflowStateFailed), step.ID, err.Error())
			return err
		}

		// Persist step states after each step
		statesJSON, _ := json.Marshal(stepStates)
		e.db.UpdateWorkflowRunStepStates(ctx, runID, statesJSON)
	}

	// Update run as completed
	if err := e.db.UpdateWorkflowRunState(ctx, runID, string(WorkflowStateCompleted), "", ""); err != nil {
		return fmt.Errorf("updating workflow run state: %w", err)
	}

	return nil
}

func (e *Engine) executeStep(
	ctx context.Context,
	runID int64,
	workflowName string,
	step StepDefinition,
	input json.RawMessage,
	state map[string]json.RawMessage,
	stepStates map[string]StepRunState,
	logger Logger,
) error {
	// Check if step was already completed (replay)
	if existingState, ok := stepStates[step.ID]; ok && existingState.State == StepStateCompleted {
		logger.Debug("skipping completed step", "step", step.ID)
		return nil
	}

	switch step.Type {
	case StepTypeParallel:
		return e.executeParallelSteps(ctx, runID, workflowName, step.Children, input, state, stepStates, logger)
	case StepTypeConditional:
		return e.executeConditionalStep(ctx, runID, workflowName, step, input, state, stepStates, logger)
	case StepTypeFanOut:
		return e.executeFanOutStep(ctx, runID, workflowName, step, input, state, stepStates, logger)
	case StepTypeHandler:
		return e.executeHandlerStep(ctx, runID, workflowName, step, input, state, stepStates, logger)
	default:
		return e.executeSequentialStep(ctx, runID, workflowName, step, input, state, stepStates, logger)
	}
}

func (e *Engine) executeSequentialStep(
	ctx context.Context,
	runID int64,
	workflowName string,
	step StepDefinition,
	input json.RawMessage,
	state map[string]json.RawMessage,
	stepStates map[string]StepRunState,
	logger Logger,
) error {
	e.mu.RLock()
	handlers := e.handlers[workflowName]
	handler, ok := handlers[step.ID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("handler not found for step: %s", step.ID)
	}

	// Get current attempt from step state
	attempt := 1
	if existingState, ok := stepStates[step.ID]; ok {
		attempt = existingState.Attempt + 1
	}

	// Mark step as running
	now := time.Now()
	stepStates[step.ID] = StepRunState{
		State:     StepStateRunning,
		Attempt:   attempt,
		StartedAt: &now,
	}

	// Create step log entry
	logID, _ := e.db.CreateWorkflowStepLog(ctx, runID, step.ID, attempt, e.workerID, input)

	// Create step context
	stepCtx := NewContext(ctx, runID, step.ID, input, state, logger)

	// Apply timeout if set
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
		stepCtx.Context = ctx
	}

	// Execute handler
	logger.Info("executing step", "step", step.ID)
	if err := handler(stepCtx); err != nil {
		completedAt := time.Now()
		stepStates[step.ID] = StepRunState{
			State:       StepStateFailed,
			Error:       err.Error(),
			Attempt:     attempt,
			StartedAt:   &now,
			CompletedAt: &completedAt,
		}
		// Log step failure
		e.db.FailWorkflowStepLog(ctx, logID, err.Error(), "", nil)
		return fmt.Errorf("step %s failed: %w", step.ID, err)
	}

	// Mark step as completed
	completedAt := time.Now()
	stepStates[step.ID] = StepRunState{
		State:       StepStateCompleted,
		Output:      state[step.ID],
		Attempt:     attempt,
		StartedAt:   &now,
		CompletedAt: &completedAt,
	}

	// Log step completion
	e.db.CompleteWorkflowStepLog(ctx, logID, state[step.ID], nil)

	logger.Info("step completed", "step", step.ID)
	return nil
}

func (e *Engine) executeParallelSteps(
	ctx context.Context,
	runID int64,
	workflowName string,
	steps []StepDefinition,
	input json.RawMessage,
	state map[string]json.RawMessage,
	stepStates map[string]StepRunState,
	logger Logger,
) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(steps))
	stateMu := sync.Mutex{}

	for _, step := range steps {
		wg.Add(1)
		go func(s StepDefinition) {
			defer wg.Done()

			// Create a copy of state for this goroutine
			localState := make(map[string]json.RawMessage)
			stateMu.Lock()
			for k, v := range state {
				localState[k] = v
			}
			stateMu.Unlock()

			if err := e.executeSequentialStep(ctx, runID, workflowName, s, input, localState, stepStates, logger); err != nil {
				errCh <- err
				return
			}

			// Merge local state back
			stateMu.Lock()
			for k, v := range localState {
				state[k] = v
			}
			stateMu.Unlock()
		}(step)
	}

	wg.Wait()
	close(errCh)

	// Return first error if any
	for err := range errCh {
		return err
	}

	return nil
}

func (e *Engine) executeConditionalStep(
	ctx context.Context,
	runID int64,
	workflowName string,
	step StepDefinition,
	input json.RawMessage,
	state map[string]json.RawMessage,
	stepStates map[string]StepRunState,
	logger Logger,
) error {
	e.mu.RLock()
	conditions := e.conditions[workflowName]
	condition, ok := conditions[step.ID]
	e.mu.RUnlock()

	if !ok {
		// If no condition registered, skip the step
		logger.Info("skipping conditional step (no condition)", "step", step.ID)
		stepStates[step.ID] = StepRunState{State: StepStateSkipped}
		return nil
	}

	// Evaluate condition
	condCtx := NewContext(ctx, runID, step.ID, input, state, logger)
	if !condition(condCtx) {
		logger.Info("skipping conditional step (condition false)", "step", step.ID)
		stepStates[step.ID] = StepRunState{State: StepStateSkipped}
		return nil
	}

	// Execute the step
	return e.executeSequentialStep(ctx, runID, workflowName, step, input, state, stepStates, logger)
}

// FanOutHandler handles fan-out step execution.
type FanOutHandler struct {
	ItemsFunc      func(ctx *Context) ([]any, error)
	MapFunc        func(ctx *Context, item any, index int) (any, error)
	ReduceFunc     func(ctx *Context, results []any) (any, error)
	MaxConcurrency int
}

// RegisterFanOut registers a fan-out handler for a workflow step.
func (e *Engine) RegisterFanOut(workflowName, stepID string, handler FanOutHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.fanOutHandlers == nil {
		e.fanOutHandlers = make(map[string]map[string]FanOutHandler)
	}
	if e.fanOutHandlers[workflowName] == nil {
		e.fanOutHandlers[workflowName] = make(map[string]FanOutHandler)
	}
	e.fanOutHandlers[workflowName][stepID] = handler
}

func (e *Engine) executeFanOutStep(
	ctx context.Context,
	runID int64,
	workflowName string,
	step StepDefinition,
	input json.RawMessage,
	state map[string]json.RawMessage,
	stepStates map[string]StepRunState,
	logger Logger,
) error {
	e.mu.RLock()
	fanOutHandlers := e.fanOutHandlers[workflowName]
	handler, ok := fanOutHandlers[step.ID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("fan-out handler not found for step: %s", step.ID)
	}

	// Mark step as running
	now := time.Now()
	stepStates[step.ID] = StepRunState{
		State:     StepStateRunning,
		Attempt:   1,
		StartedAt: &now,
	}

	// Create step context
	stepCtx := NewContext(ctx, runID, step.ID, input, state, logger)

	// Get items to fan out
	logger.Info("executing fan-out step", "step", step.ID)
	items, err := handler.ItemsFunc(stepCtx)
	if err != nil {
		completedAt := time.Now()
		stepStates[step.ID] = StepRunState{
			State:       StepStateFailed,
			Error:       err.Error(),
			Attempt:     1,
			StartedAt:   &now,
			CompletedAt: &completedAt,
		}
		return fmt.Errorf("fan-out items func failed for step %s: %w", step.ID, err)
	}

	logger.Info("fan-out step processing items", "step", step.ID, "count", len(items))

	// Execute map function for each item in parallel
	results := make([]any, len(items))
	errCh := make(chan error, len(items))

	// Determine concurrency
	concurrency := len(items)
	if handler.MaxConcurrency > 0 && handler.MaxConcurrency < concurrency {
		concurrency = handler.MaxConcurrency
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, itm any) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create a context for this item
			itemCtx := NewContext(ctx, runID, fmt.Sprintf("%s[%d]", step.ID, idx), input, state, logger)
			result, err := handler.MapFunc(itemCtx, itm, idx)
			if err != nil {
				errCh <- fmt.Errorf("map[%d] failed: %w", idx, err)
				return
			}
			results[idx] = result
		}(i, item)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		completedAt := time.Now()
		stepStates[step.ID] = StepRunState{
			State:       StepStateFailed,
			Error:       err.Error(),
			Attempt:     1,
			StartedAt:   &now,
			CompletedAt: &completedAt,
		}
		return err
	}

	// Run reduce function if provided
	var finalOutput any
	if handler.ReduceFunc != nil {
		reduceCtx := NewContext(ctx, runID, step.ID, input, state, logger)
		finalOutput, err = handler.ReduceFunc(reduceCtx, results)
		if err != nil {
			completedAt := time.Now()
			stepStates[step.ID] = StepRunState{
				State:       StepStateFailed,
				Error:       err.Error(),
				Attempt:     1,
				StartedAt:   &now,
				CompletedAt: &completedAt,
			}
			return fmt.Errorf("reduce failed for step %s: %w", step.ID, err)
		}
	} else {
		finalOutput = results
	}

	// Marshal output
	outputData, err := json.Marshal(finalOutput)
	if err != nil {
		return fmt.Errorf("marshal fan-out output: %w", err)
	}
	state[step.ID] = outputData

	// Mark step as completed
	completedAt := time.Now()
	stepStates[step.ID] = StepRunState{
		State:       StepStateCompleted,
		Output:      outputData,
		Attempt:     1,
		StartedAt:   &now,
		CompletedAt: &completedAt,
	}

	logger.Info("fan-out step completed", "step", step.ID, "items_processed", len(items))
	return nil
}

func (e *Engine) executeHandlerStep(
	ctx context.Context,
	runID int64,
	workflowName string,
	step StepDefinition,
	input json.RawMessage,
	state map[string]json.RawMessage,
	stepStates map[string]StepRunState,
	logger Logger,
) error {
	if e.handlerStepExecutor == nil {
		return fmt.Errorf("handler step executor not configured")
	}

	// Get current attempt from step state
	attempt := 1
	if existingState, ok := stepStates[step.ID]; ok {
		attempt = existingState.Attempt + 1
	}

	// Mark step as running
	now := time.Now()
	stepStates[step.ID] = StepRunState{
		State:     StepStateRunning,
		Attempt:   attempt,
		StartedAt: &now,
	}

	// Create step log entry
	logID, _ := e.db.CreateWorkflowStepLog(ctx, runID, step.ID, attempt, e.workerID, input)

	// Execute the handler step
	logger.Info("executing handler step", "step", step.ID, "handler", step.HandlerID)
	output, err := e.handlerStepExecutor(ctx, step.HandlerID, input)
	if err != nil {
		completedAt := time.Now()
		stepStates[step.ID] = StepRunState{
			State:       StepStateFailed,
			Error:       err.Error(),
			Attempt:     attempt,
			StartedAt:   &now,
			CompletedAt: &completedAt,
		}
		// Log step failure
		e.db.FailWorkflowStepLog(ctx, logID, err.Error(), "", nil)
		return fmt.Errorf("handler step %s failed: %w", step.ID, err)
	}

	// Store output
	state[step.ID] = output

	// Mark step as completed
	completedAt := time.Now()
	stepStates[step.ID] = StepRunState{
		State:       StepStateCompleted,
		Output:      output,
		Attempt:     attempt,
		StartedAt:   &now,
		CompletedAt: &completedAt,
	}

	// Log step completion
	e.db.CompleteWorkflowStepLog(ctx, logID, output, nil)

	logger.Info("handler step completed", "step", step.ID, "handler", step.HandlerID)
	return nil
}
