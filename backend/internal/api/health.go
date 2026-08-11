package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// pinger is satisfied directly by *pgxpool.Pool; go-redis's client needs a
// small adapter (see cmd/apiserver) since its Ping returns a *redis.StatusCmd,
// not a plain error. Keeping this as a narrow interface lets internal/api
// stay decoupled from both driver packages.
type pinger interface {
	Ping(ctx context.Context) error
}

func (s *Server) handleHealthz(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (s *Server) handleReadyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx); err != nil {
		writeError(c, http.StatusServiceUnavailable, codeInternal, "database unavailable")
		return
	}
	if err := s.redis.Ping(ctx); err != nil {
		writeError(c, http.StatusServiceUnavailable, codeInternal, "redis unavailable")
		return
	}
	c.Status(http.StatusOK)
}
