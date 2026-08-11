package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// ReportJobType is the job.Registry key for ReportHandler.
const ReportJobType = "generate_report"

// defaultFailedSampleSize is how many recent dead jobs ReportHandler
// includes when the payload doesn't specify FailedSampleSize.
const defaultFailedSampleSize = 5

// ReportPayload is the generate_report job's input; an empty payload is
// valid and uses the default sample size. When EmailTo is set, the
// generated report is also emailed there via Mailer -- turning "view a
// report in the dashboard" into "get the report delivered", which is also
// what makes generate_report a meaningful ScheduledTaskHandler target.
type ReportPayload struct {
	FailedSampleSize int    `json:"failed_sample_size"`
	EmailTo          string `json:"email_to,omitempty"`
}

// DeadJobBrief summarizes one dead job for ReportResult.RecentDeadJobs.
type DeadJobBrief struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	LastError string    `json:"last_error"`
}

// ReportResult is the generate_report job's output. EmailedTo is set only
// when the payload requested delivery and it succeeded.
type ReportResult struct {
	GeneratedAt    time.Time      `json:"generated_at"`
	Stats          store.Stats    `json:"stats"`
	RecentDeadJobs []DeadJobBrief `json:"recent_dead_jobs"`
	Summary        string         `json:"summary"`
	EmailedTo      string         `json:"emailed_to,omitempty"`
}

// ReportHandler generates a point-in-time report on the job queue itself:
// counts by status plus a sample of recently dead jobs, which is the kind
// of thing worth running on a recurring schedule (see ScheduledTaskHandler).
// Mailer is optional -- leave it nil if send_email isn't configured; an
// EmailTo request without one fails with a clear error instead of silently
// no-op'ing.
type ReportHandler struct {
	Store  store.Store
	Mailer *Mailer
}

var _ job.Handler = (*ReportHandler)(nil)

// Execute implements job.Handler.
func (h *ReportHandler) Execute(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var p ReportPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, fmt.Errorf("handlers: decode report payload: %w", err)
		}
	}
	sampleSize := p.FailedSampleSize
	if sampleSize <= 0 {
		sampleSize = defaultFailedSampleSize
	}

	stats, err := h.Store.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("handlers: fetch queue stats: %w", err)
	}

	deadStatus := job.StatusDead
	deadJobs, err := h.Store.List(ctx, store.ListFilter{Status: &deadStatus, Limit: sampleSize})
	if err != nil {
		return nil, fmt.Errorf("handlers: list dead jobs: %w", err)
	}

	recent := make([]DeadJobBrief, len(deadJobs.Jobs))
	for i, j := range deadJobs.Jobs {
		recent[i] = DeadJobBrief{ID: j.ID, Type: j.Type, LastError: j.LastError}
	}

	generatedAt := time.Now().UTC()
	result := ReportResult{
		GeneratedAt:    generatedAt,
		Stats:          stats,
		RecentDeadJobs: recent,
		Summary: fmt.Sprintf(
			"Queue report @ %s: %d pending, %d queued, %d running, %d completed, %d dead, %d cancelled.",
			generatedAt.Format(time.RFC3339),
			stats.Pending, stats.Queued, stats.Running, stats.Completed, stats.Dead, stats.Cancelled,
		),
	}

	if p.EmailTo != "" {
		if h.Mailer == nil {
			return nil, errors.New("handlers: report requested email delivery but no SMTP mailer is configured")
		}
		if err := h.Mailer.Send(p.EmailTo, reportEmailSubject(generatedAt), reportEmailBody(result)); err != nil {
			return nil, fmt.Errorf("handlers: email report: %w", err)
		}
		result.EmailedTo = p.EmailTo
	}

	return json.Marshal(result)
}

func reportEmailSubject(generatedAt time.Time) string {
	return fmt.Sprintf("GoFlow queue report — %s", generatedAt.Format(time.RFC3339))
}

func reportEmailBody(r ReportResult) string {
	var b strings.Builder
	b.WriteString(r.Summary)
	if len(r.RecentDeadJobs) > 0 {
		b.WriteString("\n\nRecent dead jobs:\n")
		for _, j := range r.RecentDeadJobs {
			fmt.Fprintf(&b, "- %s (%s): %s\n", j.ID, j.Type, j.LastError)
		}
	}
	return b.String()
}
