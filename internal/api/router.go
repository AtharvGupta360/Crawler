package api

import (
	"log/slog"
	"net/http"

	"github.com/AtharvGupta360/JobCrawl/internal/crawler"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/AtharvGupta360/JobCrawl/internal/ws"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// ServerConfig holds optional dependencies for the API server.
type ServerConfig struct {
	Elastic   *store.ElasticStore // nil = search disabled
	JWTSecret string
	WSHub     *ws.Hub
}

// Server holds all API dependencies and the HTTP router.
type Server struct {
	router    *chi.Mux
	pg        *store.PostgresStore
	redis     *store.RedisStore
	scheduler *crawler.Scheduler
	elastic   *store.ElasticStore
	logger    *slog.Logger
	jwtSecret string
	wsHub     *ws.Hub
}

// NewServer creates a new API server with all routes registered.
func NewServer(pg *store.PostgresStore, redis *store.RedisStore, scheduler *crawler.Scheduler, cfg ServerConfig, logger *slog.Logger) *Server {
	s := &Server{
		router:    chi.NewRouter(),
		pg:        pg,
		redis:     redis,
		scheduler: scheduler,
		elastic:   cfg.Elastic,
		logger:    logger,
		jwtSecret: cfg.JWTSecret,
		wsHub:     cfg.WSHub,
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

	// Compress responses
	s.router.Use(middleware.Compress(5))
}

func (s *Server) setupRoutes() {
	// Health check (no content-type enforcement)
	s.router.Get("/health", s.handleHealth)

	s.router.Route("/api/v1", func(r chi.Router) {

		// ── Auth (public) ──────────────────────────────
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)

			// Protected auth routes
			r.Group(func(r chi.Router) {
				r.Use(s.JWTMiddleware)
				r.Get("/me", s.handleGetMe)
				r.Put("/me", s.handleUpdateMe)
			})
		})

		// ── Jobs ───────────────────────────────────────
		r.Route("/jobs", func(r chi.Router) {
			r.Get("/", s.handleListJobs)
			r.Get("/stats", s.handleJobStats)
			r.Get("/{jobID}", s.handleGetJob)
		})

		// ── Search (Elasticsearch-backed, optional) ────
		if s.elastic != nil {
			r.Get("/search", s.handleSearchJobs)
		}

		// ── Companies ──────────────────────────────────
		r.Route("/companies", func(r chi.Router) {
			r.Get("/", s.handleListCompanies)
			r.Post("/", s.handleCreateCompany)
		})

		// ── Crawl management ───────────────────────────
		r.Route("/crawl", func(r chi.Router) {
			r.Post("/trigger", s.handleTriggerCrawl)
			r.Post("/trigger/{companySlug}", s.handleTriggerCrawlCompany)
			r.Get("/runs", s.handleListCrawlRuns)
			r.Get("/health", s.handleCrawlerHealth)
			r.Get("/status", s.handleCrawlStatus)
		})

		// ── Alerts + Notifications + Match (JWT protected) ──
		r.Group(func(r chi.Router) {
			r.Use(s.JWTMiddleware)

			r.Route("/alerts", func(r chi.Router) {
				r.Get("/", s.handleListAlerts)
				r.Post("/", s.handleCreateAlert)
				r.Put("/{alertID}", s.handleUpdateAlert)
				r.Delete("/{alertID}", s.handleDeleteAlert)
			})

			r.Route("/notifications", func(r chi.Router) {
				r.Get("/", s.handleListNotifications)
				r.Put("/read-all", s.handleMarkAllRead)
				r.Put("/{notifID}/read", s.handleMarkRead)
			})

			r.Route("/match", func(r chi.Router) {
				r.Post("/resume", s.handleMatchResume)
				r.Get("/recommendations", s.handleRecommendations)
			})
		})

		// ── Trends ─────────────────────────────────────
		r.Route("/trends", func(r chi.Router) {
			r.Get("/skills", s.handleListSkillTrends)
			r.Get("/companies", s.handleListCompanyTrends)
			r.Get("/salaries", s.handleListSalaryTrends)
			r.With(s.JWTMiddleware).Post("/refresh", s.handleRefreshTrends)
		})

		// ── WebSocket (auth via query param) ───────────
		if s.wsHub != nil {
			r.Get("/ws", s.handleWebSocket)
			r.Get("/ws/alerts", s.handleWebSocket)
		}

		// ── Admin (JWT + admin role) ──────────────────
		r.Route("/admin", func(r chi.Router) {
			r.Use(s.JWTMiddleware)
			r.Use(s.AdminOnly)
			r.Get("/stats", s.handleAdminStats)
			r.Get("/crawl-summary", s.handleAdminCrawlSummary)
			r.Get("/users", s.handleAdminUsers)
		})
	})

	// ── Embedded Frontend (must be LAST — catch-all) ────
	s.setupStaticRoutes()
}
