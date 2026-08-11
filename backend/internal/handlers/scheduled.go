package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// ScheduledJobType is the job.Registry key for ScheduledTaskHandler.
const ScheduledJobType = "scheduled_task"

// ScheduledPayload is the scheduled_task job's input. TargetType/TargetPayload
// identify another registered job type to actually run each time this fires
// (e.g. "generate_report" to email yourself a queue report every hour, or
// "make_http_request" as a recurring health check) -- the recurrence
// machinery wraps a real action instead of being a no-op demo. When
// IntervalSeconds is positive, the handler enqueues its own successor that
// many seconds out after running.
type ScheduledPayload struct {
	TargetType      string          `json:"target_type"`
	TargetPayload   json.RawMessage `json:"target_payload,omitempty"`
	IntervalSeconds int             `json:"interval_seconds"`
}

// ScheduledResult is the scheduled_task job's output. TargetResult is set
// when the target action succeeded; TargetError is set when it failed -- a
// failed target does not fail the scheduled_task job itself or break the
// recurrence, matching how a real scheduler treats one bad run of a
// recurring check.
type ScheduledResult struct {
	RanAt        time.Time       `json:"ran_at"`
	TargetType   string          `json:"target_type"`
	TargetResult json.RawMessage `json:"target_result,omitempty"`
	TargetError  string          `json:"target_error,omitempty"`
	NextJobID    *uuid.UUID      `json:"next_job_id,omitempty"`
	NextRunAt    *time.Time      `json:"next_run_at,omitempty"`
}

// ScheduledTaskHandler demonstrates recurring jobs without any separate
// cron infrastructure: it runs another registered job type via Registry,
// then -- if the payload asks for it -- enqueues its own next occurrence
// directly through the store. Postgres's run_at + the dispatcher's normal
// due-job poll take it from there -- recurrence is just "a job that creates
// another job".
type ScheduledTaskHandler struct {
	Store    store.Store
	Registry *job.Registry
}

var _ job.Handler = (*ScheduledTaskHandler)(nil)

// Execute implements job.Handler.
func (h *ScheduledTaskHandler) Execute(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var p ScheduledPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("handlers: decode scheduled task payload: %w", err)
	}
	if p.TargetType == "" {
		return nil, errors.New("handlers: scheduled task payload missing \"target_type\"")
	}
	if p.TargetType == ScheduledJobType {
		return nil, errors.New("handlers: scheduled task cannot target itself")
	}

	target, err := h.Registry.Lookup(p.TargetType)
	if err != nil {
		return nil, fmt.Errorf("handlers: scheduled task target: %w", err)
	}

	result := ScheduledResult{RanAt: time.Now().UTC(), TargetType: p.TargetType}
	if targetResult, targetErr := target.Execute(ctx, p.TargetPayload); targetErr != nil {
		result.TargetError = targetErr.Error()
	} else {
		result.TargetResult = targetResult
	}

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
