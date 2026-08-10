package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestScheduledTaskHandler_Execute_NoInterval_DoesNotReschedule(t *testing.T) {
	fs := &fakeStore{}
	h := &ScheduledTaskHandler{Store: fs}

	result, err := h.Execute(context.Background(), json.RawMessage(`{"message":"hello"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res ScheduledResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.Message != "hello" {
		t.Errorf("Message = %q, want %q", res.Message, "hello")
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
	h := &ScheduledTaskHandler{Store: fs}

	result, err := h.Execute(context.Background(), json.RawMessage(`{"message":"tick","interval_seconds":60}`))
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

func TestScheduledTaskHandler_Execute_CreateError(t *testing.T) {
	fs := &fakeStore{createErr: errors.New("db down")}
	h := &ScheduledTaskHandler{Store: fs}

	_, err := h.Execute(context.Background(), json.RawMessage(`{"interval_seconds":60}`))
	if err == nil {
		t.Fatal("Execute() error = nil, want the underlying Create error surfaced")
	}
}

func TestScheduledTaskHandler_Execute_InvalidPayload(t *testing.T) {
	h := &ScheduledTaskHandler{Store: &fakeStore{}}
	if _, err := h.Execute(context.Background(), json.RawMessage(`not json`)); err == nil {
		t.Fatal("Execute() error = nil, want a decode error")
	}
}
