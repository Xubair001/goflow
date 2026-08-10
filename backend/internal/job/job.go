// Package job defines the domain model shared by the API, dispatcher, and
// worker processes: what a Job is, the states it moves through, and the
// Handler contract that executes one.
package job

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status is the lifecycle state of a Job. Postgres is the source of truth
// for status; Redis only carries a delivery hint derived from it.
//
// Valid transitions:
//
//	pending   -> queued              (dispatcher claims a due job)
//	queued    -> running             (worker picks it up from the stream)
//	running   -> completed           (handler succeeded)
//	running   -> pending             (handler failed, attempts remain)
//	running   -> dead                (handler failed, attempts exhausted)
//	pending   -> cancelled           (cancelled before dispatch)
//	queued    -> cancelled           (cancelled before a worker claims it)
type Status string

const (
	StatusPending   Status = "pending"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusDead      Status = "dead"
	StatusCancelled Status = "cancelled"
)

// Valid reports whether s is one of the defined statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusQueued, StatusRunning, StatusCompleted, StatusDead, StatusCancelled:
		return true
	default:
		return false
	}
}

// Terminal reports whether s is an end state a job never leaves.
func (s Status) Terminal() bool {
	switch s {
	case StatusCompleted, StatusDead, StatusCancelled:
		return true
	default:
		return false
	}
}

// DefaultMaxAttempts is used when a job is created without WithMaxAttempts.
const DefaultMaxAttempts = 5

// Job is one unit of work: its type identifies which registered Handler
// executes it, and Payload is the type-specific input for that handler.
type Job struct {
	ID          uuid.UUID
	Type        string
	Payload     json.RawMessage
	Status      Status
	Priority    int
	RunAt       time.Time
	Attempts    int
	MaxAttempts int
	LastError   string
	Result      json.RawMessage
	LockedBy    string
	LockedAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Option customizes a Job at creation time.
type Option func(*Job)

// WithPriority sets the job's dispatch priority; higher values are
// dispatched first among otherwise-due pending jobs.
func WithPriority(p int) Option {
	return func(j *Job) { j.Priority = p }
}

// WithRunAt schedules the job to become eligible for dispatch at t instead
// of immediately, for delayed or self-rescheduling jobs.
func WithRunAt(t time.Time) Option {
	return func(j *Job) { j.RunAt = t }
}

// WithMaxAttempts overrides DefaultMaxAttempts for jobs that should retry
// more or less aggressively than the default.
func WithMaxAttempts(n int) Option {
	return func(j *Job) { j.MaxAttempts = n }
}

// New creates a Job of the given type ready for submission: Status is
// StatusPending and RunAt defaults to now, so it's immediately due.
func New(jobType string, payload json.RawMessage, opts ...Option) *Job {
	j := &Job{
		ID:          uuid.New(),
		Type:        jobType,
		Payload:     payload,
		Status:      StatusPending,
		RunAt:       time.Now().UTC(),
		MaxAttempts: DefaultMaxAttempts,
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}

// ExhaustedRetries reports whether the job has used up all its attempts.
func (j *Job) ExhaustedRetries() bool {
	return j.Attempts >= j.MaxAttempts
}
