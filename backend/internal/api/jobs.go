package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

const defaultListLimit = 50

// jobResponse is the wire representation of a job.Job.
type jobResponse struct {
	ID          uuid.UUID       `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Status      job.Status      `json:"status"`
	Priority    int             `json:"priority"`
	RunAt       time.Time       `json:"run_at"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	LastError   string          `json:"last_error,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func toJobResponse(j *job.Job) jobResponse {
	return jobResponse{
		ID:          j.ID,
		Type:        j.Type,
		Payload:     j.Payload,
		Status:      j.Status,
		Priority:    j.Priority,
		RunAt:       j.RunAt,
		Attempts:    j.Attempts,
		MaxAttempts: j.MaxAttempts,
		LastError:   j.LastError,
		Result:      j.Result,
		CreatedAt:   j.CreatedAt,
		UpdatedAt:   j.UpdatedAt,
	}
}

// createJobRequest is the POST /api/v1/jobs request body.
type createJobRequest struct {
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Priority    int             `json:"priority"`
	RunAt       *time.Time      `json:"run_at"`
	MaxAttempts int             `json:"max_attempts"`
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, "malformed JSON body")
		return
	}
	if req.Type == "" {
		writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, `"type" is required`)
		return
	}
	if _, err := s.registry.Lookup(req.Type); err != nil {
		writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, fmt.Sprintf("unknown job type %q", req.Type))
		return
	}
	if len(req.Payload) == 0 {
		req.Payload = json.RawMessage(`{}`)
	}

	opts := []job.Option{job.WithPriority(req.Priority)}
	if req.RunAt != nil {
		opts = append(opts, job.WithRunAt(*req.RunAt))
	}
	if req.MaxAttempts > 0 {
		opts = append(opts, job.WithMaxAttempts(req.MaxAttempts))
	}

	j := job.New(req.Type, req.Payload, opts...)
	if err := s.store.Create(r.Context(), j); err != nil {
		s.logger.Error("create job", "job_type", req.Type, "error", err)
		writeError(w, s.logger, http.StatusInternalServerError, codeInternal, "failed to create job")
		return
	}

	s.logger.Info("job created", "job_id", j.ID, "job_type", j.Type)
	writeJSON(w, s.logger, http.StatusCreated, toJobResponse(j))
}

// listJobsResponse is the GET /api/v1/jobs response body.
type listJobsResponse struct {
	Jobs   []jobResponse `json:"jobs"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var filter store.ListFilter
	if status := q.Get("status"); status != "" {
		st := job.Status(status)
		if !st.Valid() {
			writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, fmt.Sprintf("invalid status %q", status))
			return
		}
		filter.Status = &st
	}
	if jobType := q.Get("type"); jobType != "" {
		filter.Type = &jobType
	}

	limit, err := parseIntParam(q, "limit", defaultListLimit)
	if err != nil {
		writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, `invalid "limit"`)
		return
	}
	offset, err := parseIntParam(q, "offset", 0)
	if err != nil {
		writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, `invalid "offset"`)
		return
	}
	filter.Limit, filter.Offset = limit, offset

	result, err := s.store.List(r.Context(), filter)
	if err != nil {
		s.logger.Error("list jobs", "error", err)
		writeError(w, s.logger, http.StatusInternalServerError, codeInternal, "failed to list jobs")
		return
	}

	jobs := make([]jobResponse, len(result.Jobs))
	for i, j := range result.Jobs {
		jobs[i] = toJobResponse(j)
	}
	writeJSON(w, s.logger, http.StatusOK, listJobsResponse{
		Jobs: jobs, Total: result.Total, Limit: limit, Offset: offset,
	})
}

func parseIntParam(q url.Values, name string, def int) (int, error) {
	v := q.Get(name)
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}

func parseJobID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, "invalid job id")
		return
	}

	j, err := s.store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, s.logger, http.StatusNotFound, codeNotFound, "job not found")
		return
	}
	if err != nil {
		s.logger.Error("get job", "job_id", id, "error", err)
		writeError(w, s.logger, http.StatusInternalServerError, codeInternal, "failed to get job")
		return
	}
	writeJSON(w, s.logger, http.StatusOK, toJobResponse(j))
}

// handleRetryJob reactivates a dead or cancelled job. See Store.Reactivate
// for why this isn't the same as the worker's internal Retry.
func (s *Server) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, "invalid job id")
		return
	}

	j, err := s.store.Reactivate(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, s.logger, http.StatusNotFound, codeNotFound, "job not found or not eligible for retry")
		return
	}
	if err != nil {
		s.logger.Error("reactivate job", "job_id", id, "error", err)
		writeError(w, s.logger, http.StatusInternalServerError, codeInternal, "failed to retry job")
		return
	}
	s.logger.Info("job reactivated", "job_id", j.ID, "job_type", j.Type)
	writeJSON(w, s.logger, http.StatusOK, toJobResponse(j))
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, s.logger, http.StatusBadRequest, codeInvalidRequest, "invalid job id")
		return
	}

	j, err := s.store.Cancel(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, s.logger, http.StatusNotFound, codeNotFound, "job not found or not eligible for cancellation")
		return
	}
	if err != nil {
		s.logger.Error("cancel job", "job_id", id, "error", err)
		writeError(w, s.logger, http.StatusInternalServerError, codeInternal, "failed to cancel job")
		return
	}
	s.logger.Info("job cancelled", "job_id", j.ID, "job_type", j.Type)
	writeJSON(w, s.logger, http.StatusOK, toJobResponse(j))
}
