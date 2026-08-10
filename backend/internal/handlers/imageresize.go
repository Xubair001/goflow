package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoding
	_ "image/jpeg" // register JPEG decoding
	"image/png"
	"io"
	"net/http"

	"golang.org/x/image/draw"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

// ImageResizeJobType is the job.Registry key for ImageResizeHandler.
const ImageResizeJobType = "resize_image"

// defaultMaxImageBytes caps how much of a response body a resize job will
// read, so an oversized or malicious source_url can't exhaust worker memory.
const defaultMaxImageBytes = 20 << 20 // 20 MiB

// ImageResizePayload is the resize_image job's input.
type ImageResizePayload struct {
	SourceURL string `json:"source_url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// ImageResizeResult is the resize_image job's output: the resized image,
// re-encoded as PNG and base64-encoded. There's no object storage in this
// system yet, so returning the bytes inline is the simplest thing that
// works; swap this for an upload + a stored URL if payload size becomes a
// problem.
type ImageResizeResult struct {
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Format       string `json:"format"`
	ImageBase64  string `json:"image_base64"`
	OriginalSize int    `json:"original_size_bytes"`
	ResizedSize  int    `json:"resized_size_bytes"`
}

// ImageResizeHandler downloads an image and resizes it to an exact target
// size using bilinear interpolation.
//
// SourceURL is fetched as-is with no allowlist: this handler trusts whatever
// submits jobs to it. A deployment accepting job submissions from untrusted
// callers needs SSRF hardening (block internal/link-local addresses,
// allowlist schemes/hosts) before exposing this job type publicly.
type ImageResizeHandler struct {
	Client   *http.Client
	MaxBytes int64
}

var _ job.Handler = (*ImageResizeHandler)(nil)

// Execute implements job.Handler.
func (h *ImageResizeHandler) Execute(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var p ImageResizePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("handlers: decode image resize payload: %w", err)
	}
	if p.SourceURL == "" {
		return nil, errors.New("handlers: image resize payload missing \"source_url\"")
	}
	if p.Width <= 0 || p.Height <= 0 {
		return nil, errors.New("handlers: image resize width/height must be positive")
	}

	data, err := h.fetch(ctx, p.SourceURL)
	if err != nil {
		return nil, err
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("handlers: decode image: %w", err)
	}

	dst := image.NewRGBA(image.Rect(0, 0, p.Width, p.Height))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("handlers: encode resized image: %w", err)
	}

	return json.Marshal(ImageResizeResult{
		Width:        p.Width,
		Height:       p.Height,
		Format:       "png",
		ImageBase64:  base64.StdEncoding.EncodeToString(buf.Bytes()),
		OriginalSize: len(data),
		ResizedSize:  buf.Len(),
	})
}

func (h *ImageResizeHandler) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("handlers: build image request: %w", err)
	}

	resp, err := defaultClient(h.Client).Do(req)
	if err != nil {
		return nil, fmt.Errorf("handlers: fetch image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("handlers: fetch image: unexpected status %s", resp.Status)
	}

	limit := capOrDefault(h.MaxBytes, defaultMaxImageBytes)
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("handlers: read image: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("handlers: image exceeds %d byte limit", limit)
	}
	return data, nil
}
