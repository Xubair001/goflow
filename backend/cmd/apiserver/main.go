// Command apiserver runs the job queue's REST API, SSE live-updates feed,
// health checks, metrics, and API docs. It writes job rows to Postgres and
// never talks to Redis directly — the dispatcher owns getting a submitted
// job onto the queue.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/abdullah-zubair/jobqueue/internal/api"
	"github.com/abdullah-zubair/jobqueue/internal/config"
	"github.com/abdullah-zubair/jobqueue/internal/handlers"
	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/logging"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("apiserver exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadAPIServer()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := logging.New(os.Stderr, cfg.LogEnv, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = redisClient.Close() }()

	// Populated purely to validate submitted job types against the set the
	// worker can actually execute; the handler instances themselves are
	// never invoked from this process (zero-value Deps is fine).
	registry := job.NewRegistry()
	handlers.Register(registry, handlers.Deps{})

	srv := api.NewServer(api.Config{
		Store:       store.NewPostgres(pool),
		Registry:    registry,
		DB:          pool,
		Redis:       redisPinger{redisClient},
		Logger:      logger,
		CORSOrigins: cfg.CORSOrigins,
	})

	go srv.RunEventsPoller(ctx, cfg.EventsPollInterval)

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Routes(),
		// Bounds how long a client may take sending request headers, so a
		// slow/malicious client trickling bytes (a Slowloris attack) can't
		// tie up a connection indefinitely.
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("apiserver starting", "addr", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	logger.Info("apiserver shutting down")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return <-serveErr
}

// redisPinger adapts go-redis's Ping (which returns a *redis.StatusCmd,
// not a plain error) to the api.Config.Redis pinger interface.
type redisPinger struct {
	client *redis.Client
}

func (r redisPinger) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
