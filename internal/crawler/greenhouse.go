package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// GreenhouseCrawler fetches jobs from Greenhouse's public Job Board API.
// API docs: https://developers.greenhouse.io/job-board.html
//
// Endpoints used:
//   GET https://boards-api.greenhouse.io/v1/boards/{board_token}/jobs         — list jobs
//   GET https://boards-api.greenhouse.io/v1/boards/{board_token}/jobs/{id}    — job detail with content
type GreenhouseCrawler struct {
	client      *http.Client
	rateLimiter *RateLimiter
	breaker     *CircuitBreaker
	logger      *slog.Logger
}

// NewGreenhouseCrawler creates a new Greenhouse crawler.
func NewGreenhouseCrawler(rateLimiter *RateLimiter, breaker *CircuitBreaker, userAgent string, logger *slog.Logger) *GreenhouseCrawler {
	return &GreenhouseCrawler{
		client:      newHTTPClient(userAgent),
		rateLimiter: rateLimiter,
		breaker:     breaker,
		logger:      logger.With("crawler", "greenhouse"),
	}
}

func (g *GreenhouseCrawler) Name() string { return models.ATSGreenhouse }

func (g *GreenhouseCrawler) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://boards-api.greenhouse.io/v1/boards/example/jobs", nil)
	if err != nil {
		return err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("greenhouse health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("greenhouse returned status %d", resp.StatusCode)
	}
	return nil
}

// CrawlCompany fetches all jobs for a company from Greenhouse.
// The company's Slug field should be the Greenhouse board token.
func (g *GreenhouseCrawler) CrawlCompany(ctx context.Context, company models.Company) ([]RawJobListing, error) {
	domain := "boards-api.greenhouse.io"

	// Check circuit breaker
	if g.breaker.IsOpen(domain) {
		return nil, fmt.Errorf("circuit breaker open for %s", domain)
	}

	boardToken := company.Slug

	// Step 1: Fetch job list
	if err := g.rateLimiter.Wait(ctx, domain); err != nil {
		return nil, err
	}

	listURL := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs", boardToken)
	g.logger.Info("fetching job list", "company", company.Name, "url", listURL)

	jobList, err := g.fetchJobList(ctx, listURL)
	if err != nil {
		g.breaker.RecordFailure(domain)
		return nil, fmt.Errorf("fetching job list for %s: %w", company.Name, err)
	}

	g.logger.Info("found jobs", "company", company.Name, "count", len(jobList))

	// Step 2: Fetch full details for each job
	var listings []RawJobListing
	for _, summary := range jobList {
		if err := g.rateLimiter.Wait(ctx, domain); err != nil {
			return listings, err // return what we have so far
		}

		detailURL := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs/%d", boardToken, summary.ID)
		detail, err := g.fetchJobDetail(ctx, detailURL)
		if err != nil {
			g.logger.Warn("failed to fetch job detail",
				"job_id", summary.ID,
				"title", summary.Title,
				"error", err,
			)
			continue // skip this job, don't fail the entire crawl
		}

		listing := RawJobListing{
			ExternalID:      fmt.Sprintf("%d", summary.ID),
			Title:           detail.Title,
			DescriptionHTML: detail.Content,
			Location:        extractGreenhouseLocation(detail.Location),
			Department:      extractGreenhouseDepartment(detail.Departments),
			ApplyURL:        detail.AbsoluteURL,
			SourceURL:       detailURL,
		}

		if detail.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339, detail.UpdatedAt); err == nil {
				listing.UpdatedAt = t
			}
		}

		listings = append(listings, listing)
	}

	g.breaker.RecordSuccess(domain)
	return listings, nil
}

// ─────────────────────────────────────────────
// Greenhouse API response types
// ─────────────────────────────────────────────

type greenhouseJobListResponse struct {
	Jobs []greenhouseJobSummary `json:"jobs"`
}

type greenhouseJobSummary struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type greenhouseJobDetail struct {
	ID          int64                        `json:"id"`
	Title       string                       `json:"title"`
	Content     string                       `json:"content"`
	AbsoluteURL string                       `json:"absolute_url"`
	UpdatedAt   string                       `json:"updated_at"`
	Location    *greenhouseLocation          `json:"location"`
	Departments []greenhouseDepartment       `json:"departments"`
	Metadata    []greenhouseMetadata         `json:"metadata"`
}

type greenhouseLocation struct {
	Name string `json:"name"`
}

type greenhouseDepartment struct {
	Name string `json:"name"`
}

type greenhouseMetadata struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// ─────────────────────────────────────────────
// HTTP helpers
// ─────────────────────────────────────────────

func (g *GreenhouseCrawler) fetchJobList(ctx context.Context, url string) ([]greenhouseJobSummary, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result greenhouseJobListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return result.Jobs, nil
}

func (g *GreenhouseCrawler) fetchJobDetail(ctx context.Context, url string) (*greenhouseJobDetail, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url+"?questions=true", nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var detail greenhouseJobDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &detail, nil
}

// ─────────────────────────────────────────────
// Extraction helpers
// ─────────────────────────────────────────────

func extractGreenhouseLocation(loc *greenhouseLocation) string {
	if loc == nil {
		return ""
	}
	return loc.Name
}

func extractGreenhouseDepartment(depts []greenhouseDepartment) string {
	if len(depts) == 0 {
		return ""
	}
	names := make([]string, len(depts))
	for i, d := range depts {
		names[i] = d.Name
	}
	return strings.Join(names, ", ")
}
