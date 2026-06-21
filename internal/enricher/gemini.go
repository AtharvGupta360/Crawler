package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"google.golang.org/genai"
)

// ─────────────────────────────────────────────
// AI enricher (Gemini)
// Called only when GEMINI_API_KEY is set.
// Extracts structured data from job description text.
// ─────────────────────────────────────────────

// AIEnricher uses the Gemini API to extract structured fields from job descriptions.
type AIEnricher struct {
	client *genai.Client
	model  string
	logger *slog.Logger
}

// NewAIEnricher creates a Gemini-backed enricher.
// Returns nil if apiKey is empty (graceful degradation).
func NewAIEnricher(ctx context.Context, apiKey string, logger *slog.Logger) (*AIEnricher, error) {
	if apiKey == "" {
		return nil, nil
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	logger.Info("AI enricher initialized", "model", "gemini-2.0-flash")

	return &AIEnricher{
		client: client,
		model:  "gemini-2.0-flash",
		logger: logger.With("component", "ai-enricher"),
	}, nil
}

// aiJobData is what we ask Gemini to extract.
type aiJobData struct {
	SeniorityLevel     string     `json:"seniority_level"`      // intern|junior|mid|senior|lead|staff|""
	LocationType       string     `json:"location_type"`        // remote|hybrid|onsite|""
	EmploymentType     string     `json:"employment_type"`      // full_time|part_time|intern|contract|""
	SkillsRequired     []aiSkill  `json:"skills_required"`
	SkillsPreferred    []aiSkill  `json:"skills_preferred"`
	ExperienceYearsMin *int       `json:"experience_years_min"` // null if not mentioned
	ExperienceYearsMax *int       `json:"experience_years_max"` // null if not mentioned
	SalaryMin          *int       `json:"salary_min"`           // annual USD, null if not mentioned
	SalaryMax          *int       `json:"salary_max"`           // annual USD, null if not mentioned
	EducationLevel     string     `json:"education_level"`      // bachelor|master|phd|bootcamp|""
	Summary            string     `json:"summary"`              // 2-3 sentence plain-text summary
}

type aiSkill struct {
	Name     string `json:"name"`     // e.g. "Go", "PostgreSQL"
	Category string `json:"category"` // language|framework|tool|concept|cloud
}

var enrichPrompt = strings.TrimSpace(`
You are a job data extraction assistant. Given a job description, extract the following fields as JSON.
Be concise. Use null for numeric fields if not mentioned. Use "" for string fields if not mentioned.
Only include skills that are explicitly mentioned or clearly implied.

Required JSON schema:
{
  "seniority_level": "<intern|junior|mid|senior|lead|staff|>",
  "location_type": "<remote|hybrid|onsite|>",
  "employment_type": "<full_time|part_time|intern|contract|>",
  "skills_required": [{"name": "...", "category": "<language|framework|tool|concept|cloud>"}],
  "skills_preferred": [{"name": "...", "category": "..."}],
  "experience_years_min": <int or null>,
  "experience_years_max": <int or null>,
  "salary_min": <annual USD int or null>,
  "salary_max": <annual USD int or null>,
  "education_level": "<bachelor|master|phd|bootcamp|>",
  "summary": "<2-3 sentence plain English summary of the role>"
}

Respond with ONLY valid JSON. No markdown fences, no explanation.
`)

// EnrichJob calls Gemini to extract structured data and merges it into the job.
// Fields already populated by the rule-based enricher are only overwritten if
// the AI extraction found something more specific.
func (e *AIEnricher) EnrichJob(ctx context.Context, job *models.Job) error {
	// Skip very short descriptions — not worth an API call
	if len(job.DescriptionClean) < 200 {
		e.logger.Debug("description too short for AI enrichment, skipping",
			"job_id", job.ID,
			"len", len(job.DescriptionClean),
		)
		return nil
	}

	// Build prompt: title + cleaned description (capped to avoid token waste)
	desc := job.DescriptionClean
	if len(desc) > 4000 {
		desc = desc[:4000]
	}

	prompt := fmt.Sprintf("%s\n\nJob Title: %s\n\nJob Description:\n%s", enrichPrompt, job.Title, desc)

	resp, err := e.client.Models.GenerateContent(
		ctx,
		e.model,
		genai.Text(prompt),
		&genai.GenerateContentConfig{
			Temperature:     genai.Ptr[float32](0.1), // low temp for factual extraction
			MaxOutputTokens: 1024,
		},
	)
	if err != nil {
		return fmt.Errorf("gemini generate: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return fmt.Errorf("empty response from Gemini")
	}

	// Extract text from response
	var rawText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			rawText += part.Text
		}
	}
	rawText = strings.TrimSpace(rawText)

	// Strip markdown fences if the model ignored our instruction
	rawText = strings.TrimPrefix(rawText, "```json")
	rawText = strings.TrimPrefix(rawText, "```")
	rawText = strings.TrimSuffix(rawText, "```")
	rawText = strings.TrimSpace(rawText)

	var data aiJobData
	if err := json.Unmarshal([]byte(rawText), &data); err != nil {
		e.logger.Warn("failed to parse AI response",
			"job_id", job.ID,
			"raw", rawText[:min(len(rawText), 200)],
			"error", err,
		)
		return nil // non-fatal — partial enrichment is fine
	}

	// Merge into job — AI can override rule-based values where it found something specific
	if data.SeniorityLevel != "" {
		job.SeniorityLevel = data.SeniorityLevel
	}
	if data.LocationType != "" {
		job.LocationType = data.LocationType
	}
	if data.EmploymentType != "" {
		job.EmploymentType = data.EmploymentType
	}
	if data.EducationLevel != "" {
		job.EducationLevel = data.EducationLevel
	}
	if data.Summary != "" {
		job.AISummary = data.Summary
	}
	if data.ExperienceYearsMin != nil {
		job.ExperienceYearsMin = data.ExperienceYearsMin
	}
	if data.ExperienceYearsMax != nil {
		job.ExperienceYearsMax = data.ExperienceYearsMax
	}
	if data.SalaryMin != nil {
		job.SalaryMin = data.SalaryMin
		job.SalaryCurrency = "USD"
	}
	if data.SalaryMax != nil {
		job.SalaryMax = data.SalaryMax
	}

	// Map skills
	if len(data.SkillsRequired) > 0 {
		job.SkillsRequired = make([]models.SkillEntry, 0, len(data.SkillsRequired))
		for _, s := range data.SkillsRequired {
			if s.Name != "" {
				job.SkillsRequired = append(job.SkillsRequired, models.SkillEntry{
					Name:       s.Name,
					Category:   s.Category,
					Importance: "required",
				})
			}
		}
	}
	if len(data.SkillsPreferred) > 0 {
		job.SkillsPreferred = make([]models.SkillEntry, 0, len(data.SkillsPreferred))
		for _, s := range data.SkillsPreferred {
			if s.Name != "" {
				job.SkillsPreferred = append(job.SkillsPreferred, models.SkillEntry{
					Name:       s.Name,
					Category:   s.Category,
					Importance: "preferred",
				})
			}
		}
	}

	e.logger.Debug("AI enrichment applied",
		"job_id", job.ID,
		"seniority", job.SeniorityLevel,
		"location_type", job.LocationType,
		"skills_required", len(job.SkillsRequired),
		"skills_preferred", len(job.SkillsPreferred),
		"has_summary", job.AISummary != "",
	)

	return nil
}

// Close is a no-op — the Gemini Client has no Close method.
func (e *AIEnricher) Close() {}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
