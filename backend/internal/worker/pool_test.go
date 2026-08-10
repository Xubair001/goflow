package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/queue"
	"github.com/abdullah-zubair/jobqueue/internal/store"
	"github.com/abdullah-zubair/jobqueue/internal/worker"
)

// storeCall records one method invocation on fakeStore, for assertions
// like "Retry was called but Kill wasn't" without a mocking library.
type storeCall struct {
	method string
}

// fakeStore is a minimal in-memory store.Store for unit-testing Pool.Process
// without a real Postgres instance.
type fakeStore struct {
	mu sync.Mutex

	jobs map[uuid.UUID]*job.Job

	markRunningErr error
	completeErr    error

	calls []storeCall
}

func newFakeStore(jobs ...*job.Job) *fakeStore {
	m := make(map[uuid.UUID]*job.Job, len(jobs))
	for _, j := range jobs {
		m[j.ID] = j
	}
	return &fakeStore{jobs: m}
}

func (f *fakeStore) MarkRunning(_ context.Context, id uuid.UUID, consumer string) (*job.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, storeCall{"MarkRunning"})
	if f.markRunningErr != nil {
		return nil, f.markRunningErr
	}
	j, ok := f.jobs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	j.Status = job.StatusRunning
	j.LockedBy = consumer
	j.Attempts++
	return j, nil
}

func (f *fakeStore) Complete(_ context.Context, id uuid.UUID, result json.RawMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, storeCall{"Complete"})
	if f.completeErr != nil {
		return f.completeErr
	}
	if j, ok := f.jobs[id]; ok {
		j.Status = job.StatusCompleted
		j.Result = result
	}
	return nil
}

func (f *fakeStore) Retry(_ context.Context, id uuid.UUID, lastErr string, nextRunAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, storeCall{"Retry"})
	if j, ok := f.jobs[id]; ok {
		j.Status = job.StatusPending
		j.LastError = lastErr
		j.RunAt = nextRunAt
	}
	return nil
}

func (f *fakeStore) Kill(_ context.Context, id uuid.UUID, lastErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, storeCall{"Kill"})
	if j, ok := f.jobs[id]; ok {
		j.Status = job.StatusDead
		j.LastError = lastErr
	}
	return nil
}

func (f *fakeStore) calledMethods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	methods := make([]string, len(f.calls))
	for i, c := range f.calls {
		methods[i] = c.method
	}
	return methods
}

func (f *fakeStore) hasCall(method string) bool {
	for _, m := range f.calledMethods() {
		if m == method {
			return true
		}
	}
	return false
}

func (f *fakeStore) Create(context.Context, *job.Job) error { return nil }

func (f *fakeStore) Get(context.Context, uuid.UUID) (*job.Job, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) List(context.Context, store.ListFilter) (store.ListResult, error) {
	return store.ListResult{}, nil
}

func (f *fakeStore) ClaimDue(context.Context, int) ([]*job.Job, error) { return nil, nil }

func (f *fakeStore) Cancel(context.Context, uuid.UUID) error { return nil }

func (f *fakeStore) ReclaimStale(context.Context, time.Duration, int) ([]*job.Job, error) {
	return nil, nil
}

func (f *fakeStore) Stats(context.Context) (store.Stats, error) { return store.Stats{}, nil }

var _ store.Store = (*fakeStore)(nil)

// fakeQueue is a minimal in-memory queue.Queue recording acknowledgments,
// with optional hooks so Run-level tests can control what Consume returns.
type fakeQueue struct {
	mu           sync.Mutex
	acked        []queue.Message
	consumeDelay time.Duration
	consumeFn    func() []queue.Message
}

func (f *fakeQueue) Ack(_ context.Context, msg queue.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acked = append(f.acked, msg)
	return nil
}

func (f *fakeQueue) ackedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, len(f.acked))
	for i, m := range f.acked {
		ids[i] = m.ID
	}
	return ids
}

func (f *fakeQueue) Publish(context.Context, string) error { return nil }

func (f *fakeQueue) Consume(ctx context.Context, _ string, _ int64) ([]queue.Message, error) {
	if f.consumeDelay > 0 {
		select {
		case <-time.After(f.consumeDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.consumeFn != nil {
		return f.consumeFn(), nil
	}
	return nil, nil
}

func (f *fakeQueue) Reclaim(context.Context, string, time.Duration, int64) ([]queue.Message, error) {
	return nil, nil
}

var _ queue.Queue = (*fakeQueue)(nil)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testConfig() worker.Config {
	return worker.Config{
		Concurrency:     1,
		ConsumeBatch:    1,
		ReclaimInterval: time.Hour,
		ReclaimMinIdle:  time.Minute,
		JobTimeout:      time.Second,
		RetryBaseDelay:  10 * time.Millisecond,
		RetryMaxDelay:   time.Second,
	}
}

func handlerFunc(fn func(context.Context, json.RawMessage) (json.RawMessage, error)) job.Handler {
	return job.HandlerFunc(fn)
}

func TestPool_Process_Success(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	j.MaxAttempts = 3
	fs := newFakeStore(j)
	fq := &fakeQueue{}
	registry := job.NewRegistry()
	registry.Register("send_email", handlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}))

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())
	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: j.ID.String()})

	if !fs.hasCall("Complete") {
		t.Error("Complete was not called")
	}
	if fs.hasCall("Retry") || fs.hasCall("Kill") {
		t.Error("Retry/Kill should not be called on success")
	}
	if ids := fq.ackedIDs(); len(ids) != 1 || ids[0] != "1-0" {
		t.Errorf("acked = %v, want [1-0]", ids)
	}
}

