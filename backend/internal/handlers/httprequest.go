package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

// HTTPRequestJobType is the job.Registry key for HTTPRequestHandler.
const HTTPRequestJobType = "make_http_request"

// defaultMaxResponseBytes caps how much of a response body is read/returned.
const defaultMaxResponseBytes = 1 << 20 // 1 MiB

// HTTPRequestPayload is the make_http_request job's input. Method defaults
// to GET when empty.
type HTTPRequestPayload struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// HTTPRequestResult is the make_http_request job's output. Body is
// truncated to MaxBytes; Truncated reports whether that happened.
type HTTPRequestResult struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
	Truncated  bool   `json:"truncated"`
}

// HTTPRequestHandler makes an outbound HTTP request and reports the result.
//
// Like ImageResizeHandler, URL is fetched with no allowlist: a deployment
// accepting job submissions from untrusted callers needs SSRF hardening
// before exposing this job type publicly.
type HTTPRequestHandler struct {
	Client   *http.Client
	MaxBytes int64
}

var _ job.Handler = (*HTTPRequestHandler)(nil)

// Execute implements job.Handler.
func (h *HTTPRequestHandler) Execute(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var p HTTPRequestPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("handlers: decode http request payload: %w", err)
	}
	if p.URL == "" {
		return nil, errors.New("handlers: http request payload missing \"url\"")
	}
	method := p.Method
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if p.Body != "" {
		body = strings.NewReader(p.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.URL, body)
	if err != nil {
		return nil, fmt.Errorf("handlers: build http request: %w", err)
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	resp, err := defaultClient(h.Client).Do(req)
	if err != nil {
		return nil, fmt.Errorf("handlers: do http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	limit := capOrDefault(h.MaxBytes, defaultMaxResponseBytes)
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("handlers: read http response: %w", err)
	}
	truncated := int64(len(data)) > limit
	if truncated {
		data = data[:limit]
	}

	return json.Marshal(HTTPRequestResult{
		StatusCode: resp.StatusCode,
		Body:       string(data),
		Truncated:  truncated,
	})
}
