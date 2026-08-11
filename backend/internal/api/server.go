// Package api implements the HTTP surface: the versioned job queue REST
// API, a Server-Sent Events feed for live dashboard updates, health checks,
// metrics, and API docs.
package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// Server holds apiserver's dependencies and builds its HTTP handler. It
// holds no network state itself (no listener, no *http.Server) — that's
// cmd/apiserver's job, so Server.Routes() stays trivially testable with
// httptest.
type Server struct {
	store       store.Store
	registry    *job.Registry
	broadcaster *broadcaster
	uploads     *uploadStore
	logger      *slog.Logger
	db          pinger
	redis       pinger
	corsOrigins []string
	staticFS    fs.FS
	debug       bool
}

// Config controls Server construction.
type Config struct {
	Store    store.Store
	Registry *job.Registry
	DB       pinger
	Redis    pinger
	Logger   *slog.Logger
	// CORSOrigins are the origins allowed to call the API from a browser,
	// e.g. the dashboard's dev server. Empty disables cross-origin requests.
	CORSOrigins []string
	// StaticFS serves the built dashboard at "/" (see internal/web). Nil
	// skips mounting it — used by tests that only care about the API.
	StaticFS fs.FS
	// Debug enables Gin's debug mode (verbose route-registration output on
	// startup). Mirrors config.APIServer.LogEnv == "development"; leave
	// false in production.
	Debug bool
}

// NewServer returns a Server ready to build routes and serve requests.
func NewServer(cfg Config) *Server {
	return &Server{
		store:       cfg.Store,
		registry:    cfg.Registry,
		broadcaster: newBroadcaster(),
		uploads:     newUploadStore(),
		logger:      cfg.Logger,
		db:          cfg.DB,
		redis:       cfg.Redis,
		corsOrigins: cfg.CORSOrigins,
		staticFS:    cfg.StaticFS,
		debug:       cfg.Debug,
	}
}

// Routes builds the full HTTP handler: middleware, health/metrics/docs
// endpoints, and the versioned job queue API.
func (s *Server) Routes() http.Handler {
	if s.debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(requestID())
	r.Use(requestLogger(s.logger))
	r.Use(recovery(s.logger))
	// Only registered when origins are configured: gin-contrib/cors panics
	// on an empty/unset AllowOrigins (it treats that as a misconfiguration
	// to catch early, not "allow nothing"), so skipping registration
	// entirely is how "no cross-origin access" is expressed here instead.
	if len(s.corsOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins: s.corsOrigins,
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			AllowHeaders: []string{"Content-Type"},
			MaxAge:       300 * time.Second,
		}))
	}

	r.GET("/healthz", s.handleHealthz)
	r.GET("/readyz", s.handleReadyz)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/openapi.yaml", s.handleOpenAPISpec)
	r.GET("/docs", s.handleSwaggerUI)

	v1 := r.Group("/api/v1")
	{
		jobs := v1.Group("/jobs")
		{
			jobs.POST("", s.handleCreateJob)
			jobs.GET("", s.handleListJobs)
			jobs.GET("/:id", s.handleGetJob)
			jobs.POST("/:id/retry", s.handleRetryJob)
			jobs.POST("/:id/cancel", s.handleCancelJob)
		}
		v1.GET("/queue/stats", s.handleQueueStats)
		v1.GET("/events", s.handleEvents)
		v1.POST("/uploads", s.handleUpload)
		v1.GET("/uploads/:id", s.handleGetUpload)
	}

	// Dashboard last, via NoRoute rather than a registered wildcard: Gin's
	// router rejects a literal catch-all pattern that would sit alongside
	// the static routes above, and NoRoute is exactly "nothing else
	// matched" regardless of registration order anyway.
	if s.staticFS != nil {
		r.NoRoute(gin.WrapH(http.FileServer(http.FS(s.staticFS))))
	}

	return r
}
