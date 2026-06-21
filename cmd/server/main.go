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

	"github.com/AtharvGupta360/JobCrawl/internal/analytics"
	"github.com/AtharvGupta360/JobCrawl/internal/api"
	"github.com/AtharvGupta360/JobCrawl/internal/config"
	"github.com/AtharvGupta360/JobCrawl/internal/crawler"
	"github.com/AtharvGupta360/JobCrawl/internal/enricher"
	"github.com/AtharvGupta360/JobCrawl/internal/kafka"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/AtharvGupta360/JobCrawl/internal/ws"
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

	// ── Elasticsearch (optional — degrades gracefully) ──
	var elastic *store.ElasticStore
	if cfg.ElasticsearchURL != "" {
		es, err := store.NewElasticStore(cfg.ElasticsearchURL, logger)
		if err != nil {
			logger.Warn("Elasticsearch unavailable, search disabled", "error", err)
		} else {
			if err := es.EnsureJobIndex(ctx); err != nil {
				logger.Warn("failed to create ES index", "error", err)
			}
			elastic = es
			defer elastic.Close()
		}
	}

	// ── Enrichment Pipeline ──
	// Rule-based enrichment always active; AI enrichment activates when GEMINI_API_KEY is set.
	var aiEnricher *enricher.AIEnricher
	if cfg.HasGemini() {
		ae, err := enricher.NewAIEnricher(ctx, cfg.GeminiAPIKey, logger)
		if err != nil {
			logger.Warn("AI enricher unavailable, falling back to rules only", "error", err)
		} else {
			aiEnricher = ae
			defer aiEnricher.Close()
		}
	}
	// 1 AI call per second — stays well within free-tier Gemini limits
	enrichPipeline := enricher.NewPipeline(aiEnricher, 1.0, logger)

	// ── Kafka Producer (optional — degrades to synchronous processing) ──
	var publisher crawler.EventPublisher
	var kafkaProducer *kafka.Producer
	var kafkaProcessor *kafka.Processor
	var kafkaIndexer *kafka.Indexer
	var alertEvaluator *kafka.AlertEvaluator
	var notifier *kafka.Notifier

	wsHub := ws.NewHub(logger)
	go wsHub.Run()

	if len(cfg.KafkaBrokers) > 0 && cfg.KafkaBrokers[0] != "" {
		kafkaProducer = kafka.NewProducer(cfg.KafkaBrokers, logger)
		publisher = kafka.NewPublisherAdapter(kafkaProducer)
		defer kafkaProducer.Close()

		// ── Kafka Job Processor (jobs.raw → enrich → dedup → PG upsert → jobs.processed) ──
		kafkaProcessor = kafka.NewProcessor(cfg.KafkaBrokers, kafkaProducer, pg, redis, enrichPipeline, logger)
		kafkaProcessor.Start(ctx)
		defer kafkaProcessor.Stop()

		// ── Kafka ES Indexer (jobs.processed → Elasticsearch) ──
		if elastic != nil {
			kafkaIndexer = kafka.NewIndexer(cfg.KafkaBrokers, pg, elastic, logger)
			kafkaIndexer.Start(ctx)
			defer kafkaIndexer.Stop()
		}

		// Kafka Alert Evaluator (jobs.processed -> notifications + jobs.alerts)
		alertEvaluator = kafka.NewAlertEvaluator(cfg.KafkaBrokers, kafkaProducer, pg, redis, logger)
		alertEvaluator.Start(ctx)
		defer alertEvaluator.Stop()

		// Kafka Notifier (jobs.alerts -> WebSocket clients)
		notifier = kafka.NewNotifier(cfg.KafkaBrokers, wsHub, logger)
		notifier.Start(ctx)
		defer notifier.Stop()

		logger.Info("Kafka pipeline started",
			"brokers", cfg.KafkaBrokers,
			"processor", true,
			"indexer", kafkaIndexer != nil,
			"alert_evaluator", true,
			"notifier", true,
		)
	} else {
		logger.Info("Kafka not configured, using synchronous processing")
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
		crawlers, 6*time.Hour, publisher, logger,
	)
	scheduler.Start()
	defer scheduler.Stop()

	// ── Trend Analytics (daily materialized snapshots) ──
	trendScheduler := analytics.NewScheduler(pg, 24*time.Hour, 100, logger)
	trendScheduler.Start(ctx)
	defer trendScheduler.Stop()

	// ── API Server ──
	serverCfg := api.ServerConfig{
		Elastic:   elastic,
		JWTSecret: cfg.JWTSecret,
		WSHub:     wsHub,
	}
	server := api.NewServer(pg, redis, scheduler, serverCfg, logger)

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

	// Cancel context to stop Kafka consumers
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
