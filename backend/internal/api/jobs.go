package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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

func (s *Server) handleCreateJob(c *gin.Context) {
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "malformed JSON body")
		return
	}
	if req.Type == "" {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, `"type" is required`)
		return
	}
	if _, err := s.registry.Lookup(req.Type); err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, fmt.Sprintf("unknown job type %q", req.Type))
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
	if err := s.store.Create(c.Request.Context(), j); err != nil {
		s.logger.Error("create job", "job_type", req.Type, "error", err)
		writeError(c, http.StatusInternalServerError, codeInternal, "failed to create job")
		return
	}

	s.logger.Info("job created", "job_id", j.ID, "job_type", j.Type)
	c.JSON(http.StatusCreated, toJobResponse(j))
}

// listJobsResponse is the GET /api/v1/jobs response body.
type listJobsResponse struct {
	Jobs   []jobResponse `json:"jobs"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

func (s *Server) handleListJobs(c *gin.Context) {
	var filter store.ListFilter
	if status := c.Query("status"); status != "" {
		st := job.Status(status)
		if !st.Valid() {
			writeError(c, http.StatusBadRequest, codeInvalidRequest, fmt.Sprintf("invalid status %q", status))
			return
		}
		filter.Status = &st
	}
	if jobType := c.Query("type"); jobType != "" {
		filter.Type = &jobType
	}

	limit, err := parseIntQuery(c, "limit", defaultListLimit)
	if err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, `invalid "limit"`)
		return
	}
	offset, err := parseIntQuery(c, "offset", 0)
	if err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, `invalid "offset"`)
		return
	}
	filter.Limit, filter.Offset = limit, offset

	result, err := s.store.List(c.Request.Context(), filter)
	if err != nil {
		s.logger.Error("list jobs", "error", err)
		writeError(c, http.StatusInternalServerError, codeInternal, "failed to list jobs")
		return
	}

	jobs := make([]jobResponse, len(result.Jobs))
	for i, j := range result.Jobs {
		jobs[i] = toJobResponse(j)
	}
	c.JSON(http.StatusOK, listJobsResponse{
		Jobs: jobs, Total: result.Total, Limit: limit, Offset: offset,
	})
}

func parseIntQuery(c *gin.Context, name string, def int) (int, error) {
	v := c.Query(name)
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}

func parseJobID(c *gin.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

func (s *Server) handleGetJob(c *gin.Context) {
	id, err := parseJobID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "invalid job id")
		return
	}

	j, err := s.store.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, codeNotFound, "job not found")
		return
	}
	if err != nil {
		s.logger.Error("get job", "job_id", id, "error", err)
		writeError(c, http.StatusInternalServerError, codeInternal, "failed to get job")
		return
	}
	c.JSON(http.StatusOK, toJobResponse(j))
}

// handleRetryJob reactivates a dead or cancelled job. See Store.Reactivate
// for why this isn't the same as the worker's internal Retry.
func (s *Server) handleRetryJob(c *gin.Context) {
	id, err := parseJobID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "invalid job id")
		return
	}

	j, err := s.store.Reactivate(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, codeNotFound, "job not found or not eligible for retry")
		return
	}
	if err != nil {
		s.logger.Error("reactivate job", "job_id", id, "error", err)
		writeError(c, http.StatusInternalServerError, codeInternal, "failed to retry job")
		return
	}
	s.logger.Info("job reactivated", "job_id", j.ID, "job_type", j.Type)
	c.JSON(http.StatusOK, toJobResponse(j))
}

func (s *Server) handleCancelJob(c *gin.Context) {
	id, err := parseJobID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "invalid job id")
		return
	}

	j, err := s.store.Cancel(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(c, http.StatusNotFound, codeNotFound, "job not found or not eligible for cancellation")
		return
	}
	if err != nil {
		s.logger.Error("cancel job", "job_id", id, "error", err)
		writeError(c, http.StatusInternalServerError, codeInternal, "failed to cancel job")
		return
	}
	s.logger.Info("job cancelled", "job_id", j.ID, "job_type", j.Type)
	c.JSON(http.StatusOK, toJobResponse(j))
}
