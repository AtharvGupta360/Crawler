package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// AshbyCrawler fetches jobs from Ashby's public Job Board API.
// Ashby uses a JSON RPC-style API for their public job boards.
//
// Endpoint:
//   POST https://api.ashbyhq.com/posting-api/job-board/{boardSlug}  — list all jobs with details
type AshbyCrawler struct {
	client      *http.Client
	rateLimiter *RateLimiter
	breaker     *CircuitBreaker
	logger      *slog.Logger
}

// NewAshbyCrawler creates a new Ashby crawler.
func NewAshbyCrawler(rateLimiter *RateLimiter, breaker *CircuitBreaker, userAgent string, logger *slog.Logger) *AshbyCrawler {
	return &AshbyCrawler{
		client:      newHTTPClient(userAgent),
		rateLimiter: rateLimiter,
		breaker:     breaker,
		logger:      logger.With("crawler", "ashby"),
	}
}

func (a *AshbyCrawler) Name() string { return models.ATSAshby }

func (a *AshbyCrawler) HealthCheck(ctx context.Context) error {
	// Test with a known board
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.ashbyhq.com/posting-api/job-board/ashby", nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("ashby health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("ashby returned status %d", resp.StatusCode)
	}
	return nil
}

// CrawlCompany fetches all job postings for a company from Ashby.
// The company's Slug field should be the Ashby board slug.
func (a *AshbyCrawler) CrawlCompany(ctx context.Context, company models.Company) ([]RawJobListing, error) {
	domain := "api.ashbyhq.com"

	// Check circuit breaker
	if a.breaker.IsOpen(domain) {
		return nil, fmt.Errorf("circuit breaker open for %s", domain)
	}

	if err := a.rateLimiter.Wait(ctx, domain); err != nil {
		return nil, err
	}

	// Ashby has two approaches:
	// 1. GET /posting-api/job-board/{slug} — returns job board with jobs (basic info)
	// 2. POST /posting-api/job-board/{slug}/jobs — for detailed job info
	//
	// We use approach 1 first, then fetch individual job details if needed.

	url := fmt.Sprintf("https://api.ashbyhq.com/posting-api/job-board/%s", company.Slug)
	a.logger.Info("fetching job board", "company", company.Name, "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		a.breaker.RecordFailure(domain)
		return nil, fmt.Errorf("fetching ashby board: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		a.breaker.RecordFailure(domain)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var board ashbyBoardResponse
	if err := json.NewDecoder(resp.Body).Decode(&board); err != nil {
		a.breaker.RecordFailure(domain)
		return nil, fmt.Errorf("decoding ashby response: %w", err)
	}

	a.logger.Info("found jobs", "company", company.Name, "count", len(board.Jobs))

	// Fetch individual job details for full descriptions (only if not already in listing)
	var listings []RawJobListing
	for _, job := range board.Jobs {
		// Use inline description from the board listing if available (Ashby v2 API)
		descHTML := job.DescriptionHTML

		// Only fetch details if the listing didn't include description
		if descHTML == "" {
			if err := a.rateLimiter.Wait(ctx, domain); err != nil {
				return listings, err
			}

			detail, err := a.fetchJobDetail(ctx, job.ID)
			if err != nil {
				a.logger.Warn("failed to fetch ashby job detail",
					"job_id", job.ID,
					"title", job.Title,
					"error", err,
				)
				// Fall back to basic info (no description)
			} else {
				descHTML = detail.DescriptionHTML
			}
		}

		listings = append(listings, RawJobListing{
			ExternalID:      job.ID,
			Title:           job.Title,
			DescriptionHTML: descHTML,
			Location:        job.Location,
			Department:      job.Department,
			Team:            job.Team,
			ApplyURL:        fmt.Sprintf("https://jobs.ashbyhq.com/%s/%s", company.Slug, job.ID),
			SourceURL:       url,
			EmploymentType:  normalizeAshbyEmploymentType(job.EmploymentType),
		})
	}

	a.breaker.RecordSuccess(domain)
	return listings, nil
}

// fetchJobDetail gets the full description for a single Ashby job.
func (a *AshbyCrawler) fetchJobDetail(ctx context.Context, jobID string) (*ashbyJobDetail, error) {
	url := "https://api.ashbyhq.com/posting-api/job-posting/" + jobID

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var detail ashbyJobDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("decoding job detail: %w", err)
	}

	return &detail.Info, nil
}

// ─────────────────────────────────────────────
// Ashby API response types
// ─────────────────────────────────────────────

type ashbyBoardResponse struct {
	Jobs []ashbyJob `json:"jobs"`
}

type ashbyJob struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	DescriptionHTML string `json:"descriptionHtml"` // Available inline in v2 API responses
	Location        string `json:"location"`
	Department      string `json:"department"`
	Team            string `json:"team"`
	EmploymentType  string `json:"employmentType"`
	IsListed        bool   `json:"isListed"`
}

type ashbyJobDetailResponse struct {
	Info ashbyJobDetail `json:"info"`
}

type ashbyJobDetail struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	DescriptionHTML string `json:"descriptionHtml"`
	Location        string `json:"location"`
	Department      string `json:"departmentName"`
	Team            string `json:"teamName"`
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func normalizeAshbyEmploymentType(t string) string {
	switch t {
	case "FullTime":
		return models.EmploymentFullTime
	case "PartTime":
		return models.EmploymentPartTime
	case "Intern":
		return models.EmploymentIntern
	case "Contract":
		return models.EmploymentContract
	default:
		return t
	}
}
