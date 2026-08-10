// Command dispatcher polls Postgres for due jobs, publishes them to the
// Redis queue for workers to consume, and periodically reclaims jobs whose
// lease expired without completing.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/abdullah-zubair/jobqueue/internal/config"
	"github.com/abdullah-zubair/jobqueue/internal/dispatcher"
	"github.com/abdullah-zubair/jobqueue/internal/logging"
	"github.com/abdullah-zubair/jobqueue/internal/queue"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("dispatcher exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadDispatcher()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := logging.New(os.Stderr, cfg.LogEnv, cfg.LogLevel)

	// signal.NotifyContext ties SIGINT/SIGTERM to context cancellation, so
	// Run's two loops (which already select on ctx.Done()) drain cleanly
	// instead of being killed mid-iteration. Calling stop() lets a second
	// signal force an immediate exit rather than being ignored.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = redisClient.Close() }()

	q, err := queue.NewRedis(ctx, redisClient, cfg.Stream, cfg.ConsumerGroup, 0)
	if err != nil {
		return fmt.Errorf("init queue: %w", err)
	}

	d := dispatcher.New(store.NewPostgres(pool), q, dispatcher.Config{
		PollInterval:      cfg.PollInterval,
		ReconcileInterval: cfg.ReconcileInterval,
		BatchSize:         cfg.BatchSize,
		StaleAfter:        cfg.StaleAfter,
	}, logger)

	logger.Info("dispatcher starting",
		"poll_interval", cfg.PollInterval,
		"reconcile_interval", cfg.ReconcileInterval,
		"stream", cfg.Stream,
		"consumer_group", cfg.ConsumerGroup,
	)
	err = d.Run(ctx)
	logger.Info("dispatcher stopped")
	return err
}
