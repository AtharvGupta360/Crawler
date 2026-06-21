package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/google/uuid"
	kafka "github.com/segmentio/kafka-go"
)

// AlertEvaluator consumes jobs.processed, matches new jobs against active
// alert rules, writes notifications to PG, and publishes AlertEvents.
type AlertEvaluator struct {
	reader   *kafka.Reader
	producer *Producer
	pg       *store.PostgresStore
	redis    *store.RedisStore
	logger   *slog.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewAlertEvaluator creates and returns a new AlertEvaluator.
func NewAlertEvaluator(
	brokers []string,
	producer *Producer,
	pg *store.PostgresStore,
	redis *store.RedisStore,
	logger *slog.Logger,
) *AlertEvaluator {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          TopicJobsProcessed,
		GroupID:        GroupAlertEval,
		MinBytes:       1e3,
		MaxBytes:       10e6,
		MaxWait:        2 * time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})
	return &AlertEvaluator{
		reader:   reader,
		producer: producer,
		pg:       pg,
		redis:    redis,
		logger:   logger.With("component", "alert-evaluator"),
		stopCh:   make(chan struct{}),
	}
}

// Start launches the consumer loop in a background goroutine.
func (e *AlertEvaluator) Start(ctx context.Context) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.logger.Info("alert evaluator started", "topic", TopicJobsProcessed)
		for {
			select {
			case <-e.stopCh:
				return
			case <-ctx.Done():
				return
			default:
			}

			msg, err := e.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				e.logger.Error("alert evaluator: read error", "error", err)
				time.Sleep(time.Second)
				continue
			}
			e.handleMessage(ctx, msg)
		}
	}()
}

// Stop gracefully shuts down the evaluator.
func (e *AlertEvaluator) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	e.reader.Close() //nolint:errcheck
	e.logger.Info("alert evaluator stopped")
}

func (e *AlertEvaluator) handleMessage(ctx context.Context, msg kafka.Message) {
	var event ProcessedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		e.logger.Error("alert evaluator: unmarshal error", "error", err)
		return
	}

	// Only evaluate truly new jobs to avoid re-alerting on updates
	if !event.IsNew {
		return
	}

	// Fetch the full job from PG
	job, err := e.pg.GetJobByID(ctx, event.JobID)
	if err != nil || job == nil {
		e.logger.Warn("alert evaluator: job not found", "job_id", event.JobID)
		return
	}

	// Load active alerts — try Redis cache first
	alerts, err := e.loadActiveAlerts(ctx)
	if err != nil {
		e.logger.Error("alert evaluator: failed to load alerts", "error", err)
		return
	}

	companyName := ""
	if job.Company != nil {
		companyName = job.Company.Name
	}

	for _, alert := range alerts {
		if !matchesAlert(job, alert) {
			continue
		}

		e.logger.Info("alert matched",
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
		if err := e.pg.CreateNotification(ctx, notif); err != nil {
			e.logger.Error("alert evaluator: failed to create notification", "error", err)
		}

		// Stamp last_triggered
		e.pg.UpdateAlertTriggered(ctx, alert.ID) //nolint:errcheck

		// Publish AlertEvent to jobs.alerts
		alertEvent := AlertEvent{
			EventID:   fmt.Sprintf("alert-%s", uuid.New().String()[:8]),
			AlertID:   alert.ID,
			UserID:    alert.UserID,
			JobID:     job.ID,
			Title:     job.Title,
			Company:   companyName,
			MatchedAt: time.Now(),
		}
		if err := e.producer.PublishAlertEvent(ctx, alertEvent); err != nil {
			e.logger.Error("alert evaluator: failed to publish alert event", "error", err)
		}
	}
}

// loadActiveAlerts returns all active alerts, using a Redis cache to avoid
// hammering PG on every processed event. TTL is 60 seconds.
func (e *AlertEvaluator) loadActiveAlerts(ctx context.Context) ([]models.Alert, error) {
	const cacheKey = "cache:alerts:active"
	const cacheTTL = 60 * time.Second

	// Try cache
	if cached, err := e.redis.GetJSON(ctx, cacheKey); err == nil && cached != nil {
		var alerts []models.Alert
		if json.Unmarshal(cached, &alerts) == nil {
			return alerts, nil
		}
	}

	// Cache miss — query PG
	alerts, err := e.pg.ListActiveAlerts(ctx)
	if err != nil {
		return nil, err
	}

	// Populate cache (best-effort)
	if b, err := json.Marshal(alerts); err == nil {
		e.redis.SetJSON(ctx, cacheKey, b, cacheTTL) //nolint:errcheck
	}

	return alerts, nil
}

// matchesAlert checks whether a job satisfies all non-empty filter criteria
// in an alert. Multiple criteria are AND-ed; items within a list are OR-ed.
func matchesAlert(job *models.Job, alert models.Alert) bool {
	f := alert.Filters

	// --- skills ---
	if raw, ok := f["skills"]; ok {
		filterSkills := toStringSlice(raw)
		if len(filterSkills) > 0 {
			jobSkills := collectJobSkills(job)
			if !anyOverlap(filterSkills, jobSkills) {
				return false
			}
		}
	}

	// --- seniority ---
	if raw, ok := f["seniority"]; ok {
		filterSeniority := toStringSlice(raw)
		if len(filterSeniority) > 0 {
			if job.SeniorityLevel == "" || !containsCI(filterSeniority, job.SeniorityLevel) {
				return false
			}
		}
	}

	// --- location_type ---
	if raw, ok := f["location_type"]; ok {
		filterLoc := toStringSlice(raw)
		if len(filterLoc) > 0 {
			if job.LocationType == "" || !containsCI(filterLoc, job.LocationType) {
				return false
			}
		}
	}

	// --- keyword (substring in title or AI summary) ---
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

// collectJobSkills gathers all skill names from required + preferred lists.
func collectJobSkills(job *models.Job) []string {
	var skills []string
	for _, s := range job.SkillsRequired {
		skills = append(skills, strings.ToLower(s.Name))
	}
	for _, s := range job.SkillsPreferred {
		skills = append(skills, strings.ToLower(s.Name))
	}
	return skills
}

// anyOverlap returns true if any element in a exists (case-insensitive) in b.
func anyOverlap(a, b []string) bool {
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

// containsCI reports whether target exists in list (case-insensitive).
func containsCI(list []string, target string) bool {
	targetLower := strings.ToLower(target)
	for _, v := range list {
		if strings.ToLower(v) == targetLower {
			return true
		}
	}
	return false
}

// toStringSlice converts an interface{} to []string, handling both
// []string and []interface{} (the latter coming from JSON unmarshalling).
func toStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
