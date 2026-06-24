package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/google/uuid"
)

// EventPublisher is an interface for publishing crawl events to a message queue.
// This decouples the scheduler from the Kafka package to avoid circular imports.
type EventPublisher interface {
	PublishRawListing(ctx context.Context, companyID uuid.UUID, companySlug string, crawlRunID uuid.UUID, listing RawJobListing) error
}

// Scheduler manages periodic crawl runs and on-demand triggers.
type Scheduler struct {
	crawlers      map[string]Crawler // ATS name → crawler
	pg            *store.PostgresStore
	redis         *store.RedisStore
	rateLimiter   *RateLimiter
	breaker       *CircuitBreaker
	publisher     EventPublisher // nil = synchronous fallback
	syncProcessor SyncProcessor  // optional: enriches, indexes, evaluates alerts inline
	logger        *slog.Logger

	// Control
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewScheduler creates a new crawl scheduler.
// If publisher is nil, the scheduler falls back to synchronous processing
// (direct Redis dedup + PostgreSQL upsert) for local development without Kafka.
func NewScheduler(
	pg *store.PostgresStore,
	redis *store.RedisStore,
	rateLimiter *RateLimiter,
	breaker *CircuitBreaker,
	crawlers []Crawler,
	interval time.Duration,
	publisher EventPublisher,
	logger *slog.Logger,
) *Scheduler {
	crawlerMap := make(map[string]Crawler)
	for _, c := range crawlers {
		crawlerMap[c.Name()] = c
	}

	mode := "event-driven (Kafka)"
	if publisher == nil {
		mode = "synchronous (no Kafka)"
	}

	logger.Info("scheduler created", "mode", mode, "interval", interval)

	return &Scheduler{
		crawlers:    crawlerMap,
		pg:          pg,
		redis:       redis,
		rateLimiter: rateLimiter,
		breaker:     breaker,
		publisher:   publisher,
		interval:    interval,
		stopCh:      make(chan struct{}),
		logger:      logger.With("component", "scheduler"),
	}
}

// Start begins the periodic crawl loop.
func (s *Scheduler) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("scheduler started", "interval", s.interval)

		// Run immediately on startup
		s.runAllCrawls()

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runAllCrawls()
			case <-s.stopCh:
				s.logger.Info("scheduler stopped")
				return
			}
		}
	}()
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// SetSyncProcessor sets the optional synchronous processor that runs
// enrichment, ES indexing, and alert evaluation inline (no Kafka).
func (s *Scheduler) SetSyncProcessor(sp SyncProcessor) {
	s.syncProcessor = sp
	s.logger.Info("sync processor attached to scheduler")
}

// TriggerCrawl manually triggers a crawl for a specific company.
func (s *Scheduler) TriggerCrawl(ctx context.Context, company models.Company) (*models.CrawlRun, error) {
	crawler, ok := s.crawlers[company.ATSPlatform]
	if !ok {
		return nil, ErrUnsupportedATS{Platform: company.ATSPlatform}
	}

	return s.crawlCompany(ctx, crawler, company)
}

// TriggerCrawlAll manually triggers a crawl for all companies.
func (s *Scheduler) TriggerCrawlAll(ctx context.Context) {
	s.runAllCrawls()
}

// GetCrawlerHealth returns health status for all crawler platforms.
func (s *Scheduler) GetCrawlerHealth(ctx context.Context) map[string]CrawlerHealthStatus {
	statuses := make(map[string]CrawlerHealthStatus)

	for name, c := range s.crawlers {
		status := CrawlerHealthStatus{Platform: name}

		// Check circuit breaker
		isOpen, failures, cooldown := s.breaker.Status(getDomainForATS(name))
		status.CircuitOpen = isOpen
		status.ConsecutiveFailures = failures
		status.CooldownRemaining = cooldown

		// Ping the ATS
		if err := c.HealthCheck(ctx); err != nil {
			status.Healthy = false
			status.Message = err.Error()
		} else {
			status.Healthy = true
			status.Message = "reachable"
		}

		statuses[name] = status
	}

	return statuses
}

// ─────────────────────────────────────────────
// Internal
// ─────────────────────────────────────────────

func (s *Scheduler) runAllCrawls() {
	ctx := context.Background()

	companies, err := s.pg.ListCompanies(ctx)
	if err != nil {
		s.logger.Error("failed to list companies for crawl", "error", err)
		return
	}

	if len(companies) == 0 {
		s.logger.Info("no companies configured, skipping crawl")
		return
	}

	s.logger.Info("starting crawl cycle", "companies", len(companies))

	for _, company := range companies {
		crawler, ok := s.crawlers[company.ATSPlatform]
		if !ok {
			s.logger.Warn("no crawler for ATS platform",
				"company", company.Name,
				"ats", company.ATSPlatform,
			)
			continue
		}

		run, err := s.crawlCompany(ctx, crawler, company)
		if err != nil {
			s.logger.Error("crawl failed",
				"company", company.Name,
				"error", err,
			)
			continue
		}

		s.logger.Info("crawl completed",
			"company", company.Name,
			"jobs_found", run.JobsFound,
			"jobs_new", run.JobsNew,
			"jobs_updated", run.JobsUpdated,
			"jobs_removed", run.JobsRemoved,
			"duration_ms", run.DurationMs,
		)
	}

	s.logger.Info("crawl cycle complete")
}

