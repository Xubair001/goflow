package worker

import (
	"testing"
	"time"
)

func TestBackoff_WithinBounds(t *testing.T) {
	const base = 100 * time.Millisecond
	const max = 5 * time.Second

	tests := []struct {
		name      string
		attempt   int
		wantUpper time.Duration
	}{
		{"first attempt", 1, base},
		{"second attempt", 2, 2 * base},
		{"third attempt", 3, 4 * base},
		{"capped by max", 20, max},
		{"zero treated as one", 0, base},
		{"negative treated as one", -1, base},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 50 { // jitter is random; sample repeatedly
				d := Backoff(tt.attempt, base, max)
				if d < 0 || d > tt.wantUpper {
					t.Fatalf("Backoff(%d, %v, %v) = %v, want in [0, %v]", tt.attempt, base, max, d, tt.wantUpper)
				}
			}
		})
	}
}

func TestBackoff_ProducesJitter(t *testing.T) {
	const base = time.Second
	const max = time.Minute

	seen := make(map[time.Duration]bool)
	for range 20 {
		seen[Backoff(4, base, max)] = true
	}
	if len(seen) < 2 {
		t.Errorf("Backoff produced %d distinct values across 20 calls, want jitter to vary them", len(seen))
	}
}

func TestBackoff_ZeroBase(t *testing.T) {
	if d := Backoff(1, 0, time.Second); d != 0 {
		t.Errorf("Backoff(1, 0, 1s) = %v, want 0", d)
	}
}
