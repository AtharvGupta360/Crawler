package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/crawler"
	"github.com/google/uuid"
)

// PublisherAdapter adapts the Kafka Producer to the crawler.EventPublisher interface.
// This avoids circular imports between the crawler and kafka packages.
type PublisherAdapter struct {
	producer *Producer
}

// NewPublisherAdapter creates a new adapter wrapping the Kafka producer.
func NewPublisherAdapter(producer *Producer) *PublisherAdapter {
	return &PublisherAdapter{producer: producer}
}

// PublishRawListing publishes a raw job listing as a CrawlEvent to the jobs.raw topic.
func (a *PublisherAdapter) PublishRawListing(ctx context.Context, companyID uuid.UUID, companySlug string, crawlRunID uuid.UUID, listing crawler.RawJobListing) error {
	event := CrawlEvent{
		EventID:     fmt.Sprintf("crawl-%s", uuid.New().String()[:8]),
		CompanyID:   companyID,
		CompanySlug: companySlug,
		CrawlRunID:  crawlRunID,
		Listing:     listing,
		CrawledAt:   time.Now(),
	}

	return a.producer.PublishCrawlEvent(ctx, event)
}

// Compile-time check that PublisherAdapter implements EventPublisher.
var _ crawler.EventPublisher = (*PublisherAdapter)(nil)
