package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// LeverCrawler fetches jobs from Lever's public Postings API.
// API docs: https://github.com/lever/postings-api
//
// Endpoint:
//   GET https://api.lever.co/v0/postings/{company}?mode=json — list all postings with full content
type LeverCrawler struct {
	client      *http.Client
	rateLimiter *RateLimiter
	breaker     *CircuitBreaker
	logger      *slog.Logger
}

// NewLeverCrawler creates a new Lever crawler.
func NewLeverCrawler(rateLimiter *RateLimiter, breaker *CircuitBreaker, userAgent string, logger *slog.Logger) *LeverCrawler {
	return &LeverCrawler{
		client:      newHTTPClient(userAgent),
		rateLimiter: rateLimiter,
		breaker:     breaker,
		logger:      logger.With("crawler", "lever"),
	}
}

func (l *LeverCrawler) Name() string { return models.ATSLever }

func (l *LeverCrawler) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.lever.co/v0/postings/lever?mode=json&limit=1", nil)
	if err != nil {
		return err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("lever health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lever returned status %d", resp.StatusCode)
	}
	return nil
}

// CrawlCompany fetches all job postings for a company from Lever.
// The company's Slug field should be the Lever company identifier.
// Lever returns full content in the list response, so we only need one API call.
func (l *LeverCrawler) CrawlCompany(ctx context.Context, company models.Company) ([]RawJobListing, error) {
	domain := "api.lever.co"

	// Check circuit breaker
	if l.breaker.IsOpen(domain) {
		return nil, fmt.Errorf("circuit breaker open for %s", domain)
	}

	if err := l.rateLimiter.Wait(ctx, domain); err != nil {
		return nil, err
	}

	// Lever returns all postings with content in a single call
	url := fmt.Sprintf("https://api.lever.co/v0/postings/%s?mode=json", company.Slug)
	l.logger.Info("fetching postings", "company", company.Name, "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		l.breaker.RecordFailure(domain)
		return nil, fmt.Errorf("fetching lever postings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		l.breaker.RecordFailure(domain)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var postings []leverPosting
	if err := json.NewDecoder(resp.Body).Decode(&postings); err != nil {
		l.breaker.RecordFailure(domain)
		return nil, fmt.Errorf("decoding lever response: %w", err)
	}

	l.logger.Info("found postings", "company", company.Name, "count", len(postings))

	// Convert to RawJobListing
	var listings []RawJobListing
	for _, p := range postings {
		listing := RawJobListing{
			ExternalID:      p.ID,
			Title:           p.Text,
			DescriptionHTML: buildLeverDescription(p),
			Location:        extractLeverLocation(p.Categories),
			Department:      extractLeverDepartment(p.Categories),
			Team:            extractLeverTeam(p.Categories),
			ApplyURL:        p.ApplyURL,
			SourceURL:       p.HostedURL,
			EmploymentType:  extractLeverCommitment(p.Categories),
		}

		listings = append(listings, listing)
	}

	l.breaker.RecordSuccess(domain)
	return listings, nil
}

// ─────────────────────────────────────────────
// Lever API response types
// ─────────────────────────────────────────────

type leverPosting struct {
	ID          string           `json:"id"`
	Text        string           `json:"text"`
	HostedURL   string           `json:"hostedUrl"`
	ApplyURL    string           `json:"applyUrl"`
	Categories  leverCategories  `json:"categories"`
	Description string           `json:"description"`       // Opening paragraph (plaintext)
	DescriptionPlain string      `json:"descriptionPlain"`
	Lists       []leverList      `json:"lists"`              // Sections like "Requirements", "Nice to have"
	Additional  string           `json:"additional"`          // Closing content (HTML)
	AdditionalPlain string       `json:"additionalPlain"`
	CreatedAt   int64            `json:"createdAt"`
}

type leverCategories struct {
	Location   string `json:"location"`
	Department string `json:"department"`
	Team       string `json:"team"`
	Commitment string `json:"commitment"` // "Full-time", "Part-time", "Intern"
}

type leverList struct {
	Text    string `json:"text"`    // Section title, e.g., "What you'll do"
	Content string `json:"content"` // HTML content
}

// ─────────────────────────────────────────────
// Extraction helpers
// ─────────────────────────────────────────────

// buildLeverDescription combines all content sections into a single HTML string.
func buildLeverDescription(p leverPosting) string {
	var parts []string

	if p.Description != "" {
		parts = append(parts, fmt.Sprintf("<div>%s</div>", p.Description))
	}

	for _, list := range p.Lists {
		parts = append(parts, fmt.Sprintf("<h3>%s</h3>%s", list.Text, list.Content))
	}

	if p.Additional != "" {
		parts = append(parts, p.Additional)
	}

	return strings.Join(parts, "\n")
}

func extractLeverLocation(c leverCategories) string    { return c.Location }
func extractLeverDepartment(c leverCategories) string   { return c.Department }
func extractLeverTeam(c leverCategories) string         { return c.Team }

func extractLeverCommitment(c leverCategories) string {
	switch strings.ToLower(c.Commitment) {
	case "full-time", "full time":
		return models.EmploymentFullTime
	case "part-time", "part time":
		return models.EmploymentPartTime
	case "intern", "internship":
		return models.EmploymentIntern
	case "contract", "contractor":
		return models.EmploymentContract
	default:
		return c.Commitment
	}
}
