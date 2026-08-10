// Command worker consumes jobs from the Redis queue, executes them against
// registered handlers, and records the outcome in Postgres. It's
// horizontally scalable: run as many replicas as needed, each with its own
// consumer name in the shared consumer group.
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
	"github.com/abdullah-zubair/jobqueue/internal/handlers"
	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/logging"
	"github.com/abdullah-zubair/jobqueue/internal/queue"
	"github.com/abdullah-zubair/jobqueue/internal/store"
	"github.com/abdullah-zubair/jobqueue/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadWorker()
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

	// Bounded block (not 0/infinite): see config.Worker.ConsumeBlock for
	// why an unbounded blocking read is a graceful-shutdown risk.
	q, err := queue.NewRedis(ctx, redisClient, cfg.Stream, cfg.ConsumerGroup, cfg.ConsumeBlock)
	if err != nil {
		return fmt.Errorf("init queue: %w", err)
	}

	jobStore := store.NewPostgres(pool)

	registry := job.NewRegistry()
	handlers.Register(registry, handlers.Deps{
		Store:        jobStore,
		SMTPAddr:     cfg.SMTPAddr,
		SMTPFrom:     cfg.SMTPFrom,
		SMTPUsername: cfg.SMTPUsername,
		SMTPPassword: cfg.SMTPPassword,
	})

	p := worker.New(jobStore, q, registry, cfg.ConsumerName, worker.Config{
		Concurrency:     cfg.Concurrency,
		ConsumeBatch:    cfg.ConsumeBatch,
		ReclaimInterval: cfg.ReclaimInterval,
		ReclaimMinIdle:  cfg.ReclaimMinIdle,
		JobTimeout:      cfg.JobTimeout,
		RetryBaseDelay:  cfg.RetryBaseDelay,
		RetryMaxDelay:   cfg.RetryMaxDelay,
	}, logger)

	logger.Info("worker starting",
		"consumer_name", cfg.ConsumerName,
		"concurrency", cfg.Concurrency,
		"stream", cfg.Stream,
		"consumer_group", cfg.ConsumerGroup,
		"registered_job_types", registry.Types(),
	)
	err = p.Run(ctx)
	logger.Info("worker stopped")
	return err
}
