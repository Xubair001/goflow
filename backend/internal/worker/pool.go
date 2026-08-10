// Package worker consumes jobs from the queue, executes them against
// registered handlers, and records the outcome in the store.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/queue"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// Config controls worker pool sizing and timing.
type Config struct {
	// Concurrency is how many jobs may execute at once.
	Concurrency int
	// ConsumeBatch bounds how many messages a single fetch pulls at once.
	ConsumeBatch int64
	// ReclaimInterval is how often to sweep for messages abandoned by dead
	// consumers.
	ReclaimInterval time.Duration
	// ReclaimMinIdle is how long a message must sit unacknowledged before
	// this pool will take over its processing.
	ReclaimMinIdle time.Duration
	// JobTimeout bounds how long a single handler execution (including
	// recording its outcome) may run.
	JobTimeout time.Duration
	// RetryBaseDelay and RetryMaxDelay bound the exponential-backoff-with-jitter
	// delay before a failed job is retried.
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
}

// Pool consumes jobs from a queue.Queue, executes them against handlers
// registered in a job.Registry, and records the outcome in a store.Store.
//
// Architecture: one fetcher goroutine and one reclaimer goroutine feed a
// shared channel; Concurrency worker goroutines drain it. This keeps the
// queue's consumer-group identity simple — the pool is a single logical
// consumer — while still processing jobs in parallel across goroutines.
type Pool struct {
	store    store.Store
	queue    queue.Queue
	registry *job.Registry
	consumer string
	cfg      Config
	logger   *slog.Logger
}

// New returns a Pool identified to the queue's consumer group as consumer.
func New(s store.Store, q queue.Queue, registry *job.Registry, consumer string, cfg Config, logger *slog.Logger) *Pool {
	return &Pool{store: s, queue: q, registry: registry, consumer: consumer, cfg: cfg, logger: logger}
}

// Run starts the fetcher, reclaimer, and Concurrency worker goroutines, and
// blocks until ctx is cancelled and every in-flight job has finished.
func (p *Pool) Run(ctx context.Context) error {
	messages := make(chan queue.Message, p.cfg.Concurrency)

	var producers sync.WaitGroup
	producers.Add(2)
	go func() { defer producers.Done(); p.fetchLoop(ctx, messages) }()
	go func() { defer producers.Done(); p.reclaimLoop(ctx, messages) }()

	var workers sync.WaitGroup
	workers.Add(p.cfg.Concurrency)
	for range p.cfg.Concurrency {
		// Deliberately context.Background() inside, not ctx: a message
		// already pulled off the queue should get its full JobTimeout to
		// finish even if shutdown starts mid-flight, rather than being
		// cancelled just because the process is stopping.
		// fetchLoop/reclaimLoop (which do watch ctx) are what stop new
		// work from starting.
		go func() { //nolint:gosec // intentional: see comment above
			defer workers.Done()
			for msg := range messages {
				p.Process(context.Background(), msg)
			}
		}()
	}

	// Once both producers have stopped pulling new work, close the channel
	// so workers drain whatever's buffered and then exit.
	producers.Wait()
	close(messages)
	workers.Wait()
	return nil
}

func (p *Pool) fetchLoop(ctx context.Context, out chan<- queue.Message) {
	for {
		if ctx.Err() != nil {
			return
		}
		msgs, err := p.queue.Consume(ctx, p.consumer, p.cfg.ConsumeBatch)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			p.logger.Error("consume", "error", err)
			continue
		}
		if !deliver(ctx, out, msgs) {
			return
		}
	}
}

func (p *Pool) reclaimLoop(ctx context.Context, out chan<- queue.Message) {
	ticker := time.NewTicker(p.cfg.ReclaimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		msgs, err := p.queue.Reclaim(ctx, p.consumer, p.cfg.ReclaimMinIdle, p.cfg.ConsumeBatch)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			p.logger.Error("reclaim", "error", err)
			continue
		}
		if !deliver(ctx, out, msgs) {
			return
		}
	}
}

// deliver sends each message to out, returning false if ctx is cancelled
// before all of them are delivered.
func deliver(ctx context.Context, out chan<- queue.Message, msgs []queue.Message) bool {
	for _, m := range msgs {
		select {
		case out <- m:
		case <-ctx.Done():
			return false
		}
	}
	return true
}

// Process handles one queue message end to end: load the job, run its
// handler, record the outcome, and acknowledge the message. It's exported
// so tests can drive it directly and deterministically instead of through
// the fetch/reclaim loops.
//
// A message is only Ack'd once its outcome is durably recorded in Postgres
// (or Postgres has confirmed there's nothing left to do). Any other error
// leaves it unacknowledged, so the queue's own redelivery/reclaim
// mechanism gives it another chance instead of silently losing it.
func (p *Pool) Process(ctx context.Context, msg queue.Message) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.JobTimeout)
	defer cancel()

	jobID, err := uuid.Parse(msg.JobID)
	if err != nil {
		p.logger.Error("malformed job id in queue message, dropping", "raw_job_id", msg.JobID, "error", err)
		p.ack(ctx, msg)
		return
	}

	j, err := p.store.MarkRunning(ctx, jobID, p.consumer)
	if errors.Is(err, store.ErrNotFound) {
		// Already completed, cancelled, or dead via another delivery of
		// this job -- nothing left to do with this message.
		p.ack(ctx, msg)
		return
	}
	if err != nil {
		p.logger.Error("mark job running", "job_id", jobID, "error", err)
		return
	}

	logger := p.logger.With("job_id", j.ID, "job_type", j.Type, "attempt", j.Attempts)

	handler, err := p.registry.Lookup(j.Type)
	if err != nil {
		logger.Error("no handler registered", "error", err)
		if killErr := p.store.Kill(ctx, j.ID, err.Error()); killErr != nil {
			logger.Error("kill job after missing handler", "error", killErr)
			return
		}
		p.ack(ctx, msg)
		return
	}

	result, err := p.execute(ctx, handler, j.Payload)
	if err != nil {
		if failErr := p.fail(ctx, j, err.Error(), logger); failErr != nil {
			logger.Error("record job failure", "error", failErr)
			return
		}
		p.ack(ctx, msg)
		return
	}

	if err := p.store.Complete(ctx, j.ID, result); err != nil {
		logger.Error("complete job", "error", err)
		return
	}
	logger.Info("job completed")
	p.ack(ctx, msg)
}

// execute runs h.Execute, converting a panic into an error so one broken
// handler can't take down the worker goroutine running it -- and with it,
// every other job that goroutine would otherwise have processed.
func (p *Pool) execute(ctx context.Context, h job.Handler, payload json.RawMessage) (result json.RawMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()
	return h.Execute(ctx, payload)
}

// fail records a failed attempt: retry with backoff if attempts remain,
// otherwise mark the job dead.
func (p *Pool) fail(ctx context.Context, j *job.Job, errMsg string, logger *slog.Logger) error {
	if j.ExhaustedRetries() {
		logger.Warn("job exhausted retries, marking dead", "error", errMsg)
		return p.store.Kill(ctx, j.ID, errMsg)
	}
	delay := Backoff(j.Attempts, p.cfg.RetryBaseDelay, p.cfg.RetryMaxDelay)
	logger.Warn("job failed, scheduling retry", "error", errMsg, "retry_in", delay)
	return p.store.Retry(ctx, j.ID, errMsg, time.Now().Add(delay))
}

func (p *Pool) ack(ctx context.Context, msg queue.Message) {
	if err := p.queue.Ack(ctx, msg); err != nil {
		p.logger.Error("ack message", "message_id", msg.ID, "error", err)
	}
}
