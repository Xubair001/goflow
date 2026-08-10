package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

// errSimulated is a sentinel used across tests to simulate a store failure.
var errSimulated = errors.New("simulated store error")

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testServer builds a Server with a "send_email" type registered (so
// createJob validation has something valid to accept) against fs, with
// both dependency pings healthy.
func testServer(t *testing.T, fs *fakeStore) *Server {
	t.Helper()
	reg := job.NewRegistry()
	reg.Register("send_email", job.HandlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, nil
	}))
	return NewServer(Config{
		Store:    fs,
		Registry: reg,
		DB:       fakePinger{},
		Redis:    fakePinger{},
		Logger:   testLogger(),
	})
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, status, rec.Body)
	}
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v; body = %s", err, rec.Body)
	}
	if resp.Error.Code != code {
		t.Errorf("error code = %q, want %q", resp.Error.Code, code)
	}
}
