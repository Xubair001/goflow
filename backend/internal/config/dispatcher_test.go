package config

import (
	"testing"
	"time"
)

func TestLoadDispatcher_Defaults(t *testing.T) {
	c, err := LoadDispatcher()
	if err != nil {
		t.Fatalf("LoadDispatcher() error = %v", err)
	}
	if c.PollInterval != time.Second {
		t.Errorf("PollInterval = %v, want %v", c.PollInterval, time.Second)
	}
	if c.ReconcileInterval != 30*time.Second {
		t.Errorf("ReconcileInterval = %v, want %v", c.ReconcileInterval, 30*time.Second)
	}
	if c.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", c.BatchSize)
	}
	if c.StaleAfter != 5*time.Minute {
		t.Errorf("StaleAfter = %v, want %v", c.StaleAfter, 5*time.Minute)
	}
	if c.ConsumerGroup != "workers" {
		t.Errorf("ConsumerGroup = %q, want %q", c.ConsumerGroup, "workers")
	}
	if c.MetricsAddr != ":9092" {
		t.Errorf("MetricsAddr = %q, want %q", c.MetricsAddr, ":9092")
	}
}

func TestLoadDispatcher_OverridesFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom/db")
	t.Setenv("DISPATCH_POLL_INTERVAL", "5s")
	t.Setenv("DISPATCH_BATCH_SIZE", "100")

	c, err := LoadDispatcher()
	if err != nil {
		t.Fatalf("LoadDispatcher() error = %v", err)
	}
	if c.DatabaseURL != "postgres://custom/db" {
		t.Errorf("DatabaseURL = %q, want %q", c.DatabaseURL, "postgres://custom/db")
	}
	if c.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want %v", c.PollInterval, 5*time.Second)
	}
	if c.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", c.BatchSize)
	}
}

func TestLoadDispatcher_InvalidDuration(t *testing.T) {
	t.Setenv("DISPATCH_POLL_INTERVAL", "not-a-duration")
	if _, err := LoadDispatcher(); err == nil {
		t.Fatal("LoadDispatcher() error = nil, want an error for an invalid duration")
	}
}

func TestLoadDispatcher_InvalidInt(t *testing.T) {
	t.Setenv("DISPATCH_BATCH_SIZE", "not-a-number")
	if _, err := LoadDispatcher(); err == nil {
		t.Fatal("LoadDispatcher() error = nil, want an error for an invalid integer")
	}
}
