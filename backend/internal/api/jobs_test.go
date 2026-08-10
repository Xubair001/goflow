package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

func TestHandleCreateJob_Success(t *testing.T) {
	fs := &fakeStore{}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs", strings.NewReader(
		`{"type":"send_email","payload":{"to":"a@example.com"}}`,
	))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	var resp jobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Type != "send_email" {
		t.Errorf("Type = %q, want %q", resp.Type, "send_email")
	}
	if resp.Status != job.StatusPending {
		t.Errorf("Status = %q, want %q", resp.Status, job.StatusPending)
	}
	if len(fs.created) != 1 {
		t.Fatalf("Create called %d times, want 1", len(fs.created))
	}
}

func TestHandleCreateJob_MissingType(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusBadRequest, codeInvalidRequest)
}

func TestHandleCreateJob_UnknownType(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"type":"does_not_exist"}`))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusBadRequest, codeInvalidRequest)
}

func TestHandleCreateJob_MalformedJSON(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusBadRequest, codeInvalidRequest)
}

func TestHandleCreateJob_StoreError(t *testing.T) {
	fs := &fakeStore{createErr: errSimulated}
	srv := testServer(t, fs)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"type":"send_email"}`))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusInternalServerError, codeInternal)
}

func TestHandleListJobs_Success(t *testing.T) {
	j1 := job.New("send_email", json.RawMessage(`{}`))
	fs := &fakeStore{listResult: store.ListResult{Jobs: []*job.Job{j1}, Total: 1}}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/jobs?status=pending&limit=10&offset=5", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	var resp listJobsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Total != 1 || len(resp.Jobs) != 1 {
		t.Errorf("resp = %+v, want 1 job, total 1", resp)
	}
	if fs.lastFilter.Status == nil || *fs.lastFilter.Status != job.StatusPending {
		t.Errorf("filter.Status = %v, want pending", fs.lastFilter.Status)
	}
	if fs.lastFilter.Limit != 10 || fs.lastFilter.Offset != 5 {
		t.Errorf("filter = %+v, want Limit=10 Offset=5", fs.lastFilter)
	}
}

func TestHandleListJobs_InvalidStatus(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/jobs?status=bogus", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusBadRequest, codeInvalidRequest)
}

func TestHandleListJobs_InvalidLimit(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/jobs?limit=abc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusBadRequest, codeInvalidRequest)
}

func TestHandleGetJob_Success(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	fs := &fakeStore{getJob: j}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/jobs/"+j.ID.String(), nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestHandleGetJob_NotFound(t *testing.T) {
	fs := &fakeStore{getErr: store.ErrNotFound}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/jobs/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusNotFound, codeNotFound)
}

func TestHandleGetJob_InvalidID(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/jobs/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusBadRequest, codeInvalidRequest)
}

func TestHandleRetryJob_Success(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	fs := &fakeStore{reactivateJob: j}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs/"+j.ID.String()+"/retry", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestHandleRetryJob_NotEligible(t *testing.T) {
	fs := &fakeStore{reactivateErr: store.ErrNotFound}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs/"+uuid.NewString()+"/retry", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusNotFound, codeNotFound)
}

func TestHandleCancelJob_Success(t *testing.T) {
	j := job.New("send_email", json.RawMessage(`{}`))
	fs := &fakeStore{cancelJob: j}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs/"+j.ID.String()+"/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestHandleCancelJob_NotEligible(t *testing.T) {
	fs := &fakeStore{cancelErr: store.ErrNotFound}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/jobs/"+uuid.NewString()+"/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusNotFound, codeNotFound)
}

func TestHandleQueueStats(t *testing.T) {
	fs := &fakeStore{stats: store.Stats{Pending: 3, Running: 1}}
	srv := testServer(t, fs)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/queue/stats", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var stats store.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if stats.Pending != 3 || stats.Running != 1 {
		t.Errorf("stats = %+v, want Pending=3 Running=1", stats)
	}
}
