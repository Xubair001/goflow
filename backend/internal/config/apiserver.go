package config

import (
	"strings"
	"time"
)

// APIServer holds cmd/apiserver's configuration.
type APIServer struct {
	ListenAddr  string
	DatabaseURL string
	RedisAddr   string
	// CORSOrigins are the origins allowed to call the API from a browser
	// (e.g. the dashboard's dev server).
	CORSOrigins []string

	EventsPollInterval time.Duration
	ShutdownTimeout    time.Duration

	LogEnv   string
	LogLevel string
}

// LoadAPIServer reads APIServer config from the environment, applying
// defaults suitable for local development.
func LoadAPIServer() (APIServer, error) {
	c := APIServer{
		ListenAddr:  getEnv("LISTEN_ADDR", ":8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://jobqueue:jobqueue@localhost:5432/jobqueue?sslmode=disable"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		CORSOrigins: strings.Split(getEnv("CORS_ORIGINS", "http://localhost:5173"), ","),
		LogEnv:      getEnv("LOG_ENV", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}

	var err error
	if c.EventsPollInterval, err = getEnvDuration("EVENTS_POLL_INTERVAL", 2*time.Second); err != nil {
		return APIServer{}, err
	}
	if c.ShutdownTimeout, err = getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return APIServer{}, err
	}
	return c, nil
}
