package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// ScheduledJobType is the job.Registry key for ScheduledTaskHandler.
const ScheduledJobType = "scheduled_task"

// ScheduledPayload is the scheduled_task job's input. Message is an
// arbitrary payload for this occurrence (kept generic since this handler
// exists to demonstrate recurrence, not a specific task); when
// IntervalSeconds is positive, the handler enqueues its own successor that
// many seconds out after running.
type ScheduledPayload struct {
	Message         string `json:"message"`
	IntervalSeconds int    `json:"interval_seconds"`
}

// ScheduledResult is the scheduled_task job's output.
type ScheduledResult struct {
	RanAt     time.Time  `json:"ran_at"`
	Message   string     `json:"message"`
	NextJobID *uuid.UUID `json:"next_job_id,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
}

// ScheduledTaskHandler demonstrates recurring jobs without any separate
// cron infrastructure: on each successful run, if the payload asks for it,
// the handler enqueues its own next occurrence directly through the store.
// Postgres's run_at + the dispatcher's normal due-job poll take it from
// there — recurrence is just "a job that creates another job".
type ScheduledTaskHandler struct {
	Store store.Store
}

var _ job.Handler = (*ScheduledTaskHandler)(nil)

// Execute implements job.Handler.
func (h *ScheduledTaskHandler) Execute(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var p ScheduledPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("handlers: decode scheduled task payload: %w", err)
	}

	result := ScheduledResult{RanAt: time.Now().UTC(), Message: p.Message}

	if p.IntervalSeconds > 0 {
		nextRunAt := time.Now().UTC().Add(time.Duration(p.IntervalSeconds) * time.Second)
		next := job.New(ScheduledJobType, payload, job.WithRunAt(nextRunAt))
		if err := h.Store.Create(ctx, next); err != nil {
			return nil, fmt.Errorf("handlers: schedule next occurrence: %w", err)
		}
		result.NextJobID = &next.ID
		result.NextRunAt = &nextRunAt
	}

	return json.Marshal(result)
}
