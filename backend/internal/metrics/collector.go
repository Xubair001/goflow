package metrics

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/abdullah-zubair/jobqueue/internal/store"
)

var queueDepthDesc = prometheus.NewDesc(
	"jobqueue_queue_depth",
	"Current number of jobs by status.",
	[]string{"status"}, nil,
)

// QueueDepthCollector reports live job counts by status on every Prometheus
// scrape, rather than a background-updated gauge -- so it always reflects
// Postgres's actual state instead of whatever a poller last saw.
type QueueDepthCollector struct {
	store  store.Store
	logger *slog.Logger
}

// NewQueueDepthCollector returns a collector backed by s. Register it with
// prometheus.MustRegister (or similar) to include it in a /metrics scrape.
func NewQueueDepthCollector(s store.Store, logger *slog.Logger) *QueueDepthCollector {
	return &QueueDepthCollector{store: s, logger: logger}
}

// Describe implements prometheus.Collector.
func (c *QueueDepthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- queueDepthDesc
}

// Collect implements prometheus.Collector.
func (c *QueueDepthCollector) Collect(ch chan<- prometheus.Metric) {
	stats, err := c.store.Stats(context.Background())
	if err != nil {
		c.logger.Error("collect queue depth metrics", "error", err)
		return
	}
	for status, count := range map[string]int{
		"pending":   stats.Pending,
		"queued":    stats.Queued,
		"running":   stats.Running,
		"completed": stats.Completed,
		"dead":      stats.Dead,
		"cancelled": stats.Cancelled,
	} {
		ch <- prometheus.MustNewConstMetric(queueDepthDesc, prometheus.GaugeValue, float64(count), status)
	}
}

var _ prometheus.Collector = (*QueueDepthCollector)(nil)
