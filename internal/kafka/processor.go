package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/enricher"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// Processor consumes raw crawl events from jobs.raw, deduplicates,
// enriches, upserts into PostgreSQL, and publishes processed events to jobs.processed.
type Processor struct {
	reader   *kafka.Reader
	producer *Producer
	pg       *store.PostgresStore
	redis    *store.RedisStore
	enricher *enricher.Pipeline
	logger   *slog.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewProcessor creates a new job processor consumer.
// enrich may be nil — in that case no enrichment runs (raw jobs stored as-is).
func NewProcessor(
	brokers []string,
	producer *Producer,
	pg *store.PostgresStore,
	redis *store.RedisStore,
	enrich *enricher.Pipeline,
	logger *slog.Logger,
) *Processor {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          TopicJobsRaw,
		GroupID:        GroupJobProcessor,
		MinBytes:       1e3,  // 1KB
		MaxBytes:       10e6, // 10MB
		MaxWait:        2 * time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	return &Processor{
		reader:   reader,
		producer: producer,
		pg:       pg,
		redis:    redis,
		enricher: enrich,
		logger:   logger.With("component", "kafka-processor"),
		stopCh:   make(chan struct{}),
	}
}

// Start begins consuming messages from jobs.raw in a background goroutine.
func (p *Processor) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.logger.Info("processor started", "topic", TopicJobsRaw, "group", GroupJobProcessor)

		for {
			select {
			case <-p.stopCh:
				p.logger.Info("processor stopping")
				return
			case <-ctx.Done():
				p.logger.Info("processor context cancelled")
				return
			default:
			}

			// ReadMessage blocks until a message is available or context is done
			msg, err := p.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return // context cancelled, clean exit
				}
				p.logger.Error("failed to read message", "error", err)
				time.Sleep(time.Second) // back off on errors
				continue
			}

			p.processMessage(ctx, msg)
		}
	}()
}

// Stop gracefully stops the processor.
func (p *Processor) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	if err := p.reader.Close(); err != nil {
		p.logger.Error("failed to close reader", "error", err)
	}
	p.logger.Info("processor stopped")
}

func (p *Processor) processMessage(ctx context.Context, msg kafka.Message) {
	var event CrawlEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		p.logger.Error("failed to unmarshal crawl event",
			"error", err,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)
		return
	}

	p.logger.Debug("processing crawl event",
		"event_id", event.EventID,
		"company", event.CompanySlug,
		"title", event.Listing.Title,
	)

	// 1. Check dedup via Redis
	contentHash := event.Listing.ContentHash()
	seen, _ := p.redis.IsContentSeen(ctx, contentHash)
	if seen {
		p.logger.Debug("content already seen, skipping",
			"content_hash", contentHash,
			"title", event.Listing.Title,
		)
		return
	}

	// 2. Convert to Job model
	job := event.Listing.ToJob(event.CompanyID)

	// 3. Enrich (rules + optional AI)
	if p.enricher != nil {
		p.enricher.Enrich(ctx, &job)
	}

	// 4. Upsert into PostgreSQL
	isNew, err := p.pg.UpsertJob(ctx, &job)
	if err != nil {
		p.logger.Error("failed to upsert job",
			"title", event.Listing.Title,
			"error", err,
		)
		return
	}

	// 5. Mark content as seen in Redis
	p.redis.MarkContentSeen(ctx, contentHash)

	// 6. Publish ProcessedEvent
	processedEvent := ProcessedEvent{
		EventID:   fmt.Sprintf("proc-%s", uuid.New().String()[:8]),
		JobID:     job.ID,
		CompanyID: event.CompanyID,
		IsNew:     isNew,
		Title:     job.Title,
		CreatedAt: time.Now(),
	}

	if err := p.producer.PublishProcessedEvent(ctx, processedEvent); err != nil {
		p.logger.Error("failed to publish processed event",
			"job_id", job.ID,
			"error", err,
		)
		// Don't return — the job was already upserted, we just failed to publish downstream
	}

	action := "updated"
	if isNew {
		action = "created"
	}
	p.logger.Info("job processed",
		"action", action,
		"title", job.Title,
		"job_id", job.ID,
		"company", event.CompanySlug,
	)
}
