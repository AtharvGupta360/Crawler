package kafka

import (
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/crawler"
	"github.com/google/uuid"
)

// CrawlEvent is published to jobs.raw when the crawler finds a job listing.
type CrawlEvent struct {
	EventID     string               `json:"event_id"`
	CompanyID   uuid.UUID            `json:"company_id"`
	CompanySlug string               `json:"company_slug"`
	CrawlRunID  uuid.UUID            `json:"crawl_run_id"`
	Listing     crawler.RawJobListing `json:"listing"`
	CrawledAt   time.Time            `json:"crawled_at"`
}

// ProcessedEvent is published to jobs.processed after a job is deduped, upserted, and enriched.
type ProcessedEvent struct {
	EventID   string    `json:"event_id"`
	JobID     uuid.UUID `json:"job_id"`
	CompanyID uuid.UUID `json:"company_id"`
	IsNew     bool      `json:"is_new"` // true if newly created, false if updated
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// AlertEvent is published to jobs.alerts when a new job matches a user's alert rules.
type AlertEvent struct {
	EventID string    `json:"event_id"`
	AlertID uuid.UUID `json:"alert_id"`
	UserID  uuid.UUID `json:"user_id"`
	JobID   uuid.UUID `json:"job_id"`
	Title   string    `json:"title"`
	Company string    `json:"company"`
	MatchedAt time.Time `json:"matched_at"`
}
