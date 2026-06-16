package crawler

import (
	"context"
	"log/slog"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
)

// SeedCompany represents a company to add during initial setup.
type SeedCompany struct {
	Name        string
	Slug        string
	Website     string
	ATSPlatform string
	CareersURL  string
	Industry    string
}

// DefaultCompanies returns a curated list of well-known companies to crawl.
// These all use publicly accessible ATS APIs.
var DefaultCompanies = []SeedCompany{
	// ── Greenhouse companies ──
	{
		Name:        "Stripe",
		Slug:        "stripe",
		Website:     "https://stripe.com",
		ATSPlatform: models.ATSGreenhouse,
		CareersURL:  "https://stripe.com/jobs",
		Industry:    "Fintech",
	},
	{
		Name:        "Airbnb",
		Slug:        "airbnb",
		Website:     "https://airbnb.com",
		ATSPlatform: models.ATSGreenhouse,
		CareersURL:  "https://careers.airbnb.com",
		Industry:    "Travel",
	},
	{
		Name:        "Figma",
		Slug:        "figma",
		Website:     "https://figma.com",
		ATSPlatform: models.ATSGreenhouse,
		CareersURL:  "https://figma.com/careers",
		Industry:    "Design Tools",
	},
	{
		Name:        "Cloudflare",
		Slug:        "cloudflare",
		Website:     "https://cloudflare.com",
		ATSPlatform: models.ATSGreenhouse,
		CareersURL:  "https://cloudflare.com/careers",
		Industry:    "Cloud Infrastructure",
	},
	{
		Name:        "Coinbase",
		Slug:        "coinbase",
		Website:     "https://coinbase.com",
		ATSPlatform: models.ATSGreenhouse,
		CareersURL:  "https://coinbase.com/careers",
		Industry:    "Crypto",
	},
	{
		Name:        "Notion",
		Slug:        "notion",
		Website:     "https://notion.so",
		ATSPlatform: models.ATSGreenhouse,
		CareersURL:  "https://notion.so/careers",
		Industry:    "Productivity",
	},

	// ── Lever companies ──
	{
		Name:        "Netflix",
		Slug:        "netflix",
		Website:     "https://netflix.com",
		ATSPlatform: models.ATSLever,
		CareersURL:  "https://jobs.netflix.com",
		Industry:    "Streaming",
	},

	// ── Ashby companies ──
	{
		Name:        "Ramp",
		Slug:        "ramp",
		Website:     "https://ramp.com",
		ATSPlatform: models.ATSAshby,
		CareersURL:  "https://ramp.com/careers",
		Industry:    "Fintech",
	},
	{
		Name:        "Vercel",
		Slug:        "vercel",
		Website:     "https://vercel.com",
		ATSPlatform: models.ATSAshby,
		CareersURL:  "https://vercel.com/careers",
		Industry:    "Cloud Infrastructure",
	},
}

// SeedDefaultCompanies inserts the default companies if they don't already exist.
func SeedDefaultCompanies(ctx context.Context, pg *store.PostgresStore, logger *slog.Logger) error {
	for _, sc := range DefaultCompanies {
		existing, err := pg.GetCompanyBySlug(ctx, sc.Slug)
		if err != nil {
			return err
		}
		if existing != nil {
			continue // already seeded
		}

		company := &models.Company{
			Name:        sc.Name,
			Slug:        sc.Slug,
			Website:     sc.Website,
			ATSPlatform: sc.ATSPlatform,
			CareersURL:  sc.CareersURL,
			Industry:    sc.Industry,
		}

		if err := pg.CreateCompany(ctx, company); err != nil {
			logger.Error("failed to seed company", "name", sc.Name, "error", err)
			continue
		}

		logger.Info("seeded company", "name", sc.Name, "ats", sc.ATSPlatform)
	}

	return nil
}
