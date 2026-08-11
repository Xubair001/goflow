package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

// noopHandler is a job.Handler stand-in for a target job type, letting
// scheduled_test.go control exactly what the "target" returns without
// depending on a real handler's own dependencies.
type noopHandler struct {
	result json.RawMessage
	err    error
}

func (h *noopHandler) Execute(context.Context, json.RawMessage) (json.RawMessage, error) {
	return h.result, h.err
}

func registryWithTarget(targetType string, h job.Handler) *job.Registry {
	reg := job.NewRegistry()
	reg.Register(targetType, h)
	return reg
}

func TestScheduledTaskHandler_Execute_RunsTargetAndDoesNotReschedule(t *testing.T) {
	fs := &fakeStore{}
	target := &noopHandler{result: json.RawMessage(`{"ok":true}`)}
	h := &ScheduledTaskHandler{Store: fs, Registry: registryWithTarget("make_http_request", target)}

	result, err := h.Execute(context.Background(), json.RawMessage(`{"target_type":"make_http_request"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res ScheduledResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.TargetType != "make_http_request" {
		t.Errorf("TargetType = %q, want %q", res.TargetType, "make_http_request")
	}
	if string(res.TargetResult) != `{"ok":true}` {
		t.Errorf("TargetResult = %s, want {\"ok\":true}", res.TargetResult)
	}
	if res.TargetError != "" {
		t.Errorf("TargetError = %q, want empty", res.TargetError)
	}
	if res.NextJobID != nil {
		t.Error("NextJobID should be nil when interval_seconds is not set")
	}
	if len(fs.created) != 0 {
		t.Errorf("Create was called %d times, want 0", len(fs.created))
	}
}

func TestScheduledTaskHandler_Execute_WithInterval_SchedulesNextOccurrence(t *testing.T) {
	fs := &fakeStore{}
	target := &noopHandler{result: json.RawMessage(`{"ok":true}`)}
	h := &ScheduledTaskHandler{Store: fs, Registry: registryWithTarget("make_http_request", target)}

	result, err := h.Execute(context.Background(), json.RawMessage(
		`{"target_type":"make_http_request","interval_seconds":60}`,
	))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res ScheduledResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.NextJobID == nil {
		t.Fatal("NextJobID should be set when interval_seconds > 0")
	}
	if res.NextRunAt == nil {
		t.Fatal("NextRunAt should be set when interval_seconds > 0")
	}
	if !res.NextRunAt.After(res.RanAt) {
		t.Errorf("NextRunAt = %v, want after RanAt %v", res.NextRunAt, res.RanAt)
	}

	if len(fs.created) != 1 {
		t.Fatalf("Create was called %d times, want 1", len(fs.created))
	}
	next := fs.created[0]
	if next.Type != ScheduledJobType {
		t.Errorf("next job Type = %q, want %q", next.Type, ScheduledJobType)
	}
	if next.ID != *res.NextJobID {
		t.Errorf("next job ID = %s, want %s", next.ID, *res.NextJobID)
	}
}

func TestScheduledTaskHandler_Execute_TargetFailure_StillSucceedsAndReschedules(t *testing.T) {
	fs := &fakeStore{}
	target := &noopHandler{err: errors.New("boom")}
	h := &ScheduledTaskHandler{Store: fs, Registry: registryWithTarget("make_http_request", target)}

	result, err := h.Execute(context.Background(), json.RawMessage(
		`{"target_type":"make_http_request","interval_seconds":60}`,
	))
	if err != nil {
		t.Fatalf("Execute() error = %v, want the scheduled_task job itself to succeed even if its target failed", err)
	}

	var res ScheduledResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.TargetError != "boom" {
		t.Errorf("TargetError = %q, want %q", res.TargetError, "boom")
	}
	if res.TargetResult != nil {
		t.Errorf("TargetResult = %s, want nil when the target failed", res.TargetResult)
	}
	if res.NextJobID == nil {
		t.Error("a failed target run should not break the recurrence: NextJobID should still be set")
	}
}

func TestScheduledTaskHandler_Execute_CreateError(t *testing.T) {
	fs := &fakeStore{createErr: errors.New("db down")}
	target := &noopHandler{result: json.RawMessage(`{}`)}
	h := &ScheduledTaskHandler{Store: fs, Registry: registryWithTarget("make_http_request", target)}

	_, err := h.Execute(context.Background(), json.RawMessage(
		`{"target_type":"make_http_request","interval_seconds":60}`,
	))
	if err == nil {
		t.Fatal("Execute() error = nil, want the underlying Create error surfaced")
	}
}

func TestScheduledTaskHandler_Execute_InvalidPayload(t *testing.T) {
	h := &ScheduledTaskHandler{Store: &fakeStore{}, Registry: job.NewRegistry()}
	if _, err := h.Execute(context.Background(), json.RawMessage(`not json`)); err == nil {
		t.Fatal("Execute() error = nil, want a decode error")
	}
}

func TestScheduledTaskHandler_Execute_MissingTargetType(t *testing.T) {
	h := &ScheduledTaskHandler{Store: &fakeStore{}, Registry: job.NewRegistry()}
	if _, err := h.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("Execute() error = nil, want an error for missing \"target_type\"")
	}
}

func TestScheduledTaskHandler_Execute_RejectsSelfTarget(t *testing.T) {
	h := &ScheduledTaskHandler{Store: &fakeStore{}, Registry: job.NewRegistry()}
	_, err := h.Execute(context.Background(), json.RawMessage(`{"target_type":"scheduled_task"}`))
	if err == nil {
		t.Fatal("Execute() error = nil, want a rejection of a scheduled_task targeting itself")
	}
}

func TestScheduledTaskHandler_Execute_UnregisteredTargetType(t *testing.T) {
	h := &ScheduledTaskHandler{Store: &fakeStore{}, Registry: job.NewRegistry()}
	_, err := h.Execute(context.Background(), json.RawMessage(`{"target_type":"does_not_exist"}`))
	if !errors.Is(err, job.ErrHandlerNotRegistered) {
		t.Errorf("Execute() error = %v, want job.ErrHandlerNotRegistered", err)
	}
}
