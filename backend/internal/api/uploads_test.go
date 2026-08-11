package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// pngBytes returns a tiny valid PNG, for building multipart upload bodies.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func multipartUploadRequest(t *testing.T, fieldName, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/uploads", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestHandleUpload_Success(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	data := pngBytes(t)

	req := multipartUploadRequest(t, "file", "test.png", data)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	var resp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("ID should not be empty")
	}
	if resp.URL != "/api/v1/uploads/"+resp.ID {
		t.Errorf("URL = %q, want %q", resp.URL, "/api/v1/uploads/"+resp.ID)
	}

	// Fetch it back.
	getReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, resp.URL, nil)
	getRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getRec.Code, http.StatusOK)
	}
	if ct := getRec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want %q", ct, "image/png")
	}
	if !bytes.Equal(getRec.Body.Bytes(), data) {
		t.Error("fetched bytes do not match the uploaded bytes")
	}
}

func TestHandleUpload_MissingFileField(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := multipartUploadRequest(t, "not_file", "test.png", pngBytes(t))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusBadRequest, codeInvalidRequest)
}

func TestHandleUpload_RejectsNonImageContent(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := multipartUploadRequest(t, "file", "test.txt", []byte("just some plain text, not an image"))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusBadRequest, codeInvalidRequest)
}

func TestHandleGetUpload_NotFound(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/uploads/does-not-exist", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusNotFound, codeNotFound)
}
