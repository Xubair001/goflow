// Package dispatcher moves due jobs from Postgres onto the Redis queue and
// heals leases that expired without the job completing.
package dispatcher

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/abdullah-zubair/jobqueue/internal/queue"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// Config controls dispatch and reconciliation cadence.
type Config struct {
	// PollInterval is how often to look for newly-due pending jobs.
	PollInterval time.Duration
	// ReconcileInterval is how often to sweep for stale leases.
	ReconcileInterval time.Duration
	// BatchSize bounds how many jobs a single dispatch or reconcile pass
	// claims at once.
	BatchSize int
	// StaleAfter is how long a job may sit queued/running before the
	// reconciler assumes its lease was lost and resets it to pending.
	StaleAfter time.Duration
}

// Dispatcher claims due jobs from a store.Store and publishes them to a
// queue.Queue for workers to consume. It also runs a reconciliation pass
// that resets jobs stuck queued/running past their lease back to pending,
// so a dispatcher crash between claiming a job and publishing it — or a
// worker crash that a consumer-group-level reclaim never covers — doesn't
// strand the job forever.
type Dispatcher struct {
	store  store.Store
	queue  queue.Queue
	cfg    Config
	logger *slog.Logger
}

// New returns a Dispatcher ready to Run.
func New(s store.Store, q queue.Queue, cfg Config, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{store: s, queue: q, cfg: cfg, logger: logger}
}

// Run blocks, dispatching due jobs and reconciling stale leases on their
// configured intervals, until ctx is cancelled. It returns once both loops
// have exited, so the caller can rely on Run returning as a signal that
// dispatch/reconcile have stopped touching the store and queue.
func (d *Dispatcher) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		d.loop(ctx, d.cfg.PollInterval, d.DispatchOnce)
	}()
	go func() {
		defer wg.Done()
		d.loop(ctx, d.cfg.ReconcileInterval, d.ReconcileOnce)
	}()

	wg.Wait()
	return nil
}

// loop runs fn immediately, then again every interval, until ctx is done.
func (d *Dispatcher) loop(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		fn(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// DispatchOnce claims up to Config.BatchSize due jobs and publishes each to
// the queue. It's exported (rather than folded into the loop) so it can be
// driven directly and deterministically in tests, and by anything that
// wants an on-demand dispatch pass outside the regular interval.
func (d *Dispatcher) DispatchOnce(ctx context.Context) {
	jobs, err := d.store.ClaimDue(ctx, d.cfg.BatchSize)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			d.logger.Error("claim due jobs", "error", err)
		}
		return
	}

	for _, j := range jobs {
		if err := d.queue.Publish(ctx, j.ID.String()); err != nil {
			// The job stays "queued" in Postgres with a fresh locked_at.
			// If this keeps failing, ReconcileOnce eventually resets it to
			// pending once its lease goes stale, and a later dispatch pass
			// retries the publish.
			d.logger.Error("publish job", "job_id", j.ID, "job_type", j.Type, "error", err)
			continue
		}
		d.logger.Info("dispatched job", "job_id", j.ID, "job_type", j.Type)
	}
}

// ReconcileOnce resets up to Config.BatchSize jobs whose lease has been
// held longer than Config.StaleAfter back to pending. Exported for the
// same reason as DispatchOnce.
func (d *Dispatcher) ReconcileOnce(ctx context.Context) {
	jobs, err := d.store.ReclaimStale(ctx, d.cfg.StaleAfter, d.cfg.BatchSize)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			d.logger.Error("reclaim stale jobs", "error", err)
		}
		return
	}

	for _, j := range jobs {
		d.logger.Warn("reclaimed stale job", "job_id", j.ID, "job_type", j.Type, "attempts", j.Attempts)
	}
}
