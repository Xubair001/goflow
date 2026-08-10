package config

import "time"

// Dispatcher holds cmd/dispatcher's configuration.
type Dispatcher struct {
	DatabaseURL   string
	RedisAddr     string
	Stream        string
	ConsumerGroup string

	PollInterval      time.Duration
	ReconcileInterval time.Duration
	BatchSize         int
	StaleAfter        time.Duration

	LogEnv   string
	LogLevel string
}

// LoadDispatcher reads Dispatcher config from the environment, applying
// defaults suitable for local development.
func LoadDispatcher() (Dispatcher, error) {
	c := Dispatcher{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://jobqueue:jobqueue@localhost:5432/jobqueue?sslmode=disable"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Stream:        getEnv("QUEUE_STREAM", "jobqueue:jobs"),
		ConsumerGroup: getEnv("QUEUE_GROUP", "workers"),
		LogEnv:        getEnv("LOG_ENV", "development"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}

	var err error
	if c.PollInterval, err = getEnvDuration("DISPATCH_POLL_INTERVAL", time.Second); err != nil {
		return Dispatcher{}, err
	}
	if c.ReconcileInterval, err = getEnvDuration("DISPATCH_RECONCILE_INTERVAL", 30*time.Second); err != nil {
		return Dispatcher{}, err
	}
	if c.BatchSize, err = getEnvInt("DISPATCH_BATCH_SIZE", 50); err != nil {
		return Dispatcher{}, err
	}
	if c.StaleAfter, err = getEnvDuration("DISPATCH_STALE_AFTER", 5*time.Minute); err != nil {
		return Dispatcher{}, err
	}
	return c, nil
}