func (s *Scheduler) crawlCompany(ctx context.Context, c Crawler, company models.Company) (*models.CrawlRun, error) {
	// Create crawl run record
	run := &models.CrawlRun{
		CompanyID: company.ID,
	}
	if err := s.pg.CreateCrawlRun(ctx, run); err != nil {
		return nil, err
	}

	crawlStart := time.Now()
	s.logger.Info("crawling company",
		"company", company.Name,
		"ats", company.ATSPlatform,
		"run_id", run.ID,
	)

	// Execute the crawl
	listings, err := c.CrawlCompany(ctx, company)
	if err != nil {
		run.Status = "failed"
		run.ErrorMessage = err.Error()
		s.pg.CompleteCrawlRun(ctx, run)

		// Update Redis health
		s.redis.SetCrawlerHealth(ctx, c.Name(), false, err.Error())

		return run, err
	}

	// Process listings — either via Kafka events or synchronously
	var jobsNew, jobsUpdated int
	if s.publisher != nil {
		// Event-driven: publish each listing to Kafka
		jobsNew, jobsUpdated = s.processViaKafka(ctx, listings, company, run.ID)
	} else {
		// Synchronous fallback: direct Redis dedup + PostgreSQL upsert
		jobsNew, jobsUpdated = s.processSynchronous(ctx, listings, company)
	}

	// Mark jobs not seen in this crawl as inactive
	jobsRemoved, _ := s.pg.MarkJobsInactive(ctx, company.ID, crawlStart)

	// Complete the crawl run
	run.Status = "completed"
	run.JobsFound = len(listings)
	run.JobsNew = jobsNew
	run.JobsUpdated = jobsUpdated
	run.JobsRemoved = int(jobsRemoved)
	s.pg.CompleteCrawlRun(ctx, run)

	// Update Redis health
	s.redis.SetCrawlerHealth(ctx, c.Name(), true, "ok")

	return run, nil
}

// processViaKafka publishes each listing as a CrawlEvent to Kafka.
// The Kafka processor consumer handles dedup and upsert.
func (s *Scheduler) processViaKafka(ctx context.Context, listings []RawJobListing, company models.Company, crawlRunID uuid.UUID) (jobsNew, jobsUpdated int) {
	published := 0
	for _, listing := range listings {
		if err := s.publisher.PublishRawListing(ctx, company.ID, company.Slug, crawlRunID, listing); err != nil {
			s.logger.Error("failed to publish listing to Kafka",
				"title", listing.Title,
				"error", err,
			)
			continue
		}
		published++
	}

	s.logger.Info("published listings to Kafka",
		"company", company.Name,
		"published", published,
		"total", len(listings),
	)

	// Note: actual new/updated counts come from the processor consumer.
	// We report published count as "found" — the run record captures this.
	return 0, 0
}

// processSynchronous is the direct-processing path for when Kafka is unavailable.
// When a SyncProcessor is attached, it also runs enrichment, ES indexing,
// alert evaluation, and WebSocket notifications inline.
func (s *Scheduler) processSynchronous(ctx context.Context, listings []RawJobListing, company models.Company) (jobsNew, jobsUpdated int) {
	for _, listing := range listings {
		// Check dedup via Redis
		contentHash := listing.ContentHash()
		seen, _ := s.redis.IsContentSeen(ctx, contentHash)
		if seen {
			jobsUpdated++ // content unchanged, just update last_seen_at
			continue
		}

		// Convert to job model and upsert
		job := listing.ToJob(company.ID)
		isNew, err := s.pg.UpsertJob(ctx, &job)
		if err != nil {
			s.logger.Error("failed to upsert job",
				"title", listing.Title,
				"error", err,
			)
			continue
		}

		// Mark content as seen in Redis
		s.redis.MarkContentSeen(ctx, contentHash)

		// Attach company for downstream alert matching
		job.Company = &company

		// Run enrichment + alerts + ES indexing inline (if configured)
		if s.syncProcessor != nil {
			if err := s.syncProcessor.ProcessSync(ctx, &job, isNew); err != nil {
				s.logger.Warn("sync processor error",
					"title", job.Title,
					"error", err,
				)
			}
		}

		if isNew {
			jobsNew++
		} else {
			jobsUpdated++
		}
	}

	return jobsNew, jobsUpdated
}

func getDomainForATS(ats string) string {
	switch ats {
	case models.ATSGreenhouse:
		return "boards-api.greenhouse.io"
	case models.ATSLever:
		return "api.lever.co"
	case models.ATSAshby:
		return "api.ashbyhq.com"
	default:
		return ats
	}
}

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

// CrawlerHealthStatus represents the health of a single ATS crawler.
type CrawlerHealthStatus struct {
	Platform            string        `json:"platform"`
	Healthy             bool          `json:"healthy"`
	Message             string        `json:"message"`
	CircuitOpen         bool          `json:"circuit_open"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	CooldownRemaining   time.Duration `json:"cooldown_remaining_ms"`
}

// ErrUnsupportedATS is returned when a company uses an ATS we don't have a crawler for.
type ErrUnsupportedATS struct {
	Platform string
}

func (e ErrUnsupportedATS) Error() string {
	return fmt.Sprintf("unsupported ATS platform: %s", e.Platform)
}
