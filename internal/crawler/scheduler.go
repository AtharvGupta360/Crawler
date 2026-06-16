package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
)

// Scheduler manages periodic crawl runs and on-demand triggers.
type Scheduler struct {
	crawlers    map[string]Crawler // ATS name → crawler
	pg          *store.PostgresStore
	redis       *store.RedisStore
	rateLimiter *RateLimiter
	breaker     *CircuitBreaker
	logger      *slog.Logger

	// Control
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewScheduler creates a new crawl scheduler.
func NewScheduler(
	pg *store.PostgresStore,
	redis *store.RedisStore,
	rateLimiter *RateLimiter,
	breaker *CircuitBreaker,
	crawlers []Crawler,
	interval time.Duration,
	logger *slog.Logger,
) *Scheduler {
	crawlerMap := make(map[string]Crawler)
	for _, c := range crawlers {
		crawlerMap[c.Name()] = c
	}

	return &Scheduler{
		crawlers:    crawlerMap,
		pg:          pg,
		redis:       redis,
		rateLimiter: rateLimiter,
		breaker:     breaker,
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

	// Process each listing
	var jobsNew, jobsUpdated int
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

		if isNew {
			jobsNew++
		} else {
			jobsUpdated++
		}
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
