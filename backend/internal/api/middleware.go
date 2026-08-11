package api

import (
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDKey = "request_id"

// requestID assigns each request a UUID (reusing the caller's X-Request-Id
// if it already set one, so a request traced through an upstream proxy
// keeps one ID end to end), stashes it in the gin context for
// requestLogger, and echoes it back as a response header.
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(requestIDKey, id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

// requestLogger logs one structured line per request through logger (the
// same slog.Logger, JSON-in-production / text-in-dev, as the rest of the
// process) rather than Gin's own default logger, which writes plain text
// straight to stdout regardless of environment.
func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		size := c.Writer.Size()
		if size < 0 { // nothing written, e.g. a 204 or an aborted request
			size = 0
		}
		logger.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"bytes", size,
			"duration", time.Since(start),
			"request_id", c.GetString(requestIDKey),
		)
	}
}

// recovery turns a panicking handler into a 500 instead of a crashed
// process, logging the panic and its stack trace through logger. It's a
// custom middleware rather than gin.Recovery() so the panic goes through
// the same structured logger as every other log line instead of gin's own
// default writer -- the same reasoning as requestLogger above.
func recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		logger.Error("panic recovered",
			"error", recovered,
			"path", c.Request.URL.Path,
			"stack", string(debug.Stack()),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