func TestPool_Process_HandlerError_RetriesWhenAttemptsRemain(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	j.MaxAttempts = 3
	fs := newFakeStore(j)
	fq := &fakeQueue{}
	registry := job.NewRegistry()
	registry.Register("send_email", handlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("smtp timeout")
	}))

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())
	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: j.ID.String()})

	if !fs.hasCall("Retry") {
		t.Error("Retry was not called")
	}
	if fs.hasCall("Kill") {
		t.Error("Kill should not be called while attempts remain")
	}
	if len(fq.ackedIDs()) != 1 {
		t.Error("message should be acked after recording a retry")
	}
}

func TestPool_Process_HandlerError_KillsWhenExhausted(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	j.MaxAttempts = 1
	fs := newFakeStore(j)
	fq := &fakeQueue{}
	registry := job.NewRegistry()
	registry.Register("send_email", handlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("permanent failure")
	}))

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())
	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: j.ID.String()})

	if !fs.hasCall("Kill") {
		t.Error("Kill was not called after exhausting attempts")
	}
	if fs.hasCall("Retry") {
		t.Error("Retry should not be called once attempts are exhausted")
	}
}

func TestPool_Process_HandlerPanic_TreatedAsFailure(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	j.MaxAttempts = 3
	fs := newFakeStore(j)
	fq := &fakeQueue{}
	registry := job.NewRegistry()
	registry.Register("send_email", handlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		panic("boom")
	}))

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())

	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: j.ID.String()}) // must not crash the test

	if !fs.hasCall("Retry") {
		t.Error("a panicking handler should be treated as a failed attempt (Retry called)")
	}
}

func TestPool_Process_NoHandlerRegistered_KillsJob(t *testing.T) {
	j := job.New("unknown_type", json.RawMessage(`{}`))
	fs := newFakeStore(j)
	fq := &fakeQueue{}
	registry := job.NewRegistry() // nothing registered

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())
	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: j.ID.String()})

	if !fs.hasCall("Kill") {
		t.Error("Kill was not called for an unregistered job type")
	}
	if len(fq.ackedIDs()) != 1 {
		t.Error("message should be acked after killing the job")
	}
}

func TestPool_Process_MarkRunningNotFound_AcksWithoutFurtherWrites(t *testing.T) {
	fs := newFakeStore() // no jobs -- MarkRunning returns store.ErrNotFound
	fq := &fakeQueue{}
	registry := job.NewRegistry()

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())
	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: uuid.NewString()})

	if calls := fs.calledMethods(); len(calls) != 1 || calls[0] != "MarkRunning" {
		t.Errorf("calls = %v, want only [MarkRunning]", calls)
	}
	if len(fq.ackedIDs()) != 1 {
		t.Error("message should still be acked when the job is gone")
	}
}

func TestPool_Process_MarkRunningError_DoesNotAck(t *testing.T) {
	fs := newFakeStore()
	fs.markRunningErr = errors.New("db down")
	fq := &fakeQueue{}
	registry := job.NewRegistry()

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())
	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: uuid.NewString()})

	if len(fq.ackedIDs()) != 0 {
		t.Error("message should not be acked when MarkRunning fails for a reason other than not-found")
	}
}

func TestPool_Process_CompleteError_DoesNotAck(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	fs := newFakeStore(j)
	fs.completeErr = errors.New("db down")
	fq := &fakeQueue{}
	registry := job.NewRegistry()
	registry.Register("send_email", handlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	}))

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())
	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: j.ID.String()})

	if len(fq.ackedIDs()) != 0 {
		t.Error("message should not be acked when Complete fails")
	}
}

func TestPool_Process_MalformedJobID_AcksAndDrops(t *testing.T) {
	fs := newFakeStore()
	fq := &fakeQueue{}
	registry := job.NewRegistry()

	p := worker.New(fs, fq, registry, "worker-1", testConfig(), testLogger())
	p.Process(context.Background(), queue.Message{ID: "1-0", JobID: "not-a-uuid"})

	if calls := fs.calledMethods(); len(calls) != 0 {
		t.Errorf("store should not be called for a malformed job id, got %v", calls)
	}
	if len(fq.ackedIDs()) != 1 {
		t.Error("a malformed message should still be acked to avoid an infinite redelivery loop")
	}
}

func TestPool_Run_StopsOnContextCancel(t *testing.T) {
	fq := &fakeQueue{consumeDelay: 5 * time.Millisecond}
	cfg := testConfig()
	cfg.ReclaimInterval = 10 * time.Millisecond

	p := worker.New(newFakeStore(), fq, job.NewRegistry(), "worker-1", cfg, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

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

// TestPool_Run_ProcessesDeliveredMessage wires the whole pool together
// (fetch loop -> channel -> worker goroutine -> Process) against fakes, to
// prove Run's plumbing actually delivers a message end to end rather than
// just testing Process in isolation.
func TestPool_Run_ProcessesDeliveredMessage(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	fs := newFakeStore(j)

	var delivered atomic.Bool
	fq := &fakeQueue{consumeDelay: 5 * time.Millisecond}
	fq.consumeFn = func() []queue.Message {
		if delivered.CompareAndSwap(false, true) {
			return []queue.Message{{ID: "1-0", JobID: j.ID.String()}}
		}
		return nil
	}

	handled := make(chan struct{})
	registry := job.NewRegistry()
	registry.Register("send_email", handlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		close(handled)
		return json.RawMessage(`{}`), nil
	}))

	cfg := testConfig()
	cfg.ReclaimInterval = time.Hour
	p := worker.New(fs, fq, registry, "worker-1", cfg, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- p.Run(ctx) }()

	select {
	case <-handled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not invoked within 2s")
	}

	cancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Errorf("Run() error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after cancel")
	}

	if !fs.hasCall("Complete") {
		t.Error("Complete was not called")
	}
}
