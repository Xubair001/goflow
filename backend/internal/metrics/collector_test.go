package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// fakeStore is a minimal store.Store exposing only Stats with real
// behavior, for testing QueueDepthCollector without a real Postgres.
type fakeStore struct {
	stats    store.Stats
	statsErr error
}

func (f *fakeStore) Stats(context.Context) (store.Stats, error) {
	if f.statsErr != nil {
		return store.Stats{}, f.statsErr
	}
	return f.stats, nil
}

func (f *fakeStore) Create(context.Context, *job.Job) error { return nil }

func (f *fakeStore) Get(context.Context, uuid.UUID) (*job.Job, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) List(context.Context, store.ListFilter) (store.ListResult, error) {
	return store.ListResult{}, nil
}

func (f *fakeStore) ClaimDue(context.Context, int) ([]*job.Job, error) { return nil, nil }

func (f *fakeStore) MarkRunning(context.Context, uuid.UUID, string) (*job.Job, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) Complete(context.Context, uuid.UUID, json.RawMessage) error { return nil }

func (f *fakeStore) Retry(context.Context, uuid.UUID, string, time.Time) error { return nil }

func (f *fakeStore) Kill(context.Context, uuid.UUID, string) error { return nil }

func (f *fakeStore) Cancel(context.Context, uuid.UUID) (*job.Job, error) { return nil, nil }

func (f *fakeStore) Reactivate(context.Context, uuid.UUID) (*job.Job, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) ReclaimStale(context.Context, time.Duration, int) ([]*job.Job, error) {
	return nil, nil
}

var _ store.Store = (*fakeStore)(nil)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func collectMetrics(t *testing.T, c prometheus.Collector) map[string]float64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 16)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	out := make(map[string]float64)
	for m := range ch {
		var pb dto.Metric
		if err := m.Write(&pb); err != nil {
			t.Fatalf("write metric: %v", err)
		}
		var status string
		for _, l := range pb.GetLabel() {
			if l.GetName() == "status" {
				status = l.GetValue()
			}
		}
		out[status] = pb.GetGauge().GetValue()
	}
	return out
}

func TestQueueDepthCollector_Collect(t *testing.T) {
	fs := &fakeStore{stats: store.Stats{
		Pending: 3, Queued: 1, Running: 2, Completed: 40, Dead: 2, Cancelled: 1,
	}}
	c := NewQueueDepthCollector(fs, testLogger())

	got := collectMetrics(t, c)
	want := map[string]float64{
		"pending": 3, "queued": 1, "running": 2, "completed": 40, "dead": 2, "cancelled": 1,
	}
	for status, wantVal := range want {
		if got[status] != wantVal {
			t.Errorf("status %q = %v, want %v", status, got[status], wantVal)
		}
	}
}

func TestQueueDepthCollector_Collect_StoreError(t *testing.T) {
	fs := &fakeStore{statsErr: errors.New("db down")}
	c := NewQueueDepthCollector(fs, testLogger())

	// Must not panic; just yields no metrics for this scrape.
	got := collectMetrics(t, c)
	if len(got) != 0 {
		t.Errorf("Collect() on Stats error produced %v, want no metrics", got)
	}
}

func TestQueueDepthCollector_Describe(t *testing.T) {
	c := NewQueueDepthCollector(&fakeStore{}, testLogger())
	ch := make(chan *prometheus.Desc, 1)
	c.Describe(ch)
	close(ch)

	if _, ok := <-ch; !ok {
		t.Error("Describe() sent no descriptors")
	}
}
