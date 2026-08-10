package config

import (
	"fmt"
	"os"
	"time"
)

// Worker holds cmd/worker's configuration.
type Worker struct {
	DatabaseURL   string
	RedisAddr     string
	Stream        string
	ConsumerGroup string
	ConsumerName  string

	Concurrency     int
	ConsumeBatch    int64
	ReclaimInterval time.Duration
	ReclaimMinIdle  time.Duration
	JobTimeout      time.Duration
	RetryBaseDelay  time.Duration
	RetryMaxDelay   time.Duration

	LogEnv   string
	LogLevel string
}

// LoadWorker reads Worker config from the environment, applying defaults
// suitable for local development.
func LoadWorker() (Worker, error) {
	c := Worker{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://jobqueue:jobqueue@localhost:5432/jobqueue?sslmode=disable"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Stream:        getEnv("QUEUE_STREAM", "jobqueue:jobs"),
		ConsumerGroup: getEnv("QUEUE_GROUP", "workers"),
		ConsumerName:  getEnv("CONSUMER_NAME", defaultConsumerName()),
		LogEnv:        getEnv("LOG_ENV", "development"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}

	var err error
	if c.Concurrency, err = getEnvInt("WORKER_CONCURRENCY", 10); err != nil {
		return Worker{}, err
	}
	consumeBatch, err := getEnvInt("WORKER_CONSUME_BATCH", 10)
	if err != nil {
		return Worker{}, err
	}
	c.ConsumeBatch = int64(consumeBatch)
	if c.ReclaimInterval, err = getEnvDuration("WORKER_RECLAIM_INTERVAL", 30*time.Second); err != nil {
		return Worker{}, err
	}
	if c.ReclaimMinIdle, err = getEnvDuration("WORKER_RECLAIM_MIN_IDLE", time.Minute); err != nil {
		return Worker{}, err
	}
	if c.JobTimeout, err = getEnvDuration("WORKER_JOB_TIMEOUT", 2*time.Minute); err != nil {
		return Worker{}, err
	}
	if c.RetryBaseDelay, err = getEnvDuration("WORKER_RETRY_BASE_DELAY", time.Second); err != nil {
		return Worker{}, err
	}
	if c.RetryMaxDelay, err = getEnvDuration("WORKER_RETRY_MAX_DELAY", 5*time.Minute); err != nil {
		return Worker{}, err
	}
	return c, nil
}

// defaultConsumerName gives each worker process a name that's unique
// enough to distinguish it from other replicas in the same consumer group
// without requiring any operator configuration.
func defaultConsumerName() string {
	host, err := os.Hostname()
	if err != nil {
		host = "worker"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}
