// Package store defines the durable job repository. Postgres is the only
// implementation and the system's source of truth: Redis (see package
// queue) only ever carries a delivery hint derived from what's stored here.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

// ErrNotFound is returned when a lookup or state-transition targets a job
// ID that doesn't exist.
var ErrNotFound = errors.New("store: job not found")

// ListFilter narrows List results. A nil field means "don't filter on this".
type ListFilter struct {
	Status *job.Status
	Type   *string
	Limit  int
	Offset int
}

// ListResult is a page of jobs plus the total count matching the filter,
// for pagination.
type ListResult struct {
	Jobs  []*job.Job
	Total int
}

// Stats is a point-in-time count of jobs per status, for the queue overview.
type Stats struct {
	Pending   int
	Queued    int
	Running   int
	Completed int
	Dead      int
	Cancelled int
}

// Store is the durable job repository. All methods are safe for concurrent
// use by multiple goroutines and multiple processes (apiserver, dispatcher,
// worker) against the same database.
type Store interface {
	// Create inserts j. j.Status must be job.StatusPending.
	Create(ctx context.Context, j *job.Job) error

	// Get returns the job with the given ID, or ErrNotFound.
	Get(ctx context.Context, id uuid.UUID) (*job.Job, error)

	// List returns a filtered, paginated view of jobs, newest first.
	List(ctx context.Context, filter ListFilter) (ListResult, error)

	// ClaimDue atomically claims up to limit pending jobs whose RunAt has
	// arrived, transitioning them to StatusQueued, and returns them.
	// Concurrent callers (multiple dispatcher instances) never receive the
	// same row: unclaimable rows are skipped via SELECT ... FOR UPDATE
	// SKIP LOCKED rather than blocking.
	ClaimDue(ctx context.Context, limit int) ([]*job.Job, error)

	// MarkRunning transitions a queued (or already-running, for redelivery)
	// job to StatusRunning, records the claiming consumer, and increments
	// Attempts. It returns ErrNotFound if the job is no longer in a claimable
	// state (e.g. it was cancelled or already completed by another delivery
	// of the same stream entry).
	MarkRunning(ctx context.Context, id uuid.UUID, consumer string) (*job.Job, error)

	// Complete marks a job StatusCompleted with the given result.
	Complete(ctx context.Context, id uuid.UUID, result json.RawMessage) error

	// Retry records a failed attempt and reschedules the job as
	// StatusPending at nextRunAt for another attempt.
	Retry(ctx context.Context, id uuid.UUID, lastErr string, nextRunAt time.Time) error

	// Kill marks a job StatusDead after its attempts are exhausted.
	Kill(ctx context.Context, id uuid.UUID, lastErr string) error

	// Cancel marks a not-yet-running job StatusCancelled. It returns
	// ErrNotFound if the job is already running or in a terminal state.
	Cancel(ctx context.Context, id uuid.UUID) error

	// ReclaimStale finds queued/running jobs whose lease (locked_at) is
	// older than olderThan — evidence their Redis stream entry was lost or
	// their worker died without reporting back — and resets them to
	// StatusPending so the dispatcher republishes them.
	ReclaimStale(ctx context.Context, olderThan time.Duration, limit int) ([]*job.Job, error)

	// Stats returns current job counts by status.
	Stats(ctx context.Context) (Stats, error)
}
