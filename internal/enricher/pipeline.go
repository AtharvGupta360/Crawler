package enricher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// Pipeline composes rule-based and AI enrichment into a single step.
// Rule enrichment runs first (always), AI enrichment runs second (if configured).
// Both are safe to call concurrently.
type Pipeline struct {
	ai     *AIEnricher
	logger *slog.Logger

	// Rate limiting for AI calls: max N calls per second
	aiMu       sync.Mutex
	aiLastCall time.Time
	aiMinGap   time.Duration // minimum time between AI calls
}

// NewPipeline creates an enrichment pipeline.
// ai may be nil — in that case only rule-based enrichment runs.
func NewPipeline(ai *AIEnricher, aiCallsPerSecond float64, logger *slog.Logger) *Pipeline {
	gap := time.Duration(0)
	if aiCallsPerSecond > 0 {
		gap = time.Duration(float64(time.Second) / aiCallsPerSecond)
	}

	mode := "rules-only"
	if ai != nil {
		mode = "rules + AI (Gemini)"
	}
	logger.Info("enrichment pipeline ready", "mode", mode, "ai_rate_limit", aiCallsPerSecond)

	return &Pipeline{
		ai:       ai,
		logger:   logger.With("component", "enricher"),
		aiMinGap: gap,
	}
}

// Enrich runs the full enrichment pipeline on a job in-place.
// It is safe to call from multiple goroutines.
func (p *Pipeline) Enrich(ctx context.Context, job *models.Job) {
	// 1. Rule-based enrichment (always, instant)
	EnrichRules(job)

	// 2. AI enrichment (if configured, rate-limited)
	if p.ai == nil {
		return
	}

	// Simple client-side rate limiting
	p.aiMu.Lock()
	since := time.Since(p.aiLastCall)
	if since < p.aiMinGap {
		wait := p.aiMinGap - since
		p.aiMu.Unlock()
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}
		p.aiMu.Lock()
	}
	p.aiLastCall = time.Now()
	p.aiMu.Unlock()

	if err := p.ai.EnrichJob(ctx, job); err != nil {
		p.logger.Warn("AI enrichment failed, using rule-based only",
			"job_id", job.ID,
			"title", job.Title,
			"error", err,
		)
	}
}
