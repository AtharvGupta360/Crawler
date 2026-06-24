package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/crawler"
	"github.com/AtharvGupta360/JobCrawl/internal/enricher"
	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/AtharvGupta360/JobCrawl/internal/ws"
	"github.com/google/uuid"
)

// SyncAdapter implements crawler.SyncProcessor by running enrichment,
// alert evaluation, ES indexing, and WebSocket notifications inline —
// the same pipeline that normally flows through Kafka topics.
type SyncAdapter struct {
	pg       *store.PostgresStore
	redis    *store.RedisStore
	enricher *enricher.Pipeline
	elastic  *store.ElasticStore // nil = ES disabled
	wsHub    *ws.Hub            // nil = WS disabled
	logger   *slog.Logger
}

// NewSyncAdapter creates a SyncAdapter. elastic and wsHub may be nil.
func NewSyncAdapter(
	pg *store.PostgresStore,
	redis *store.RedisStore,
	enrich *enricher.Pipeline,
	elastic *store.ElasticStore,
	wsHub *ws.Hub,
	logger *slog.Logger,
) *SyncAdapter {
	return &SyncAdapter{
		pg:       pg,
		redis:    redis,
		enricher: enrich,
		elastic:  elastic,
		wsHub:    wsHub,
		logger:   logger.With("component", "sync-adapter"),
	}
}

// ProcessSync enriches, indexes into ES, evaluates alerts, creates
// notifications, and pushes WebSocket messages — all synchronously.
func (s *SyncAdapter) ProcessSync(ctx context.Context, job *models.Job, isNew bool) error {
	// 1. Enrich (rules + optional AI)
	if s.enricher != nil {
		s.enricher.Enrich(ctx, job)

		// Persist enriched fields back to PG
		if err := s.pg.UpdateJobEnrichment(ctx, job); err != nil {
			s.logger.Warn("sync: failed to persist enrichment", "job_id", job.ID, "error", err)
		}
	}

	// 2. Index into Elasticsearch
	if s.elastic != nil {
		if err := s.elastic.IndexJob(ctx, job); err != nil {
			s.logger.Warn("sync: ES index failed", "job_id", job.ID, "error", err)
			// Non-fatal — continue
		}
	}

	// 3. Evaluate alerts (only for new jobs)
	if isNew {
		s.evaluateAlerts(ctx, job)
	}

	return nil
}

// evaluateAlerts mirrors the Kafka AlertEvaluator's logic without Kafka.
func (s *SyncAdapter) evaluateAlerts(ctx context.Context, job *models.Job) {
	alerts, err := s.loadActiveAlerts(ctx)
	if err != nil {
		s.logger.Warn("sync: failed to load alerts", "error", err)
		return
	}

	companyName := ""
	if job.Company != nil {
		companyName = job.Company.Name
	}

	for _, alert := range alerts {
		if !s.matchesAlert(job, alert) {
			continue
		}

		s.logger.Info("sync: alert matched",
			"alert_id", alert.ID,
			"user_id", alert.UserID,
			"job_title", job.Title,
		)

		// Persist notification
		alertID := alert.ID
		jobID := job.ID
		notif := &models.Notification{
			UserID:   alert.UserID,
			AlertID:  &alertID,
			JobID:    &jobID,
			Title:    job.Title,
			Company:  companyName,
			ApplyURL: job.ApplyURL,
		}
		if err := s.pg.CreateNotification(ctx, notif); err != nil {
			s.logger.Warn("sync: failed to create notification", "error", err)
		}

		// Stamp last_triggered
		s.pg.UpdateAlertTriggered(ctx, alert.ID) //nolint:errcheck

		// Push via WebSocket (if hub is available)
		if s.wsHub != nil {
			payload := wsPayload{
				Type:      "job_alert",
				AlertID:   alert.ID.String(),
				JobID:     job.ID.String(),
				Title:     job.Title,
				Company:   companyName,
				MatchedAt: time.Now(),
			}
			s.wsHub.SendToUser(alert.UserID, payload)
		}
	}
}

// loadActiveAlerts reads alerts using the same Redis cache as the Kafka evaluator.
func (s *SyncAdapter) loadActiveAlerts(ctx context.Context) ([]models.Alert, error) {
	const cacheKey = "cache:alerts:active"
	const cacheTTL = 60 * time.Second

	if cached, err := s.redis.GetJSON(ctx, cacheKey); err == nil && cached != nil {
		var alerts []models.Alert
		if json.Unmarshal(cached, &alerts) == nil {
			return alerts, nil
		}
	}

	alerts, err := s.pg.ListActiveAlerts(ctx)
	if err != nil {
		return nil, err
	}

	if b, err := json.Marshal(alerts); err == nil {
		s.redis.SetJSON(ctx, cacheKey, b, cacheTTL) //nolint:errcheck
	}

	return alerts, nil
}

// matchesAlert checks whether a job satisfies all filter criteria in an alert.
// This is identical to the logic in alert_evaluator.go.
func (s *SyncAdapter) matchesAlert(job *models.Job, alert models.Alert) bool {
	f := alert.Filters

	if raw, ok := f["skills"]; ok {
		filterSkills := toStringSlice(raw)
		if len(filterSkills) > 0 {
			jobSkills := s.collectJobSkills(job)
			if !s.anyOverlap(filterSkills, jobSkills) {
				return false
			}
		}
	}

	if raw, ok := f["seniority"]; ok {
		filterSeniority := toStringSlice(raw)
		if len(filterSeniority) > 0 {
			if job.SeniorityLevel == "" || !containsCI(filterSeniority, job.SeniorityLevel) {
				return false
			}
		}
	}

	if raw, ok := f["location_type"]; ok {
		filterLoc := toStringSlice(raw)
		if len(filterLoc) > 0 {
			if job.LocationType == "" || !containsCI(filterLoc, job.LocationType) {
				return false
			}
		}
	}

	if raw, ok := f["keyword"]; ok {
		if kw, _ := raw.(string); kw != "" {
			kwLower := strings.ToLower(kw)
			titleLower := strings.ToLower(job.Title)
			summaryLower := strings.ToLower(job.AISummary)
			if !strings.Contains(titleLower, kwLower) && !strings.Contains(summaryLower, kwLower) {
				return false
			}
		}
	}

	return true
}

func (s *SyncAdapter) collectJobSkills(job *models.Job) []string {
	var skills []string
	for _, sk := range job.SkillsRequired {
		skills = append(skills, strings.ToLower(sk.Name))
	}
	for _, sk := range job.SkillsPreferred {
		skills = append(skills, strings.ToLower(sk.Name))
	}
	return skills
}

func (s *SyncAdapter) anyOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[strings.ToLower(v)] = struct{}{}
	}
	for _, v := range a {
		if _, ok := set[strings.ToLower(v)]; ok {
			return true
		}
	}
	return false
}

// Compile-time check that SyncAdapter implements SyncProcessor.
var _ crawler.SyncProcessor = (*SyncAdapter)(nil)

// suppress unused import warning — uuid is used for alert/notification IDs
var _ = fmt.Sprintf
var _ = uuid.New
