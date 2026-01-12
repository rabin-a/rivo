package rivo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// WorkflowState represents the current state of a workflow run.
type WorkflowState string

const (
	WorkflowStatePending   WorkflowState = "pending"
	WorkflowStateRunning   WorkflowState = "running"
	WorkflowStateCompleted WorkflowState = "completed"
	WorkflowStateFailed    WorkflowState = "failed"
	WorkflowStateCancelled WorkflowState = "cancelled"
)

// StepState represents the current state of a workflow step.
type StepState string

const (
	StepStatePending   StepState = "pending"
	StepStateRunning   StepState = "running"
	StepStateCompleted StepState = "completed"
	StepStateFailed    StepState = "failed"
	StepStateSkipped   StepState = "skipped"
)

// WorkflowFunc is a function that processes a workflow step.
type WorkflowFunc func(ctx *WorkflowContext) error

// Workflow defines a workflow with multiple steps.
type Workflow struct {
	name        string
	description string
	steps       []workflowStep
	timeout     time.Duration
	retryPolicy RetryPolicy
}

// workflowStep represents a step in the workflow.
type workflowStep struct {
	id          string
	name        string
	stepType    stepType
	handler     WorkflowFunc
	children    []workflowStep // For parallel steps
	condition   func(ctx *WorkflowContext) bool
	retryPolicy *RetryPolicy
	timeout     time.Duration

	// Fan-out configuration
	fanOutConfig *fanOutConfig

	// Handler step configuration (uses job handler)
	handlerID string
}

// fanOutConfig contains configuration for fan-out (dynamic parallelism) steps.
type fanOutConfig struct {
	// ItemsFunc returns a slice of items to process in parallel.
	// The function receives the workflow context and should return items.
	ItemsFunc func(ctx *WorkflowContext) ([]any, error)

	// MapFunc processes each item and returns the result.
	MapFunc func(ctx *WorkflowContext, item any, index int) (any, error)

	// ReduceFunc aggregates all mapped results into final output.
	// If nil, results are returned as an array.
	ReduceFunc func(ctx *WorkflowContext, results []any) (any, error)

	// MaxConcurrency limits parallel execution (0 = unlimited).
	MaxConcurrency int
}

type stepType int

const (
	stepTypeSequential stepType = iota
	stepTypeParallel
	stepTypeConditional
	stepTypeFanOut  // Dynamic parallel expansion
	stepTypeHandler // Uses job handler
)

// NewWorkflow creates a new workflow definition.
func NewWorkflow(name string) *Workflow {
	return &Workflow{
		name:  name,
		steps: make([]workflowStep, 0),
	}
}

// Description sets the workflow description.
func (w *Workflow) Description(desc string) *Workflow {
	w.description = desc
	return w
}

// Timeout sets the overall workflow timeout.
func (w *Workflow) Timeout(d time.Duration) *Workflow {
	w.timeout = d
	return w
}

// Retry sets the default retry policy for all steps.
func (w *Workflow) Retry(policy RetryPolicy) *Workflow {
	w.retryPolicy = policy
	return w
}

// Step adds a sequential step to the workflow.
func (w *Workflow) Step(id string, fn WorkflowFunc, opts ...StepOption) *Workflow {
	step := workflowStep{
		id:       id,
		name:     id,
		stepType: stepTypeSequential,
		handler:  fn,
	}

	for _, opt := range opts {
		opt(&step)
	}

	w.steps = append(w.steps, step)
	return w
}

// Parallel adds parallel steps that execute concurrently.
func (w *Workflow) Parallel(steps ...ParallelStep) *Workflow {
	children := make([]workflowStep, len(steps))
	for i, s := range steps {
		children[i] = workflowStep{
			id:       s.ID,
			name:     s.Name,
			stepType: stepTypeSequential,
			handler:  s.Handler,
			timeout:  s.Timeout,
		}
		if s.Retry != nil {
			children[i].retryPolicy = s.Retry
		}
	}

	w.steps = append(w.steps, workflowStep{
		id:       fmt.Sprintf("parallel_%d", len(w.steps)),
		stepType: stepTypeParallel,
		children: children,
	})
	return w
}

// If adds a conditional step that only executes if the condition is true.
func (w *Workflow) If(condition func(ctx *WorkflowContext) bool, fn WorkflowFunc, opts ...StepOption) *Workflow {
	step := workflowStep{
		id:        fmt.Sprintf("conditional_%d", len(w.steps)),
		stepType:  stepTypeConditional,
		handler:   fn,
		condition: condition,
	}

	for _, opt := range opts {
		opt(&step)
	}

	w.steps = append(w.steps, step)
	return w
}

// HandlerStep adds a step that executes a registered job handler.
// The handler must be registered with RegisterHandler before the workflow starts.
// The step input is passed to the handler as the job payload.
//
// Example:
//
//	// First, register a handler
//	client.RegisterHandler("send-email", func(ctx *rivo.JobContext, job *rivo.Job) error {
//	    var payload EmailPayload
//	    job.Unmarshal(&payload)
//	    return sendEmail(payload)
//	})
//
//	// Then use it in a workflow
//	workflow := rivo.NewWorkflow("order-processing").
//	    Step("validate", validateOrder).
//	    HandlerStep("send-email", "send-confirmation") // Uses the send-email handler
func (w *Workflow) HandlerStep(handlerID string, stepID string, opts ...StepOption) *Workflow {
	step := workflowStep{
		id:        stepID,
		name:      stepID,
		stepType:  stepTypeHandler,
		handlerID: handlerID,
	}

	for _, opt := range opts {
		opt(&step)
	}

	w.steps = append(w.steps, step)
	return w
}

