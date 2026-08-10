// Package logging configures the process-wide structured logger.
package logging

import (
	"io"
	"log/slog"
)

// New returns a slog.Logger writing to w. In "development" env it uses a
// human-readable text handler; anything else gets JSON, which is what
// production log pipelines (Loki, Datadog, CloudWatch, ...) expect.
// An unparseable level falls back to Info rather than failing startup over
// a log-level typo.
func New(w io.Writer, env, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if env == "development" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}
	return slog.New(handler)
}
