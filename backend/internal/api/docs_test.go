package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleOpenAPISpec(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "openapi:") {
		t.Error("response does not look like an OpenAPI spec")
	}
}

func TestHandleSwaggerUI(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "swagger-ui") {
		t.Error("response does not look like the Swagger UI page")
	}
}
