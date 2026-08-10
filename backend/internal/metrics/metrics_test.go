package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// These metrics are package-level (promauto registers them once against the
// default registry), so tests assert on the delta a call produces rather
// than an absolute value -- safe regardless of test order or what other
// tests in this package have already recorded.

func TestJobsDispatched_IncrementsPerJobType(t *testing.T) {
	before := testutil.ToFloat64(JobsDispatched.WithLabelValues("send_email"))
	JobsDispatched.WithLabelValues("send_email").Inc()
	after := testutil.ToFloat64(JobsDispatched.WithLabelValues("send_email"))

	if after-before != 1 {
		t.Errorf("JobsDispatched delta = %v, want 1", after-before)
	}
}

func TestJobsCompletedRetriedDead_AreIndependentPerJobType(t *testing.T) {
	beforeCompleted := testutil.ToFloat64(JobsCompleted.WithLabelValues("process_csv"))
	beforeRetried := testutil.ToFloat64(JobsRetried.WithLabelValues("process_csv"))
	beforeDead := testutil.ToFloat64(JobsDead.WithLabelValues("process_csv"))

	JobsCompleted.WithLabelValues("process_csv").Inc()
	JobsRetried.WithLabelValues("process_csv").Inc()
	JobsRetried.WithLabelValues("process_csv").Inc()

	if got := testutil.ToFloat64(JobsCompleted.WithLabelValues("process_csv")) - beforeCompleted; got != 1 {
		t.Errorf("JobsCompleted delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(JobsRetried.WithLabelValues("process_csv")) - beforeRetried; got != 2 {
		t.Errorf("JobsRetried delta = %v, want 2", got)
	}
	if got := testutil.ToFloat64(JobsDead.WithLabelValues("process_csv")) - beforeDead; got != 0 {
		t.Errorf("JobsDead delta = %v, want 0 (never incremented)", got)
	}
}

func TestJobDuration_ObservesByOutcome(t *testing.T) {
	beforeCount := testutil.CollectAndCount(JobDuration)

	JobDuration.WithLabelValues("resize_image", OutcomeSuccess).Observe(0.5)
	JobDuration.WithLabelValues("resize_image", OutcomeFailure).Observe(1.5)

	afterCount := testutil.CollectAndCount(JobDuration)
	if afterCount-beforeCount != 2 {
		t.Errorf("JobDuration series count delta = %d, want 2 (one per outcome label)", afterCount-beforeCount)
	}
}
