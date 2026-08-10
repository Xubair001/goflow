package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// fakeStore is a minimal in-memory store.Store for the handlers that depend
// on it (ReportHandler, ScheduledTaskHandler). Only Stats, List, and Create
// have real behavior; the rest exist solely to satisfy the interface.
type fakeStore struct {
	stats   store.Stats
	dead    []*job.Job
	created []*job.Job

	statsErr  error
	listErr   error
	createErr error
}

func (f *fakeStore) Stats(context.Context) (store.Stats, error) {
	if f.statsErr != nil {
		return store.Stats{}, f.statsErr
	}
	return f.stats, nil
}

func (f *fakeStore) List(_ context.Context, filter store.ListFilter) (store.ListResult, error) {
	if f.listErr != nil {
		return store.ListResult{}, f.listErr
	}
	jobs := f.dead
	if filter.Limit > 0 && filter.Limit < len(jobs) {
		jobs = jobs[:filter.Limit]
	}
	return store.ListResult{Jobs: jobs, Total: len(f.dead)}, nil
}

func (f *fakeStore) Create(_ context.Context, j *job.Job) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, j)
	return nil
}

func (f *fakeStore) Get(context.Context, uuid.UUID) (*job.Job, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) ClaimDue(context.Context, int) ([]*job.Job, error) { return nil, nil }

func (f *fakeStore) MarkRunning(context.Context, uuid.UUID, string) (*job.Job, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) Complete(context.Context, uuid.UUID, json.RawMessage) error { return nil }

func (f *fakeStore) Retry(context.Context, uuid.UUID, string, time.Time) error { return nil }

func (f *fakeStore) Kill(context.Context, uuid.UUID, string) error { return nil }

func (f *fakeStore) Cancel(context.Context, uuid.UUID) error { return nil }

func (f *fakeStore) ReclaimStale(context.Context, time.Duration, int) ([]*job.Job, error) {
	return nil, nil
}

var _ store.Store = (*fakeStore)(nil)
