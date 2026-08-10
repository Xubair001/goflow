package job

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNew_Defaults(t *testing.T) {
	before := time.Now().UTC()
	j := New("send_email", []byte(`{"to":"a@example.com"}`))
	after := time.Now().UTC()

	if j.ID == uuid.Nil {
		t.Fatal("New() did not assign an ID")
	}
	if j.Type != "send_email" {
		t.Errorf("Type = %q, want %q", j.Type, "send_email")
	}
	if j.Status != StatusPending {
		t.Errorf("Status = %q, want %q", j.Status, StatusPending)
	}
	if j.MaxAttempts != DefaultMaxAttempts {
		t.Errorf("MaxAttempts = %d, want %d", j.MaxAttempts, DefaultMaxAttempts)
	}
	if j.RunAt.Before(before) || j.RunAt.After(after) {
		t.Errorf("RunAt = %v, want between %v and %v", j.RunAt, before, after)
	}
}

func TestNew_Options(t *testing.T) {
	runAt := time.Now().Add(24 * time.Hour)
	j := New("resize_image", nil,
		WithPriority(10),
		WithRunAt(runAt),
		WithMaxAttempts(1),
	)

	if j.Priority != 10 {
		t.Errorf("Priority = %d, want 10", j.Priority)
	}
	if !j.RunAt.Equal(runAt) {
		t.Errorf("RunAt = %v, want %v", j.RunAt, runAt)
	}
	if j.MaxAttempts != 1 {
		t.Errorf("MaxAttempts = %d, want 1", j.MaxAttempts)
	}
}

func TestStatus_Valid(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusPending, true},
		{StatusQueued, true},
		{StatusRunning, true},
		{StatusCompleted, true},
		{StatusDead, true},
		{StatusCancelled, true},
		{Status("bogus"), false},
		{Status(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.Valid(); got != tt.want {
				t.Errorf("Status(%q).Valid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatus_Terminal(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusPending, false},
		{StatusQueued, false},
		{StatusRunning, false},
		{StatusCompleted, true},
		{StatusDead, true},
		{StatusCancelled, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.Terminal(); got != tt.want {
				t.Errorf("Status(%q).Terminal() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestJob_ExhaustedRetries(t *testing.T) {
	tests := []struct {
		name        string
		attempts    int
		maxAttempts int
		want        bool
	}{
		{"below max", 2, 5, false},
		{"at max", 5, 5, true},
		{"above max", 6, 5, true},
		{"zero attempts", 0, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &Job{Attempts: tt.attempts, MaxAttempts: tt.maxAttempts}
			if got := j.ExhaustedRetries(); got != tt.want {
				t.Errorf("ExhaustedRetries() = %v, want %v", got, tt.want)
			}
		})
	}
}
