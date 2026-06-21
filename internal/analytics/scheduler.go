package analytics

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/store"
)

// Scheduler periodically materializes trend snapshots.
type Scheduler struct {
	pg       *store.PostgresStore
	interval time.Duration
	limit    int
	logger   *slog.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewScheduler creates a trend snapshot scheduler.
func NewScheduler(pg *store.PostgresStore, interval time.Duration, limit int, logger *slog.Logger) *Scheduler {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if limit <= 0 {
		limit = 100
	}
	return &Scheduler{
		pg:       pg,
		interval: interval,
		limit:    limit,
		logger:   logger.With("component", "trend-scheduler"),
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic snapshot loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("trend scheduler started", "interval", s.interval, "limit", s.limit)

		s.refresh(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.refresh(ctx)
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop shuts down the scheduler.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("trend scheduler stopped")
}

func (s *Scheduler) refresh(ctx context.Context) {
	result, err := s.pg.RefreshTrendSnapshots(ctx, time.Now().UTC(), s.limit)
	if err != nil {
		s.logger.Error("trend snapshot refresh failed", "error", err)
		return
	}
	s.logger.Info("trend snapshot refreshed",
		"snapshot_date", result.SnapshotDate.Format("2006-01-02"),
		"skills", result.Skills,
	)
}
