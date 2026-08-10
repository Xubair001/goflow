package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server returns a minimal HTTP server exposing /metrics and /healthz, for
// processes (worker, dispatcher) that don't otherwise have an HTTP surface.
// Readiness isn't included here: unlike apiserver, these processes have no
// meaningful way to serve traffic degraded, so liveness is the only signal
// worth exposing.
func Server(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// ServeUntil starts a metrics/health server on addr and shuts it down when
// ctx is cancelled. Meant to be run in its own goroutine:
//
//	go metrics.ServeUntil(ctx, cfg.MetricsAddr, logger)
//
// It's fire-and-forget from the caller's perspective: this server carries
// no in-flight state worth waiting on at exit, unlike the job processing
// loops it runs alongside.
func ServeUntil(ctx context.Context, addr string, logger *slog.Logger) {
	srv := Server(addr)
	errCh := make(chan error, 1)

	go func() {
		logger.Info("metrics server starting", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server error", "error", err)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server shutdown", "error", err)
	}
}
