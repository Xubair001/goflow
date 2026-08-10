package dispatcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/abdullah-zubair/jobqueue/internal/dispatcher"
	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/metrics"
	"github.com/abdullah-zubair/jobqueue/internal/queue"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// fakeStore is a minimal in-memory store.Store for unit-testing the
// dispatcher without a real Postgres instance. Only ClaimDue and
// ReclaimStale have real behavior; the rest exist solely to satisfy the
// interface.
type fakeStore struct {
	mu       sync.Mutex
	due      []*job.Job
	stale    []*job.Job
	claimErr error
	staleErr error
}

func (f *fakeStore) ClaimDue(_ context.Context, limit int) ([]*job.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	n := min(limit, len(f.due))
	claimed := f.due[:n]
	f.due = f.due[n:]
	return claimed, nil
}

func (f *fakeStore) ReclaimStale(_ context.Context, _ time.Duration, limit int) ([]*job.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.staleErr != nil {
		return nil, f.staleErr
	}
	n := min(limit, len(f.stale))
	reclaimed := f.stale[:n]
	f.stale = f.stale[n:]
	return reclaimed, nil
}

func (f *fakeStore) remainingStale() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.stale)
}

func (f *fakeStore) Create(context.Context, *job.Job) error { return nil }

func (f *fakeStore) Get(context.Context, uuid.UUID) (*job.Job, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) List(context.Context, store.ListFilter) (store.ListResult, error) {
	return store.ListResult{}, nil
}

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

func (f *fakeStore) Stats(context.Context) (store.Stats, error) { return store.Stats{}, nil }

var _ store.Store = (*fakeStore)(nil)

// fakeQueue is a minimal in-memory queue.Queue recording published job IDs.
type fakeQueue struct {
	mu         sync.Mutex
	published  []string
	publishErr error
}

func (f *fakeQueue) Publish(_ context.Context, jobID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishErr != nil {
		return f.publishErr
	}
	f.published = append(f.published, jobID)
	return nil
}

func (f *fakeQueue) publishedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.published...)
}

func (f *fakeQueue) Consume(context.Context, string, int64) ([]queue.Message, error) {
	return nil, nil
}

func (f *fakeQueue) Ack(context.Context, queue.Message) error { return nil }

func (f *fakeQueue) Reclaim(context.Context, string, time.Duration, int64) ([]queue.Message, error) {
	return nil, nil
}

var _ queue.Queue = (*fakeQueue)(nil)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newJob() *job.Job {
	return job.New("send_email", json.RawMessage(`{}`))
}

func TestDispatcher_DispatchOnce_PublishesDueJobs(t *testing.T) {
	j1, j2 := newJob(), newJob()
	fs := &fakeStore{due: []*job.Job{j1, j2}}
	fq := &fakeQueue{}
	d := dispatcher.New(fs, fq, dispatcher.Config{BatchSize: 10}, testLogger())

	d.DispatchOnce(context.Background())

	published := fq.publishedIDs()
	if len(published) != 2 {
		t.Fatalf("published %d jobs, want 2", len(published))
	}
	want := map[string]bool{j1.ID.String(): true, j2.ID.String(): true}
	for _, id := range published {
		if !want[id] {
			t.Errorf("published unexpected job id %s", id)
		}
	}
}

func TestDispatcher_DispatchOnce_ClaimDueError_DoesNotPublish(t *testing.T) {
	fs := &fakeStore{claimErr: errors.New("db down")}
	fq := &fakeQueue{}
	d := dispatcher.New(fs, fq, dispatcher.Config{BatchSize: 10}, testLogger())

	d.DispatchOnce(context.Background())

	if len(fq.publishedIDs()) != 0 {
		t.Errorf("published %d jobs after ClaimDue error, want 0", len(fq.publishedIDs()))
	}
}

func TestDispatcher_DispatchOnce_PublishError_SkipsRemainingSilently(t *testing.T) {
	fs := &fakeStore{due: []*job.Job{newJob(), newJob()}}
	fq := &fakeQueue{publishErr: errors.New("redis down")}
	d := dispatcher.New(fs, fq, dispatcher.Config{BatchSize: 10}, testLogger())

	// Must not panic despite every publish failing.
	d.DispatchOnce(context.Background())

	if len(fq.publishedIDs()) != 0 {
		t.Errorf("published %d jobs despite publish errors, want 0", len(fq.publishedIDs()))
	}
}

func TestDispatcher_ReconcileOnce_ResetsStaleJobs(t *testing.T) {
	fs := &fakeStore{stale: []*job.Job{newJob(), newJob()}}
	fq := &fakeQueue{}
	d := dispatcher.New(fs, fq, dispatcher.Config{BatchSize: 10, StaleAfter: 5 * time.Minute}, testLogger())

	d.ReconcileOnce(context.Background())

	if remaining := fs.remainingStale(); remaining != 0 {
		t.Errorf("fakeStore has %d stale jobs left after reconcile, want 0", remaining)
	}
}

func TestDispatcher_ReconcileOnce_ReclaimStaleError_DoesNotPanic(t *testing.T) {
	fs := &fakeStore{staleErr: errors.New("db down")}
	d := dispatcher.New(fs, &fakeQueue{}, dispatcher.Config{BatchSize: 10}, testLogger())

	d.ReconcileOnce(context.Background())
}

func TestDispatcher_Run_StopsOnContextCancel(t *testing.T) {
	d := dispatcher.New(&fakeStore{}, &fakeQueue{}, dispatcher.Config{
		PollInterval:      10 * time.Millisecond,
		ReconcileInterval: 10 * time.Millisecond,
		BatchSize:         10,
		StaleAfter:        time.Minute,
	}, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	time.Sleep(30 * time.Millisecond) // let a couple of ticks happen
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return within 2s of context cancellation")
	}
}

func TestDispatcher_DispatchOnce_RecordsDispatchedMetric(t *testing.T) {
	j := newJob()
	fs := &fakeStore{due: []*job.Job{j}}
	fq := &fakeQueue{}
	d := dispatcher.New(fs, fq, dispatcher.Config{BatchSize: 10}, testLogger())

	before := testutil.ToFloat64(metrics.JobsDispatched.WithLabelValues(j.Type))
	d.DispatchOnce(context.Background())
	after := testutil.ToFloat64(metrics.JobsDispatched.WithLabelValues(j.Type))

	if after-before != 1 {
		t.Errorf("JobsDispatched delta = %v, want 1", after-before)
	}
}

func TestDispatcher_ReconcileOnce_RecordsReconciledMetric(t *testing.T) {
	j := newJob()
	fs := &fakeStore{stale: []*job.Job{j}}
	d := dispatcher.New(fs, &fakeQueue{}, dispatcher.Config{BatchSize: 10, StaleAfter: 5 * time.Minute}, testLogger())

	before := testutil.ToFloat64(metrics.JobsReconciled.WithLabelValues(j.Type))
	d.ReconcileOnce(context.Background())
	after := testutil.ToFloat64(metrics.JobsReconciled.WithLabelValues(j.Type))

	if after-before != 1 {
		t.Errorf("JobsReconciled delta = %v, want 1", after-before)
	}
}
