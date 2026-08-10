package config

import (
	"testing"
	"time"
)

func TestLoadAPIServer_Defaults(t *testing.T) {
	c, err := LoadAPIServer()
	if err != nil {
		t.Fatalf("LoadAPIServer() error = %v", err)
	}
	if c.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", c.ListenAddr, ":8080")
	}
	if c.EventsPollInterval != 2*time.Second {
		t.Errorf("EventsPollInterval = %v, want %v", c.EventsPollInterval, 2*time.Second)
	}
	if len(c.CORSOrigins) != 1 || c.CORSOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSOrigins = %v, want [http://localhost:5173]", c.CORSOrigins)
	}
}

func TestLoadAPIServer_MultipleCORSOrigins(t *testing.T) {
	t.Setenv("CORS_ORIGINS", "http://localhost:5173,https://app.example.com")
	c, err := LoadAPIServer()
	if err != nil {
		t.Fatalf("LoadAPIServer() error = %v", err)
	}
	want := []string{"http://localhost:5173", "https://app.example.com"}
	if len(c.CORSOrigins) != len(want) {
		t.Fatalf("CORSOrigins = %v, want %v", c.CORSOrigins, want)
	}
	for i, origin := range want {
		if c.CORSOrigins[i] != origin {
			t.Errorf("CORSOrigins[%d] = %q, want %q", i, c.CORSOrigins[i], origin)
		}
	}
}

func TestLoadAPIServer_InvalidDuration(t *testing.T) {
	t.Setenv("EVENTS_POLL_INTERVAL", "not-a-duration")
	if _, err := LoadAPIServer(); err == nil {
		t.Fatal("LoadAPIServer() error = nil, want an error for an invalid duration")
	}
}
