package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/segmentio/kafka-go"
)

// Indexer consumes processed job events from jobs.processed and indexes
// them into Elasticsearch for full-text search.
type Indexer struct {
	reader *kafka.Reader
	pg     *store.PostgresStore
	es     *store.ElasticStore
	logger *slog.Logger

	// Batching
	batchSize    int
	batchTimeout time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewIndexer creates a new Elasticsearch indexer consumer.
func NewIndexer(
	brokers []string,
	pg *store.PostgresStore,
	es *store.ElasticStore,
	logger *slog.Logger,
) *Indexer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          TopicJobsProcessed,
		GroupID:        GroupESIndexer,
		MinBytes:       1e3,  // 1KB
		MaxBytes:       10e6, // 10MB
		MaxWait:        2 * time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	return &Indexer{
		reader:       reader,
		pg:           pg,
		es:           es,
		logger:       logger.With("component", "kafka-indexer"),
		batchSize:    50,
		batchTimeout: time.Second,
		stopCh:       make(chan struct{}),
	}
}

// Start begins consuming messages from jobs.processed and indexing into ES.
func (idx *Indexer) Start(ctx context.Context) {
	idx.wg.Add(1)
	go func() {
		defer idx.wg.Done()
		idx.logger.Info("indexer started",
			"topic", TopicJobsProcessed,
			"group", GroupESIndexer,
			"batch_size", idx.batchSize,
		)

		batch := make([]ProcessedEvent, 0, idx.batchSize)
		timer := time.NewTimer(idx.batchTimeout)
		defer timer.Stop()

		for {
			select {
			case <-idx.stopCh:
				// Flush remaining batch before exit
				if len(batch) > 0 {
					idx.processBatch(ctx, batch)
				}
				idx.logger.Info("indexer stopping")
				return
			case <-ctx.Done():
				if len(batch) > 0 {
					idx.processBatch(context.Background(), batch)
				}
				idx.logger.Info("indexer context cancelled")
				return
			case <-timer.C:
				// Flush batch on timeout
				if len(batch) > 0 {
					idx.processBatch(ctx, batch)
					batch = batch[:0]
				}
				timer.Reset(idx.batchTimeout)
			default:
			}

			// Non-blocking read with a short timeout
			readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			msg, err := idx.reader.ReadMessage(readCtx)
			cancel()

			if err != nil {
				if readCtx.Err() != nil && ctx.Err() == nil {
					// Just a read timeout, not a real error
					continue
				}
				if ctx.Err() != nil {
					return // context cancelled
				}
				idx.logger.Error("failed to read message", "error", err)
				time.Sleep(time.Second)
				continue
			}

			var event ProcessedEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				idx.logger.Error("failed to unmarshal processed event",
					"error", err,
					"partition", msg.Partition,
					"offset", msg.Offset,
				)
				continue
			}

			batch = append(batch, event)

			// Flush if batch is full
			if len(batch) >= idx.batchSize {
				idx.processBatch(ctx, batch)
				batch = batch[:0]
				timer.Reset(idx.batchTimeout)
			}
		}
	}()
}

// Stop gracefully stops the indexer.
func (idx *Indexer) Stop() {
	close(idx.stopCh)
	idx.wg.Wait()
	if err := idx.reader.Close(); err != nil {
		idx.logger.Error("failed to close reader", "error", err)
	}
	idx.logger.Info("indexer stopped")
}

func (idx *Indexer) processBatch(ctx context.Context, events []ProcessedEvent) {
	idx.logger.Debug("processing batch", "size", len(events))

	// Fetch full jobs from PostgreSQL and index into ES
	jobs := make([]*models.Job, 0, len(events))
	for _, event := range events {
		job, err := idx.pg.GetJobByID(ctx, event.JobID)
		if err != nil {
			idx.logger.Error("failed to fetch job for indexing",
				"job_id", event.JobID,
				"error", err,
			)
			continue
		}
		if job == nil {
			idx.logger.Warn("job not found for indexing", "job_id", event.JobID)
			continue
		}
		jobs = append(jobs, job)
	}

	if len(jobs) == 0 {
		return
	}

	// Bulk index
	if err := idx.es.BulkIndexJobs(ctx, jobs); err != nil {
		idx.logger.Error("bulk index failed",
			"error", err,
			"batch_size", len(jobs),
		)
		// Fall back to individual indexing
		for _, job := range jobs {
			if err := idx.es.IndexJob(ctx, job); err != nil {
				idx.logger.Error("individual index failed",
					"job_id", job.ID,
					"error", err,
				)
			}
		}
		return
	}

	idx.logger.Info("batch indexed",
		"jobs", len(jobs),
	)
}
