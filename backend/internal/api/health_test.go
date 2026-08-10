package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

func TestHandleHealthz(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleReadyz_AllHealthy(t *testing.T) {
	srv := NewServer(Config{
		Store: &fakeStore{}, Registry: job.NewRegistry(),
		DB: fakePinger{}, Redis: fakePinger{},
		Logger: testLogger(),
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleReadyz_DatabaseDown(t *testing.T) {
	srv := NewServer(Config{
		Store: &fakeStore{}, Registry: job.NewRegistry(),
		DB: fakePinger{err: errSimulated}, Redis: fakePinger{},
		Logger: testLogger(),
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleReadyz_RedisDown(t *testing.T) {
	srv := NewServer(Config{
		Store: &fakeStore{}, Registry: job.NewRegistry(),
		DB: fakePinger{}, Redis: fakePinger{err: errSimulated},
		Logger: testLogger(),
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
