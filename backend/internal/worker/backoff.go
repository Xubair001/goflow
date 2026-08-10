package worker

import (
	"math/rand/v2"
	"time"
)

// Backoff computes an exponential-backoff-with-full-jitter delay for the
// attempt-th try (1-indexed: the attempt that just failed). Full jitter —
// a uniform random draw over [0, cap] rather than a fixed exp value — is
// what actually prevents a thundering herd: jobs that failed together
// don't all retry together.
func Backoff(attempt int, base, max time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	delay := base
	for i := 1; i < attempt && delay < max; i++ {
		delay *= 2
	}
	if delay > max {
		delay = max
	}
	if delay <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(delay) + 1))
}
