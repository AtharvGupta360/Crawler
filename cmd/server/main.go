package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/api"
	"github.com/AtharvGupta360/JobCrawl/internal/config"
	"github.com/AtharvGupta360/JobCrawl/internal/crawler"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/joho/godotenv"
)

func main() {
	// ── Load .env (development only, no error if missing) ──
	_ = godotenv.Load()

	// ── Structured logging ──
	logLevel := slog.LevelInfo
	if os.Getenv("APP_ENV") == "development" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	// ── Configuration ──
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("starting JobCrawl",
		"env", cfg.AppEnv,
		"port", cfg.AppPort,
	)

	// ── Context with cancellation for graceful shutdown ──
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── PostgreSQL ──
	pg, err := store.NewPostgresStore(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	// ── Redis ──
	redis, err := store.NewRedisStore(ctx, cfg.RedisURL, logger)
	if err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redis.Close()

	// ── Seed default companies ──
	if err := crawler.SeedDefaultCompanies(ctx, pg, logger); err != nil {
		logger.Warn("failed to seed companies", "error", err)
	}

	// ── Crawlers ──
	rateLimiter := crawler.NewRateLimiter(cfg.CrawlRateLimitPerSecond, 3, logger)
	circuitBreaker := crawler.NewCircuitBreaker(5, 5*time.Minute, logger)

	crawlers := []crawler.Crawler{
		crawler.NewGreenhouseCrawler(rateLimiter, circuitBreaker, cfg.CrawlUserAgent, logger),
		crawler.NewLeverCrawler(rateLimiter, circuitBreaker, cfg.CrawlUserAgent, logger),
		crawler.NewAshbyCrawler(rateLimiter, circuitBreaker, cfg.CrawlUserAgent, logger),
	}

	// ── Scheduler (crawls every 6 hours) ──
	scheduler := crawler.NewScheduler(
		pg, redis, rateLimiter, circuitBreaker,
		crawlers, 6*time.Hour, logger,
	)
	scheduler.Start()
	defer scheduler.Stop()

	// ── API Server ──
	server := api.NewServer(pg, redis, scheduler, logger)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AppPort),
		Handler:      server,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Start server in goroutine ──
	go func() {
		logger.Info("HTTP server listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// ── Graceful shutdown ──
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutting down", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
