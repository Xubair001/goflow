package api

import (
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxUploadBytes bounds a single upload, read before any of it is buffered
// in memory.
const maxUploadBytes = 10 << 20 // 10 MiB

// uploadStore holds uploaded files in memory, keyed by a generated ID.
// There's no object storage in this system (see ImageResizeHandler's own
// doc comment), so uploads are lost on an apiserver restart -- an
// acceptable limitation for a demo upload path feeding a job that
// typically runs within seconds of the upload, not a durable file store.
type uploadStore struct {
	mu    sync.RWMutex
	files map[string]uploadedFile
}

type uploadedFile struct {
	data        []byte
	contentType string
}

func newUploadStore() *uploadStore {
	return &uploadStore{files: make(map[string]uploadedFile)}
}

func (s *uploadStore) put(data []byte, contentType string) string {
	id := uuid.NewString()
	s.mu.Lock()
	s.files[id] = uploadedFile{data: data, contentType: contentType}
	s.mu.Unlock()
	return id
}

func (s *uploadStore) get(id string) (uploadedFile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.files[id]
	return f, ok
}

// uploadResponse is the POST /api/v1/uploads response body.
type uploadResponse struct {
	ID string `json:"id"`
	// URL is relative to this server; the browser resolves it against the
	// page origin. Server-to-server fetches (a worker downloading it to
	// resize) need an absolute URL instead -- see
	// config.Worker.APIServerURL / handlers.Deps.UploadBaseURL, since a
	// worker in its own container can't resolve "localhost" the way the
	// browser that uploaded the file did.
	URL string `json:"url"`
}

func (s *Server) handleUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
	if err := c.Request.ParseMultipartForm(maxUploadBytes); err != nil { //nolint:gosec // bounded above via http.MaxBytesReader
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "upload too large or malformed")
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, `missing "file" field`)
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "failed to read upload")
		return
	}

	// Sniffed from content, not trusted from the client-supplied filename
	// or form field: http.DetectContentType looks at the actual bytes.
	contentType := http.DetectContentType(data)
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/gif" && contentType != "image/webp" {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "only PNG, JPEG, GIF, or WebP images are supported")
		return
	}

	id := s.uploads.put(data, contentType)
	s.logger.Info("file uploaded", "upload_id", id, "content_type", contentType, "bytes", len(data))
	c.JSON(http.StatusCreated, uploadResponse{ID: id, URL: "/api/v1/uploads/" + id})
}

func (s *Server) handleGetUpload(c *gin.Context) {
	id := c.Param("id")
	f, ok := s.uploads.get(id)
	if !ok {
		writeError(c, http.StatusNotFound, codeNotFound, "upload not found")
		return
	}
	c.Header("Content-Type", f.contentType)
	c.Header("Cache-Control", "private, max-age=3600")
	if _, err := c.Writer.Write(f.data); err != nil {
		s.logger.Error("write upload response", "upload_id", id, "error", err)
	}
}
