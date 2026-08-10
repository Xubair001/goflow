package config

import (
	"testing"
	"time"
)

func TestLoadWorker_Defaults(t *testing.T) {
	c, err := LoadWorker()
	if err != nil {
		t.Fatalf("LoadWorker() error = %v", err)
	}
	if c.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want 10", c.Concurrency)
	}
	if c.ConsumeBatch != 10 {
		t.Errorf("ConsumeBatch = %d, want 10", c.ConsumeBatch)
	}
	if c.JobTimeout != 2*time.Minute {
		t.Errorf("JobTimeout = %v, want %v", c.JobTimeout, 2*time.Minute)
	}
	if c.ConsumeBlock != 5*time.Second {
		t.Errorf("ConsumeBlock = %v, want %v", c.ConsumeBlock, 5*time.Second)
	}
	if c.ConsumerName == "" {
		t.Error("ConsumerName default should not be empty")
	}
}

func TestLoadWorker_OverridesFromEnv(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "25")
	t.Setenv("CONSUMER_NAME", "worker-test-1")

	c, err := LoadWorker()
	if err != nil {
		t.Fatalf("LoadWorker() error = %v", err)
	}
	if c.Concurrency != 25 {
		t.Errorf("Concurrency = %d, want 25", c.Concurrency)
	}
	if c.ConsumerName != "worker-test-1" {
		t.Errorf("ConsumerName = %q, want %q", c.ConsumerName, "worker-test-1")
	}
}

func TestLoadWorker_InvalidConcurrency(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "not-a-number")
	if _, err := LoadWorker(); err == nil {
		t.Fatal("LoadWorker() error = nil, want an error for an invalid integer")
	}
}