// ForEach adds a fan-out step that processes items in parallel.
// It retrieves items using itemsFunc, processes each with mapFunc, and optionally
// reduces the results with reduceFunc.
//
// Example:
//
//	workflow.ForEach(
//	    "process-orders",
//	    func(ctx *WorkflowContext) ([]any, error) {
//	        // Get order IDs from previous step output
//	        var prevOutput struct{ OrderIDs []string }
//	        ctx.GetStepOutput("get-orders", &prevOutput)
//	        items := make([]any, len(prevOutput.OrderIDs))
//	        for i, id := range prevOutput.OrderIDs {
//	            items[i] = id
//	        }
//	        return items, nil
//	    },
//	    func(ctx *WorkflowContext, item any, index int) (any, error) {
//	        orderID := item.(string)
//	        // Process each order
//	        return map[string]any{"order_id": orderID, "processed": true}, nil
//	    },
//	    WithReducer(func(ctx *WorkflowContext, results []any) (any, error) {
//	        // Summarize all results
//	        return map[string]any{"total_processed": len(results)}, nil
//	    }),
//	)
func (w *Workflow) ForEach(
	id string,
	itemsFunc func(ctx *WorkflowContext) ([]any, error),
	mapFunc func(ctx *WorkflowContext, item any, index int) (any, error),
	opts ...FanOutOption,
) *Workflow {
	config := &fanOutConfig{
		ItemsFunc: itemsFunc,
		MapFunc:   mapFunc,
	}

	for _, opt := range opts {
		opt(config)
	}

	step := workflowStep{
		id:           id,
		name:         id,
		stepType:     stepTypeFanOut,
		fanOutConfig: config,
	}

	w.steps = append(w.steps, step)
	return w
}

// FanOutOption configures a fan-out step.
type FanOutOption func(*fanOutConfig)

// WithReducer sets a reducer function to aggregate fan-out results.
func WithReducer(fn func(ctx *WorkflowContext, results []any) (any, error)) FanOutOption {
	return func(c *fanOutConfig) {
		c.ReduceFunc = fn
	}
}

// WithMaxConcurrency limits the number of concurrent fan-out executions.
func WithMaxConcurrency(max int) FanOutOption {
	return func(c *fanOutConfig) {
		c.MaxConcurrency = max
	}
}

// Name returns the workflow name.
func (w *Workflow) Name() string {
	return w.name
}

// Steps returns the workflow steps for inspection.
func (w *Workflow) Steps() []string {
	names := make([]string, 0, len(w.steps))
	for _, s := range w.steps {
		if s.stepType == stepTypeParallel {
			for _, c := range s.children {
				names = append(names, c.id)
			}
		} else {
			names = append(names, s.id)
		}
	}
	return names
}

// StepOption configures a workflow step.
type StepOption func(*workflowStep)

// WithStepName sets the step display name.
func WithStepName(name string) StepOption {
	return func(s *workflowStep) {
		s.name = name
	}
}

// WithStepRetry sets the retry policy for a step.
func WithStepRetry(policy RetryPolicy) StepOption {
	return func(s *workflowStep) {
		s.retryPolicy = &policy
	}
}

// WithStepTimeout sets the timeout for a step.
func WithStepTimeout(d time.Duration) StepOption {
	return func(s *workflowStep) {
		s.timeout = d
	}
}

// ParallelStep defines a step to run in parallel.
type ParallelStep struct {
	ID      string
	Name    string
	Handler WorkflowFunc
	Retry   *RetryPolicy
	Timeout time.Duration
}

// WorkflowContext provides context for workflow step execution.
type WorkflowContext struct {
	context.Context
	runID    int64
	stepID   string
	workflow *Workflow
	input    json.RawMessage
	state    map[string]json.RawMessage // Step outputs keyed by step ID
	logger   *JobLogger
}

// RunID returns the workflow run ID.
func (c *WorkflowContext) RunID() int64 {
	return c.runID
}

// StepID returns the current step ID.
func (c *WorkflowContext) StepID() string {
	return c.stepID
}

// Input unmarshals the workflow input into v.
func (c *WorkflowContext) Input(v any) error {
	return json.Unmarshal(c.input, v)
}

// GetStepOutput retrieves the output from a previous step.
func (c *WorkflowContext) GetStepOutput(stepID string, v any) error {
	data, ok := c.state[stepID]
	if !ok {
		return fmt.Errorf("step output not found: %s", stepID)
	}
	return json.Unmarshal(data, v)
}

// SetOutput sets the output for the current step.
func (c *WorkflowContext) SetOutput(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.state[c.stepID] = data
	return nil
}

// Logger returns the workflow logger.
func (c *WorkflowContext) Logger() *JobLogger {
	return c.logger
}

// WorkflowRun represents a running or completed workflow instance.
type WorkflowRun struct {
	ID           int64
	WorkflowName string
	Namespace    string
	State        WorkflowState
	Input        json.RawMessage
	Output       json.RawMessage
	StepStates   map[string]StepRunState
	CurrentStep  string
	Error        string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StepRunState represents the state of a step in a workflow run.
type StepRunState struct {
	State       StepState       `json:"state"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	Attempt     int             `json:"attempt"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// StartWorkflowParams contains parameters for starting a workflow.
type StartWorkflowParams struct {
	// WorkflowName is the name of the workflow to start.
	WorkflowName string

	// Input is the initial input data for the workflow.
	Input any

	// IdempotencyKey prevents duplicate workflow runs.
	IdempotencyKey string
}

// StartWorkflowResult contains the result of starting a workflow.
type StartWorkflowResult struct {
	Run       *WorkflowRun
	Duplicate bool
}
