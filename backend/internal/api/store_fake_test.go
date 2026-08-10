package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// fakeStore is a minimal in-memory store.Store for testing the API's HTTP
// handlers. Only the methods the API actually calls (Create, Get, List,
// Cancel, Reactivate, Stats) have configurable behavior; the rest exist
// solely to satisfy the interface, since the API never calls them
// (ClaimDue/MarkRunning/Complete/Retry/Kill/ReclaimStale are the
// dispatcher's and worker's job).
type fakeStore struct {
	createErr error
	created   []*job.Job

	getJob *job.Job
	getErr error

	listResult store.ListResult
	listErr    error
	lastFilter store.ListFilter

	cancelJob *job.Job
	cancelErr error

	reactivateJob *job.Job
	reactivateErr error

	stats    store.Stats
	statsErr error
}

func (f *fakeStore) Create(_ context.Context, j *job.Job) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, j)
	return nil
}

func (f *fakeStore) Get(context.Context, uuid.UUID) (*job.Job, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getJob, nil
}

func (f *fakeStore) List(_ context.Context, filter store.ListFilter) (store.ListResult, error) {
	f.lastFilter = filter
	if f.listErr != nil {
		return store.ListResult{}, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeStore) Cancel(context.Context, uuid.UUID) (*job.Job, error) {
	if f.cancelErr != nil {
		return nil, f.cancelErr
	}
	return f.cancelJob, nil
}

func (f *fakeStore) Reactivate(context.Context, uuid.UUID) (*job.Job, error) {
	if f.reactivateErr != nil {
		return nil, f.reactivateErr
	}
	return f.reactivateJob, nil
}

func (f *fakeStore) Stats(context.Context) (store.Stats, error) {
	if f.statsErr != nil {
		return store.Stats{}, f.statsErr
	}
	return f.stats, nil
}

func (f *fakeStore) ClaimDue(context.Context, int) ([]*job.Job, error) { return nil, nil }

func (f *fakeStore) MarkRunning(context.Context, uuid.UUID, string) (*job.Job, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) Complete(context.Context, uuid.UUID, json.RawMessage) error { return nil }

func (f *fakeStore) Retry(context.Context, uuid.UUID, string, time.Time) error { return nil }

func (f *fakeStore) Kill(context.Context, uuid.UUID, string) error { return nil }

func (f *fakeStore) ReclaimStale(context.Context, time.Duration, int) ([]*job.Job, error) {
	return nil, nil
}

var _ store.Store = (*fakeStore)(nil)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }
