package api

import (
	"context"
	"net/http"
	"time"
)

// pinger is satisfied directly by *pgxpool.Pool; go-redis's client needs a
// small adapter (see cmd/apiserver) since its Ping returns a *redis.StatusCmd,
// not a plain error. Keeping this as a narrow interface lets internal/api
// stay decoupled from both driver packages.
type pinger interface {
	Ping(ctx context.Context) error
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx); err != nil {
		writeError(w, s.logger, http.StatusServiceUnavailable, codeInternal, "database unavailable")
		return
	}
	if err := s.redis.Ping(ctx); err != nil {
		writeError(w, s.logger, http.StatusServiceUnavailable, codeInternal, "redis unavailable")
		return
	}
	w.WriteHeader(http.StatusOK)
}
