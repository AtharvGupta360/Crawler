package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

// Producer wraps a Kafka writer for publishing events to topics.
type Producer struct {
	rawWriter       *kafka.Writer
	processedWriter *kafka.Writer
	alertWriter     *kafka.Writer
	logger          *slog.Logger
}

// NewProducer creates a new Kafka producer that writes to the configured topics.
func NewProducer(brokers []string, logger *slog.Logger) *Producer {
	rawWriter := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        TopicJobsRaw,
		Balancer:     &kafka.Hash{}, // Key-based partitioning (by company slug)
		RequiredAcks: kafka.RequireOne,
		Async:        false, // Synchronous for reliability
		Logger:       kafka.LoggerFunc(func(msg string, args ...interface{}) {}), // suppress verbose logs
	}

	processedWriter := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        TopicJobsProcessed,
		Balancer:     &kafka.Hash{}, // Key-based partitioning (by job ID)
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		Logger:       kafka.LoggerFunc(func(msg string, args ...interface{}) {}),
	}

	alertWriter := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        TopicJobsAlerts,
		Balancer:     &kafka.Hash{}, // Key-based by user UUID
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		Logger:       kafka.LoggerFunc(func(msg string, args ...interface{}) {}),
	}

	logger.Info("Kafka producer initialized",
		"brokers", brokers,
		"topics", []string{TopicJobsRaw, TopicJobsProcessed, TopicJobsAlerts},
	)

	return &Producer{
		rawWriter:       rawWriter,
		processedWriter: processedWriter,
		alertWriter:     alertWriter,
		logger:          logger.With("component", "kafka-producer"),
	}
}

// PublishCrawlEvent publishes a raw crawl event to the jobs.raw topic.
// The message is keyed by company slug for per-company ordering.
func (p *Producer) PublishCrawlEvent(ctx context.Context, event CrawlEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling crawl event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(event.CompanySlug),
		Value: data,
	}

	if err := p.rawWriter.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("failed to publish crawl event",
			"topic", TopicJobsRaw,
			"company", event.CompanySlug,
			"error", err,
		)
		return fmt.Errorf("publishing to %s: %w", TopicJobsRaw, err)
	}

	p.logger.Debug("published crawl event",
		"topic", TopicJobsRaw,
		"company", event.CompanySlug,
		"event_id", event.EventID,
	)
	return nil
}

// PublishProcessedEvent publishes a processed job event to the jobs.processed topic.
// The message is keyed by job UUID for per-job ordering.
func (p *Producer) PublishProcessedEvent(ctx context.Context, event ProcessedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling processed event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(event.JobID.String()),
		Value: data,
	}

	if err := p.processedWriter.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("failed to publish processed event",
			"topic", TopicJobsProcessed,
			"job_id", event.JobID,
			"error", err,
		)
		return fmt.Errorf("publishing to %s: %w", TopicJobsProcessed, err)
	}

	p.logger.Debug("published processed event",
		"topic", TopicJobsProcessed,
		"job_id", event.JobID,
		"is_new", event.IsNew,
	)
	return nil
}

// PublishAlertEvent publishes an alert notification event to the jobs.alerts topic.
// The message is keyed by user UUID so all alerts for one user land on the same partition.
func (p *Producer) PublishAlertEvent(ctx context.Context, event AlertEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling alert event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(event.UserID.String()),
		Value: data,
	}

	if err := p.alertWriter.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("failed to publish alert event",
			"topic", TopicJobsAlerts,
			"user_id", event.UserID,
			"error", err,
		)
		return fmt.Errorf("publishing to %s: %w", TopicJobsAlerts, err)
	}

	p.logger.Debug("published alert event",
		"topic", TopicJobsAlerts,
		"user_id", event.UserID,
		"alert_id", event.AlertID,
	)
	return nil
}

// Close gracefully shuts down the producer, flushing pending messages.
func (p *Producer) Close() error {
	p.logger.Info("closing Kafka producer")
	var errs []error
	if err := p.rawWriter.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing raw writer: %w", err))
	}
	if err := p.processedWriter.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing processed writer: %w", err))
	}
	if err := p.alertWriter.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing alert writer: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("producer close errors: %v", errs)
	}
	return nil
}
