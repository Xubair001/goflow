package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPRequestHandler_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Test"); got != "yes" {
			t.Errorf("X-Test header = %q, want %q", got, "yes")
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer srv.Close()

	h := &HTTPRequestHandler{}
	payload := fmt.Sprintf(`{"url":%q,"method":"POST","headers":{"X-Test":"yes"},"body":"payload"}`, srv.URL)
	result, err := h.Execute(context.Background(), json.RawMessage(payload))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res HTTPRequestResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want %d", res.StatusCode, http.StatusCreated)
	}
	if res.Body != "created" {
		t.Errorf("Body = %q, want %q", res.Body, "created")
	}
}

func TestHTTPRequestHandler_Execute_DefaultsToGET(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
	}))
	defer srv.Close()

	h := &HTTPRequestHandler{}
	payload := fmt.Sprintf(`{"url":%q}`, srv.URL)
	if _, err := h.Execute(context.Background(), json.RawMessage(payload)); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
}

func TestHTTPRequestHandler_Execute_MissingURL(t *testing.T) {
	h := &HTTPRequestHandler{}
	if _, err := h.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("Execute() error = nil, want an error for missing url")
	}
}

func TestHTTPRequestHandler_Execute_TruncatesLargeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 100))
	}))
	defer srv.Close()

	h := &HTTPRequestHandler{MaxBytes: 10}
	payload := fmt.Sprintf(`{"url":%q}`, srv.URL)
	result, err := h.Execute(context.Background(), json.RawMessage(payload))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res HTTPRequestResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !res.Truncated {
		t.Error("Truncated = false, want true")
	}
	if len(res.Body) != 10 {
		t.Errorf("len(Body) = %d, want 10", len(res.Body))
	}
}
