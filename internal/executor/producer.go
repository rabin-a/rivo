package executor

import (
	"context"
	"sync"
	"time"

	"github.com/rabin-a/rivo/internal/db"
)

// Producer fetches jobs from the database and sends them to workers.
type Producer struct {
	db           *db.DB
	namespace    string
	queues       []string
	workerID     string
	pollInterval time.Duration
	batchSize    int

	jobs     chan *db.JobRow
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// ProducerConfig configures the producer.
type ProducerConfig struct {
	DB           *db.DB
	Namespace    string
	Queues       []string
	WorkerID     string
	PollInterval time.Duration
	BatchSize    int
	BufferSize   int
}

// NewProducer creates a new job producer.
func NewProducer(cfg ProducerConfig) *Producer {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 1000
	}
	if len(cfg.Queues) == 0 {
		cfg.Queues = []string{"default"}
	}

	return &Producer{
		db:           cfg.DB,
		namespace:    cfg.Namespace,
		queues:       cfg.Queues,
		workerID:     cfg.WorkerID,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		jobs:         make(chan *db.JobRow, cfg.BufferSize),
		shutdown:     make(chan struct{}),
	}
}

// Start begins producing jobs.
func (p *Producer) Start(ctx context.Context) {
	p.wg.Add(1)
	go p.run(ctx)
}

// Stop signals the producer to stop.
func (p *Producer) Stop() {
	close(p.shutdown)
}

// Wait waits for the producer to finish.
func (p *Producer) Wait() {
	p.wg.Wait()
}

// Jobs returns the channel of available jobs.
func (p *Producer) Jobs() <-chan *db.JobRow {
	return p.jobs
}

func (p *Producer) run(ctx context.Context) {
	defer p.wg.Done()
	defer close(p.jobs)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Start with an immediate fetch
	p.fetchAndSend(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.shutdown:
			return
		case <-ticker.C:
			p.fetchAndSend(ctx)
		}
	}
}

func (p *Producer) fetchAndSend(ctx context.Context) {
	// Calculate how many jobs we can fetch based on buffer space
	available := cap(p.jobs) - len(p.jobs)
	if available <= 0 {
		return
	}

	limit := p.batchSize
	if available < limit {
		limit = available
	}

	jobs, err := p.db.FetchJobs(ctx, p.namespace, p.queues, limit, p.workerID)
	if err != nil {
		// Log error in production
		return
	}

	for _, job := range jobs {
		select {
		case p.jobs <- job:
		case <-ctx.Done():
			return
		case <-p.shutdown:
			return
		}
	}
}
