package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

func TestReportHandler_Execute_Success(t *testing.T) {
	fs := &fakeStore{
		stats: store.Stats{Pending: 3, Queued: 1, Running: 2, Completed: 40, Dead: 2, Cancelled: 1},
		dead: []*job.Job{
			{ID: uuid.New(), Type: "send_email", LastError: "smtp timeout"},
			{ID: uuid.New(), Type: "resize_image", LastError: "decode failed"},
		},
	}
	h := &ReportHandler{Store: fs}

	result, err := h.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res ReportResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.Stats != fs.stats {
		t.Errorf("Stats = %+v, want %+v", res.Stats, fs.stats)
	}
	if len(res.RecentDeadJobs) != 2 {
		t.Fatalf("len(RecentDeadJobs) = %d, want 2", len(res.RecentDeadJobs))
	}
	if res.Summary == "" {
		t.Error("Summary should not be empty")
	}
}

func TestReportHandler_Execute_EmptyPayloadUsesDefaults(t *testing.T) {
	h := &ReportHandler{Store: &fakeStore{}}
	if _, err := h.Execute(context.Background(), nil); err != nil {
		t.Fatalf("Execute() error = %v, want nil for an empty payload", err)
	}
}

func TestReportHandler_Execute_RespectsSampleSize(t *testing.T) {
	fs := &fakeStore{dead: []*job.Job{
		{ID: uuid.New(), Type: "a"},
		{ID: uuid.New(), Type: "b"},
		{ID: uuid.New(), Type: "c"},
	}}
	h := &ReportHandler{Store: fs}

	result, err := h.Execute(context.Background(), json.RawMessage(`{"failed_sample_size":1}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var res ReportResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(res.RecentDeadJobs) != 1 {
		t.Errorf("len(RecentDeadJobs) = %d, want 1", len(res.RecentDeadJobs))
	}
}

func TestReportHandler_Execute_StatsError(t *testing.T) {
	h := &ReportHandler{Store: &fakeStore{statsErr: errors.New("db down")}}
	if _, err := h.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("Execute() error = nil, want the underlying Stats error surfaced")
	}
}
