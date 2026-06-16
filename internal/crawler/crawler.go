package crawler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/google/uuid"
)

// Crawler defines the interface that every ATS-specific crawler must implement.
type Crawler interface {
	// Name returns the ATS platform identifier (e.g., "greenhouse", "lever", "ashby").
	Name() string

	// CrawlCompany fetches all active job listings from a company's career page.
	// Returns raw job listings that still need normalization and AI processing.
	CrawlCompany(ctx context.Context, company models.Company) ([]RawJobListing, error)

	// HealthCheck verifies the crawler can reach the ATS platform.
	HealthCheck(ctx context.Context) error
}

// RawJobListing is the intermediate representation of a job posting
// as extracted from an ATS, before normalization and AI processing.
type RawJobListing struct {
	ExternalID     string    `json:"external_id"`
	Title          string    `json:"title"`
	DescriptionHTML string   `json:"description_html"` // Raw HTML from the ATS
	Location       string    `json:"location"`
	Department     string    `json:"department"`
	Team           string    `json:"team"`
	ApplyURL       string    `json:"apply_url"`
	SourceURL      string    `json:"source_url"`
	EmploymentType string    `json:"employment_type,omitempty"`
	PostedAt       time.Time `json:"posted_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// ContentHash generates a deterministic hash of the job listing content
// for deduplication and change detection.
func (r *RawJobListing) ContentHash() string {
	data := fmt.Sprintf("%s|%s|%s|%s",
		strings.TrimSpace(r.Title),
		strings.TrimSpace(r.DescriptionHTML),
		strings.TrimSpace(r.Location),
		strings.TrimSpace(r.Department),
	)
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h[:16]) // 32-char hex string
}

// ToJob converts a raw listing into a Job model, associating it with a company.
func (r *RawJobListing) ToJob(companyID uuid.UUID) models.Job {
	return models.Job{
		ID:              uuid.New(),
		CompanyID:       companyID,
		ExternalID:      r.ExternalID,
		Title:           r.Title,
		DescriptionRaw:  r.DescriptionHTML,
		DescriptionClean: stripHTML(r.DescriptionHTML),
		Location:        r.Location,
		Department:      r.Department,
		Team:            r.Team,
		ApplyURL:        r.ApplyURL,
		SourceURL:       r.SourceURL,
		EmploymentType:  r.EmploymentType,
		ContentHash:     r.ContentHash(),
		IsActive:        true,
		FirstSeenAt:     time.Now(),
		LastSeenAt:      time.Now(),
	}
}

// stripHTML removes HTML tags to produce clean plaintext.
// This is a simple implementation — handles common cases.
func stripHTML(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			result.WriteRune(' ')
		case !inTag:
			result.WriteRune(r)
		}
	}

	// Collapse whitespace
	text := result.String()
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

// newHTTPClient creates a configured HTTP client for crawling.
func newHTTPClient(userAgent string) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &userAgentTransport{
			base:      http.DefaultTransport,
			userAgent: userAgent,
		},
	}
}

// userAgentTransport wraps http.RoundTripper to inject a custom User-Agent.
type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", t.userAgent)
	return t.base.RoundTrip(req)
}
