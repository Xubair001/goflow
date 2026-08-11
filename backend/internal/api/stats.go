package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleQueueStats(c *gin.Context) {
	stats, err := s.store.Stats(c.Request.Context())
	if err != nil {
		s.logger.Error("queue stats", "error", err)
		writeError(c, http.StatusInternalServerError, codeInternal, "failed to fetch queue stats")
		return
	}
	c.JSON(http.StatusOK, stats)
}
