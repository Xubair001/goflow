// Package api implements the HTTP surface: the versioned job queue REST
// API, a Server-Sent Events feed for live dashboard updates, health checks,
// metrics, and API docs.
package api

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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
	logger      *slog.Logger
	db          pinger
	redis       pinger
	corsOrigins []string
	staticFS    fs.FS
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
}

// NewServer returns a Server ready to build routes and serve requests.
func NewServer(cfg Config) *Server {
	return &Server{
		store:       cfg.Store,
		registry:    cfg.Registry,
		broadcaster: newBroadcaster(),
		logger:      cfg.Logger,
		db:          cfg.DB,
		redis:       cfg.Redis,
		corsOrigins: cfg.CORSOrigins,
		staticFS:    cfg.StaticFS,
	}
}

// Routes builds the full HTTP handler: middleware, health/metrics/docs
// endpoints, and the versioned job queue API.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(s.logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: s.corsOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)
	r.Handle("/metrics", promhttp.Handler())

	r.Get("/openapi.yaml", s.handleOpenAPISpec)
	r.Get("/docs", s.handleSwaggerUI)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/jobs", func(r chi.Router) {
			r.Post("/", s.handleCreateJob)
			r.Get("/", s.handleListJobs)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleGetJob)
				r.Post("/retry", s.handleRetryJob)
				r.Post("/cancel", s.handleCancelJob)
			})
		})
		r.Get("/queue/stats", s.handleQueueStats)
		r.Get("/events", s.handleEvents)
	})

	// Dashboard last: chi matches the more specific routes above first
	// regardless of registration order, so this catch-all never shadows
	// the API/health/docs routes.
	if s.staticFS != nil {
		r.Handle("/*", http.FileServer(http.FS(s.staticFS)))
	}

	return r
}
