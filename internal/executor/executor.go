package executor

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rabin-a/rivo/internal/db"
)

// JobHandler is the function signature for handling jobs.
type JobHandler func(ctx context.Context, job *db.JobRow) error

// JobResult contains the result of job execution.
type JobResult struct {
	Job   *db.JobRow
	Error error
}

// Executor manages a pool of workers that execute jobs.
type Executor struct {
	workers     int
	handler     JobHandler
	jobs        <-chan *db.JobRow
	results     chan JobResult
	timeout     time.Duration

	shutdown chan struct{}
	wg       sync.WaitGroup
}

// ExecutorConfig configures the executor.
type ExecutorConfig struct {
	Workers int
	Handler JobHandler
	Jobs    <-chan *db.JobRow
	Timeout time.Duration
}

// NewExecutor creates a new job executor.
func NewExecutor(cfg ExecutorConfig) *Executor {
	if cfg.Workers == 0 {
		cfg.Workers = 10
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}

	return &Executor{
		workers:  cfg.Workers,
		handler:  cfg.Handler,
		jobs:     cfg.Jobs,
		results:  make(chan JobResult, cfg.Workers*2),
		timeout:  cfg.Timeout,
		shutdown: make(chan struct{}),
	}
}

// Start begins executing jobs.
func (e *Executor) Start(ctx context.Context) {
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(ctx, i)
	}
}

// Stop signals the executor to stop.
func (e *Executor) Stop() {
	close(e.shutdown)
}

// Wait waits for all workers to finish.
func (e *Executor) Wait() {
	e.wg.Wait()
	close(e.results)
}

// Results returns the channel of job results.
func (e *Executor) Results() <-chan JobResult {
	return e.results
}

func (e *Executor) worker(ctx context.Context, id int) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.shutdown:
			return
		case job, ok := <-e.jobs:
			if !ok {
				return
			}
			result := e.executeJob(ctx, job)
			select {
			case e.results <- result:
			case <-ctx.Done():
				return
			case <-e.shutdown:
				return
			}
		}
	}
}

func (e *Executor) executeJob(ctx context.Context, job *db.JobRow) (result JobResult) {
	result.Job = job

	// Create context with timeout
	if e.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.timeout)
		defer cancel()
	}

	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			result.Error = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
	}()

	result.Error = e.handler(ctx, job)
	return result
}
