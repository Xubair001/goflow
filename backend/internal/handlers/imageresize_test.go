package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestImageResizeHandler_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 100, 50)))
	}))
	defer srv.Close()

	h := &ImageResizeHandler{}
	payload := fmt.Sprintf(`{"source_url":%q,"width":10,"height":10}`, srv.URL)
	result, err := h.Execute(context.Background(), json.RawMessage(payload))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res ImageResizeResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.Width != 10 || res.Height != 10 {
		t.Errorf("dimensions = %dx%d, want 10x10", res.Width, res.Height)
	}

	decoded, err := base64.StdEncoding.DecodeString(res.ImageBase64)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	out, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("decode resized image: %v", err)
	}
	if out.Bounds().Dx() != 10 || out.Bounds().Dy() != 10 {
		t.Errorf("resized image bounds = %v, want 10x10", out.Bounds())
	}
}

func TestImageResizeHandler_Execute_MissingSource(t *testing.T) {
	h := &ImageResizeHandler{}
	_, err := h.Execute(context.Background(), json.RawMessage(`{"width":10,"height":10}`))
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for missing source_url/upload_id")
	}
}

func TestImageResizeHandler_Execute_UploadID_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/uploads/abc123" {
			t.Errorf("fetched path = %q, want /api/v1/uploads/abc123", r.URL.Path)
		}
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 100, 50)))
	}))
	defer srv.Close()

	h := &ImageResizeHandler{UploadBaseURL: srv.URL}
	payload := json.RawMessage(`{"upload_id":"abc123","width":20,"height":20}`)
	result, err := h.Execute(context.Background(), payload)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res ImageResizeResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.Width != 20 || res.Height != 20 {
		t.Errorf("dimensions = %dx%d, want 20x20", res.Width, res.Height)
	}
}

func TestImageResizeHandler_Execute_UploadID_PrioritizedOverSourceURL(t *testing.T) {
	var fetchedFromUpload bool
	uploadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetchedFromUpload = true
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	}))
	defer uploadSrv.Close()

	sourceURLSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("source_url should not be fetched when upload_id is present")
	}))
	defer sourceURLSrv.Close()

	h := &ImageResizeHandler{UploadBaseURL: uploadSrv.URL}
	payload := fmt.Sprintf(`{"upload_id":"abc123","source_url":%q,"width":10,"height":10}`, sourceURLSrv.URL)
	if _, err := h.Execute(context.Background(), json.RawMessage(payload)); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !fetchedFromUpload {
		t.Error("expected the upload to be fetched, but it wasn't")
	}
}

func TestImageResizeHandler_Execute_UploadIDWithoutBaseURL(t *testing.T) {
	h := &ImageResizeHandler{} // UploadBaseURL unset
	payload := json.RawMessage(`{"upload_id":"abc123","width":10,"height":10}`)
	if _, err := h.Execute(context.Background(), payload); err == nil {
		t.Fatal("Execute() error = nil, want an error when upload_id is given but UploadBaseURL is unset")
	}
}

func TestImageResizeHandler_Execute_InvalidDimensions(t *testing.T) {
	h := &ImageResizeHandler{}
	payload := json.RawMessage(`{"source_url":"http://example.com/x.png","width":0,"height":10}`)
	if _, err := h.Execute(context.Background(), payload); err == nil {
		t.Fatal("Execute() error = nil, want an error for non-positive width")
	}
}

func TestImageResizeHandler_Execute_FetchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	h := &ImageResizeHandler{}
	payload := fmt.Sprintf(`{"source_url":%q,"width":10,"height":10}`, srv.URL)
	if _, err := h.Execute(context.Background(), json.RawMessage(payload)); err == nil {
		t.Fatal("Execute() error = nil, want an error for a 404 response")
	}
}

func TestImageResizeHandler_Execute_ExceedsSizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 100))
	}))
	defer srv.Close()

	h := &ImageResizeHandler{MaxBytes: 10}
	payload := fmt.Sprintf(`{"source_url":%q,"width":10,"height":10}`, srv.URL)
	if _, err := h.Execute(context.Background(), json.RawMessage(payload)); err == nil {
		t.Fatal("Execute() error = nil, want an error for exceeding the byte limit")
	}
}
