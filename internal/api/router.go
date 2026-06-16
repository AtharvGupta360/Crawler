package api

import (
	"log/slog"
	"net/http"

	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server holds all API dependencies and the HTTP router.
type Server struct {
	router *chi.Mux
	pg     *store.PostgresStore
	redis  *store.RedisStore
	logger *slog.Logger
}

// NewServer creates a new API server with all routes registered.
func NewServer(pg *store.PostgresStore, redis *store.RedisStore, logger *slog.Logger) *Server {
	s := &Server{
		router: chi.NewRouter(),
		pg:     pg,
		redis:  redis,
		logger: logger,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) setupMiddleware() {
	// Request ID for tracing
	s.router.Use(middleware.RequestID)

	// Structured request logging
	s.router.Use(middleware.RealIP)
	s.router.Use(NewStructuredLogger(s.logger))

	// Recover from panics
	s.router.Use(middleware.Recoverer)

	// CORS — permissive for development
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "https://localhost:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Content-Type enforcement for POST/PUT
	s.router.Use(middleware.AllowContentType("application/json"))

	// Compress responses
	s.router.Use(middleware.Compress(5))
}

func (s *Server) setupRoutes() {
	s.router.Get("/health", s.handleHealth)

	s.router.Route("/api/v1", func(r chi.Router) {
		// Jobs
		r.Route("/jobs", func(r chi.Router) {
			r.Get("/", s.handleListJobs)
			r.Get("/stats", s.handleJobStats)
			r.Get("/{jobID}", s.handleGetJob)
		})

		// Companies
		r.Route("/companies", func(r chi.Router) {
			r.Get("/", s.handleListCompanies)
			r.Post("/", s.handleCreateCompany)
		})

		// Crawl management
		r.Route("/crawl", func(r chi.Router) {
			r.Post("/trigger", s.handleTriggerCrawl)
			r.Get("/runs", s.handleListCrawlRuns)
			r.Get("/health", s.handleCrawlerHealth)
		})
	})
}
