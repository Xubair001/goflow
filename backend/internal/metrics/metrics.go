// Package metrics defines the Prometheus metrics shared across binaries and
// a minimal HTTP server for exposing them from processes (worker,
// dispatcher) that don't otherwise have an HTTP surface.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// JobsDispatched counts jobs the dispatcher published to the queue.
	JobsDispatched = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobqueue_jobs_dispatched_total",
		Help: "Total jobs published to the queue by the dispatcher, by job type.",
	}, []string{"job_type"})

	// JobsReconciled counts jobs the reconciler reset to pending after
	// their lease went stale (a lost Redis entry or a dead worker).
	JobsReconciled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobqueue_jobs_reconciled_total",
		Help: "Total jobs reset to pending after their lease went stale, by job type.",
	}, []string{"job_type"})

	// JobsCompleted counts successful handler executions.
	JobsCompleted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobqueue_jobs_completed_total",
		Help: "Total jobs that completed successfully, by job type.",
	}, []string{"job_type"})

	// JobsRetried counts failed attempts that still had a retry budget.
	JobsRetried = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobqueue_jobs_retried_total",
		Help: "Total job attempts that failed but were retried, by job type.",
	}, []string{"job_type"})

	// JobsDead counts jobs that exhausted their retry budget.
	JobsDead = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobqueue_jobs_dead_total",
		Help: "Total jobs that exhausted retries and were marked dead, by job type.",
	}, []string{"job_type"})

	// JobDuration is handler execution latency, labeled by outcome so a
	// slow failure path doesn't get averaged in with fast successes.
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "jobqueue_job_duration_seconds",
		Help:    "Handler execution duration in seconds, by job type and outcome.",
		Buckets: prometheus.DefBuckets,
	}, []string{"job_type", "outcome"})
)

// Outcome labels for JobDuration.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)
