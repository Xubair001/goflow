// Package config loads process configuration from environment variables.
// There's one struct per binary (cmd/dispatcher, cmd/worker, cmd/apiserver)
// rather than a single shared struct, since each process only needs a
// subset of settings and a framework-free approach makes exactly what's
// read, and its default, visible at the call site.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s=%q: %w", key, v, err)
	}
	return d, nil
}

func getEnvInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid integer for %s=%q: %w", key, v, err)
	}
	return n, nil
}
