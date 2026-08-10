package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

func TestRoutes_ServesStaticDashboard(t *testing.T) {
	reg := job.NewRegistry()
	srv := NewServer(Config{
		Store:    &fakeStore{},
		Registry: reg,
		DB:       fakePinger{},
		Redis:    fakePinger{},
		Logger:   testLogger(),
		StaticFS: fstest.MapFS{
			"index.html": {Data: []byte("<html>dashboard</html>")},
		},
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "<html>dashboard</html>" {
		t.Errorf("body = %q, want the embedded index.html contents", rec.Body.String())
	}
}

func TestRoutes_NoStaticFS_APIStillWorks(t *testing.T) {
	srv := testServer(t, &fakeStore{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
