// Package main implements a demo data seeder for the JobCrawl platform.
// It populates PostgreSQL with ~150 realistic job postings, a skills taxonomy,
// 14 days of trend snapshots, and a demo user account.
//
// Usage:
//
//	go run ./cmd/seed/
//	make seed
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/auth"
	"github.com/AtharvGupta360/JobCrawl/internal/config"
	"github.com/AtharvGupta360/JobCrawl/internal/crawler"
	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	//fmt.Printf("CFG = %+v\n", cfg)
//fmt.Printf("DATABASE_URL = %s\n", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	//fmt.Println("DatabaseURL =", cfg.DatabaseURL)

	ctx := context.Background()

	pg, err := store.NewPostgresStore(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	// Step 1: Seed companies (reuse the existing seeder)
	if err := crawler.SeedDefaultCompanies(ctx, pg, logger); err != nil {
		logger.Error("failed to seed companies", "error", err)
		os.Exit(1)
	}

	// Step 2: Fetch all companies (we need their IDs)
	companies, err := pg.ListCompanies(ctx)
	if err != nil {
		logger.Error("failed to list companies", "error", err)
		os.Exit(1)
	}
	logger.Info("companies loaded", "count", len(companies))

	// Step 3: Seed skills taxonomy
	seedSkills(ctx, pg, logger)

	// Step 4: Seed jobs
	seedJobs(ctx, pg, companies, logger)

	// Step 5: Seed trend snapshots
	seedTrends(ctx, pg, companies, logger)

	// Step 6: Seed demo user
	seedDemoUser(ctx, pg, logger)

	logger.Info("✅ seeding complete")
}

// ─────────────────────────────────────────────
// Skills taxonomy
// ─────────────────────────────────────────────

type skillDef struct {
	Name     string
	Category string
	Aliases  []string
}

var skillsTaxonomy = []skillDef{
	// Languages
	{Name: "Go", Category: "language", Aliases: []string{"Golang", "Go lang"}},
	{Name: "Python", Category: "language", Aliases: []string{"Python3"}},
	{Name: "JavaScript", Category: "language", Aliases: []string{"JS", "ECMAScript"}},
	{Name: "TypeScript", Category: "language", Aliases: []string{"TS"}},
	{Name: "Java", Category: "language", Aliases: []string{}},
	{Name: "Rust", Category: "language", Aliases: []string{}},
	{Name: "C++", Category: "language", Aliases: []string{"CPP"}},
	{Name: "SQL", Category: "language", Aliases: []string{}},

	// Frameworks
	{Name: "React", Category: "framework", Aliases: []string{"React.js", "ReactJS"}},
	{Name: "Node.js", Category: "framework", Aliases: []string{"Node", "NodeJS"}},
	{Name: "Django", Category: "framework", Aliases: []string{}},
	{Name: "Spring Boot", Category: "framework", Aliases: []string{"Spring"}},
	{Name: "Next.js", Category: "framework", Aliases: []string{"NextJS"}},

	// Tools & Infra
	{Name: "Docker", Category: "tool", Aliases: []string{"Containers"}},
	{Name: "Kubernetes", Category: "tool", Aliases: []string{"K8s"}},
	{Name: "AWS", Category: "tool", Aliases: []string{"Amazon Web Services"}},
	{Name: "GCP", Category: "tool", Aliases: []string{"Google Cloud"}},
	{Name: "PostgreSQL", Category: "tool", Aliases: []string{"Postgres", "PG"}},
	{Name: "Redis", Category: "tool", Aliases: []string{}},
	{Name: "Kafka", Category: "tool", Aliases: []string{"Apache Kafka"}},
	{Name: "Elasticsearch", Category: "tool", Aliases: []string{"ES"}},
	{Name: "Terraform", Category: "tool", Aliases: []string{"TF"}},
	{Name: "Git", Category: "tool", Aliases: []string{}},
	{Name: "CI/CD", Category: "tool", Aliases: []string{"Jenkins", "GitHub Actions"}},
	{Name: "GraphQL", Category: "tool", Aliases: []string{}},

	// Concepts
	{Name: "System Design", Category: "concept", Aliases: []string{}},
	{Name: "Distributed Systems", Category: "concept", Aliases: []string{}},
	{Name: "Machine Learning", Category: "concept", Aliases: []string{"ML"}},
	{Name: "Data Structures", Category: "concept", Aliases: []string{"DSA", "Algorithms"}},
	{Name: "REST APIs", Category: "concept", Aliases: []string{"RESTful"}},
	{Name: "Microservices", Category: "concept", Aliases: []string{}},
}

func seedSkills(ctx context.Context, pg *store.PostgresStore, logger *slog.Logger) {
	pool := pg.Pool()
	inserted := 0
	for _, s := range skillsTaxonomy {
		aliasArr := "{}"
		if len(s.Aliases) > 0 {
			aliasArr = "{" + strings.Join(quoteStrings(s.Aliases), ",") + "}"
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO skills (id, name, category, aliases)
			VALUES ($1, $2, $3, $4::text[])
			ON CONFLICT (name) DO NOTHING
		`, uuid.New(), s.Name, s.Category, aliasArr)
		if err != nil {
			logger.Warn("failed to seed skill", "name", s.Name, "error", err)
			continue
		}
		inserted++
	}
	logger.Info("skills seeded", "inserted", inserted, "total", len(skillsTaxonomy))
}

func quoteStrings(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = `"` + s + `"`
	}
	return out
}

// ─────────────────────────────────────────────
// Jobs
// ─────────────────────────────────────────────

type jobTemplate struct {
	Title           string
	NormalizedTitle string
	Department      string
	Team            string
	DescriptionSnippet string
	SkillsRequired  []skillEntry
	SkillsPreferred []skillEntry
}

type skillEntry struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	Importance string `json:"importance"`
}

var jobTemplates = []jobTemplate{
	{
		Title: "Backend Engineer", NormalizedTitle: "Backend Engineer",
		Department: "Engineering", Team: "Platform",
		DescriptionSnippet: "Design and build scalable backend services that power our core platform. You'll work with distributed systems, APIs, and databases to deliver reliable, high-performance infrastructure.",
		SkillsRequired:     []skillEntry{{Name: "Go", Category: "language", Importance: "required"}, {Name: "PostgreSQL", Category: "tool", Importance: "required"}, {Name: "REST APIs", Category: "concept", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Kafka", Category: "tool", Importance: "preferred"}, {Name: "Docker", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Frontend Engineer", NormalizedTitle: "Frontend Engineer",
		Department: "Engineering", Team: "Product",
		DescriptionSnippet: "Build beautiful, responsive user interfaces that delight millions of users. You'll collaborate closely with designers and backend engineers to ship pixel-perfect features.",
		SkillsRequired:     []skillEntry{{Name: "React", Category: "framework", Importance: "required"}, {Name: "TypeScript", Category: "language", Importance: "required"}, {Name: "JavaScript", Category: "language", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Next.js", Category: "framework", Importance: "preferred"}, {Name: "GraphQL", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Full Stack Engineer", NormalizedTitle: "Full Stack Engineer",
		Department: "Engineering", Team: "Product",
		DescriptionSnippet: "Own features end-to-end, from database schema to polished UI. You'll make architectural decisions that balance velocity with long-term maintainability.",
		SkillsRequired:     []skillEntry{{Name: "Python", Category: "language", Importance: "required"}, {Name: "React", Category: "framework", Importance: "required"}, {Name: "PostgreSQL", Category: "tool", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Docker", Category: "tool", Importance: "preferred"}, {Name: "AWS", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Site Reliability Engineer", NormalizedTitle: "SRE",
		Department: "Infrastructure", Team: "SRE",
		DescriptionSnippet: "Ensure our systems are reliable, scalable, and performant. You'll design monitoring solutions, automate incident response, and drive reliability improvements across the stack.",
		SkillsRequired:     []skillEntry{{Name: "Kubernetes", Category: "tool", Importance: "required"}, {Name: "AWS", Category: "tool", Importance: "required"}, {Name: "Terraform", Category: "tool", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Go", Category: "language", Importance: "preferred"}, {Name: "Python", Category: "language", Importance: "preferred"}},
	},
	{
		Title: "Data Engineer", NormalizedTitle: "Data Engineer",
		Department: "Data", Team: "Data Platform",
		DescriptionSnippet: "Build and maintain the data infrastructure that enables analytics, ML, and data-driven decision making. Design robust ETL pipelines and data models.",
		SkillsRequired:     []skillEntry{{Name: "Python", Category: "language", Importance: "required"}, {Name: "SQL", Category: "language", Importance: "required"}, {Name: "AWS", Category: "tool", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Kafka", Category: "tool", Importance: "preferred"}, {Name: "Elasticsearch", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Machine Learning Engineer", NormalizedTitle: "ML Engineer",
		Department: "Data", Team: "ML Platform",
		DescriptionSnippet: "Develop and deploy machine learning models at scale. You'll work on recommendation systems, NLP, and computer vision to create intelligent product features.",
		SkillsRequired:     []skillEntry{{Name: "Python", Category: "language", Importance: "required"}, {Name: "Machine Learning", Category: "concept", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Docker", Category: "tool", Importance: "preferred"}, {Name: "Kubernetes", Category: "tool", Importance: "preferred"}, {Name: "GCP", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "DevOps Engineer", NormalizedTitle: "DevOps Engineer",
		Department: "Infrastructure", Team: "DevOps",
		DescriptionSnippet: "Automate infrastructure, CI/CD pipelines, and deployment workflows. You'll empower engineering teams to ship code faster and more reliably.",
		SkillsRequired:     []skillEntry{{Name: "Docker", Category: "tool", Importance: "required"}, {Name: "CI/CD", Category: "tool", Importance: "required"}, {Name: "Terraform", Category: "tool", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Kubernetes", Category: "tool", Importance: "preferred"}, {Name: "AWS", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Software Engineer", NormalizedTitle: "Software Engineer",
		Department: "Engineering", Team: "Core",
		DescriptionSnippet: "Solve complex engineering challenges and build systems that serve millions of users. You'll participate in architecture discussions and contribute across the stack.",
		SkillsRequired:     []skillEntry{{Name: "Java", Category: "language", Importance: "required"}, {Name: "Spring Boot", Category: "framework", Importance: "required"}, {Name: "System Design", Category: "concept", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Kafka", Category: "tool", Importance: "preferred"}, {Name: "Redis", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Platform Engineer", NormalizedTitle: "Platform Engineer",
		Department: "Engineering", Team: "Platform",
		DescriptionSnippet: "Build internal developer tools and platforms that accelerate product development. Design APIs, SDKs, and infrastructure abstractions used by the entire engineering org.",
		SkillsRequired:     []skillEntry{{Name: "Go", Category: "language", Importance: "required"}, {Name: "Kubernetes", Category: "tool", Importance: "required"}, {Name: "Distributed Systems", Category: "concept", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Rust", Category: "language", Importance: "preferred"}, {Name: "Terraform", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Security Engineer", NormalizedTitle: "Security Engineer",
		Department: "Security", Team: "AppSec",
		DescriptionSnippet: "Protect our users and infrastructure from security threats. You'll conduct security reviews, build detection systems, and drive security best practices across engineering.",
		SkillsRequired:     []skillEntry{{Name: "Python", Category: "language", Importance: "required"}, {Name: "AWS", Category: "tool", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Go", Category: "language", Importance: "preferred"}, {Name: "Kubernetes", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "iOS Engineer", NormalizedTitle: "iOS Engineer",
		Department: "Mobile", Team: "iOS",
		DescriptionSnippet: "Build and ship features for our iOS app used by millions. You'll collaborate with design and product to create seamless mobile experiences with Swift and UIKit/SwiftUI.",
		SkillsRequired:     []skillEntry{{Name: "JavaScript", Category: "language", Importance: "required"}, {Name: "REST APIs", Category: "concept", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "GraphQL", Category: "tool", Importance: "preferred"}, {Name: "CI/CD", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Backend Engineer — Payments", NormalizedTitle: "Backend Engineer",
		Department: "Engineering", Team: "Payments",
		DescriptionSnippet: "Build and maintain the payment processing infrastructure that handles billions in transactions. You'll work on ledger systems, compliance, and financial integrations.",
		SkillsRequired:     []skillEntry{{Name: "Go", Category: "language", Importance: "required"}, {Name: "PostgreSQL", Category: "tool", Importance: "required"}, {Name: "Distributed Systems", Category: "concept", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Redis", Category: "tool", Importance: "preferred"}, {Name: "Kafka", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Infrastructure Engineer", NormalizedTitle: "Infrastructure Engineer",
		Department: "Infrastructure", Team: "Cloud",
		DescriptionSnippet: "Design and operate cloud infrastructure at scale. You'll build self-healing systems, optimize costs, and ensure five-nines uptime for critical services.",
		SkillsRequired:     []skillEntry{{Name: "AWS", Category: "tool", Importance: "required"}, {Name: "Terraform", Category: "tool", Importance: "required"}, {Name: "Docker", Category: "tool", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Go", Category: "language", Importance: "preferred"}, {Name: "Python", Category: "language", Importance: "preferred"}},
	},
	{
		Title: "Staff Software Engineer", NormalizedTitle: "Staff Software Engineer",
		Department: "Engineering", Team: "Architecture",
		DescriptionSnippet: "Drive technical strategy and mentor engineering teams. You'll make cross-team architectural decisions, lead design reviews, and establish engineering best practices.",
		SkillsRequired:     []skillEntry{{Name: "System Design", Category: "concept", Importance: "required"}, {Name: "Distributed Systems", Category: "concept", Importance: "required"}, {Name: "Microservices", Category: "concept", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "Go", Category: "language", Importance: "preferred"}, {Name: "Kafka", Category: "tool", Importance: "preferred"}, {Name: "Kubernetes", Category: "tool", Importance: "preferred"}},
	},
	{
		Title: "Product Engineer", NormalizedTitle: "Product Engineer",
		Department: "Product", Team: "Growth",
		DescriptionSnippet: "Combine product thinking with engineering execution. You'll own features from ideation to deployment, run experiments, and use data to drive user growth.",
		SkillsRequired:     []skillEntry{{Name: "TypeScript", Category: "language", Importance: "required"}, {Name: "React", Category: "framework", Importance: "required"}, {Name: "Node.js", Category: "framework", Importance: "required"}},
		SkillsPreferred:    []skillEntry{{Name: "PostgreSQL", Category: "tool", Importance: "preferred"}, {Name: "Redis", Category: "tool", Importance: "preferred"}},
	},
}

// Seniority → salary ranges (USD)
var salaryRanges = map[string][2]int{
	"intern":  {60000, 85000},
	"junior":  {85000, 135000},
	"mid":     {120000, 185000},
	"senior":  {155000, 260000},
	"lead":    {180000, 300000},
	"staff":   {220000, 380000},
}

var locations = []struct {
	Name string
	Type string
}{
	{"Remote", "remote"},
	{"Remote — US", "remote"},
	{"Remote — Global", "remote"},
	{"San Francisco, CA", "onsite"},
	{"New York, NY", "onsite"},
	{"Seattle, WA", "onsite"},
	{"Austin, TX", "onsite"},
	{"Boston, MA", "onsite"},
	{"San Francisco, CA (Hybrid)", "hybrid"},
	{"New York, NY (Hybrid)", "hybrid"},
	{"Seattle, WA (Hybrid)", "hybrid"},
	{"London, UK (Hybrid)", "hybrid"},
}

// Weighted seniority distribution: ~15% intern, ~25% junior, ~30% mid, ~20% senior, ~5% lead, ~5% staff
var seniorityWeights = []struct {
	Level  string
	Weight int
}{
	{"intern", 15},
	{"junior", 25},
	{"mid", 30},
	{"senior", 20},
	{"lead", 5},
	{"staff", 5},
}

func pickSeniority() string {
	total := 0
	for _, sw := range seniorityWeights {
		total += sw.Weight
	}
	n := rand.IntN(total)
	for _, sw := range seniorityWeights {
		n -= sw.Weight
		if n < 0 {
			return sw.Level
		}
	}
	return "mid"
}

var employmentTypes = []string{"full_time", "full_time", "full_time", "full_time", "intern", "contract"}

var aiSummaries = []string{
	"This role focuses on building and maintaining core %s systems. The ideal candidate has strong %s experience and thrives in a fast-paced environment. %s offers competitive compensation and a collaborative engineering culture.",
	"Join %s's engineering team to work on %s challenges at scale. You'll collaborate with cross-functional teams to deliver impactful features. Strong problem-solving skills and experience with %s are essential.",
	"An exciting opportunity to shape the future of %s's %s infrastructure. The team values clean code, thorough testing, and continuous improvement. Experience with %s is a strong plus.",
	"%s is looking for a talented engineer to strengthen their %s capabilities. You'll have ownership over key technical decisions and mentor junior team members. Familiarity with %s technologies is highly valued.",
}

func seedJobs(ctx context.Context, pg *store.PostgresStore, companies []models.Company, logger *slog.Logger) {
	pool := pg.Pool()
	inserted := 0
	now := time.Now()

	for i := 0; i < 150; i++ {
		// Pick random company and template
		company := companies[rand.IntN(len(companies))]
		tmpl := jobTemplates[rand.IntN(len(jobTemplates))]
		seniority := pickSeniority()
		loc := locations[rand.IntN(len(locations))]
		empType := employmentTypes[rand.IntN(len(employmentTypes))]

		// For interns, force employment_type = intern
		if seniority == "intern" {
			empType = "intern"
		}

		// Generate title with seniority prefix
		title := tmpl.Title
		switch seniority {
		case "intern":
			title += " Intern"
		case "junior":
			title += " (L3)"
		case "mid":
			title += " (L4)"
		case "senior":
			title = "Senior " + title
		case "lead":
			title = "Lead " + title
		case "staff":
			title = "Staff " + title
		}

		// Salary
		salaryRange := salaryRanges[seniority]
		salaryMin := salaryRange[0] + rand.IntN(20000)
		salaryMax := salaryMin + 30000 + rand.IntN(40000)
		if salaryMax > salaryRange[1] {
			salaryMax = salaryRange[1]
		}

		// Spread first_seen_at over last 30 days
		daysAgo := rand.IntN(30)
		firstSeen := now.AddDate(0, 0, -daysAgo)

		externalID := fmt.Sprintf("demo-%s-%d", company.Slug, i)

		// Build description
		desc := fmt.Sprintf("<h2>%s at %s</h2><p>%s</p><h3>Requirements</h3><ul>", title, company.Name, tmpl.DescriptionSnippet)
		for _, s := range tmpl.SkillsRequired {
			desc += fmt.Sprintf("<li>%s (%s)</li>", s.Name, s.Importance)
		}
		desc += "</ul>"
		descClean := fmt.Sprintf("%s at %s. %s", title, company.Name, tmpl.DescriptionSnippet)

		// AI summary
		summaryTemplate := aiSummaries[rand.IntN(len(aiSummaries))]
		primarySkill := "engineering"
		if len(tmpl.SkillsRequired) > 0 {
			primarySkill = tmpl.SkillsRequired[0].Name
		}
		aiSummary := fmt.Sprintf(summaryTemplate, company.Name, primarySkill, company.Name)

		// Content hash for dedup
		hashInput := fmt.Sprintf("%s|%s|%s|%s", title, desc, loc.Name, tmpl.Department)
		contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(hashInput)))

		skillsReqJSON, _ := json.Marshal(tmpl.SkillsRequired)
		skillsPrefJSON, _ := json.Marshal(tmpl.SkillsPreferred)

		applyURL := fmt.Sprintf("https://%s.com/careers/%s", company.Slug, externalID)
		sourceURL := fmt.Sprintf("https://boards.greenhouse.io/%s/jobs/%s", company.Slug, externalID)

		_, err := pool.Exec(ctx, `
			INSERT INTO jobs (
				id, company_id, external_id, title, normalized_title,
				description_raw, description_clean, location, location_type,
				employment_type, seniority_level, salary_min, salary_max,
				salary_currency, department, team, apply_url, source_url,
				skills_required, skills_preferred, experience_years_min, experience_years_max,
				education_level, ai_summary, first_seen_at, last_seen_at,
				is_active, content_hash
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
				$14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24,
				$25, $26, $27, $28
			)
			ON CONFLICT (company_id, external_id) DO UPDATE SET
				title = EXCLUDED.title,
				last_seen_at = EXCLUDED.last_seen_at,
				content_hash = EXCLUDED.content_hash,
				is_active = TRUE
		`,
			uuid.New(), company.ID, externalID, title, tmpl.NormalizedTitle,
			desc, descClean, loc.Name, loc.Type,
			empType, seniority, salaryMin, salaryMax,
			"USD", tmpl.Department, tmpl.Team, applyURL, sourceURL,
			skillsReqJSON, skillsPrefJSON, experienceMin(seniority), experienceMax(seniority),
			educationLevel(seniority), aiSummary, firstSeen, now,
			true, contentHash,
		)
		if err != nil {
			logger.Warn("failed to seed job", "title", title, "company", company.Name, "error", err)
			continue
		}
		inserted++
	}

	logger.Info("jobs seeded", "inserted", inserted)
}

func experienceMin(seniority string) *int {
	m := map[string]int{"intern": 0, "junior": 0, "mid": 2, "senior": 5, "lead": 7, "staff": 10}
	v := m[seniority]
	return &v
}

func experienceMax(seniority string) *int {
	m := map[string]int{"intern": 1, "junior": 2, "mid": 5, "senior": 10, "lead": 12, "staff": 15}
	v := m[seniority]
	return &v
}

func educationLevel(seniority string) string {
	if seniority == "intern" {
		return "pursuing_bachelors"
	}
	return "bachelors"
}

// ─────────────────────────────────────────────
// Trend Snapshots (14 days)
// ─────────────────────────────────────────────

var trendSkills = []string{
	"Go", "Python", "JavaScript", "TypeScript", "React", "Kubernetes",
	"AWS", "Docker", "PostgreSQL", "Kafka", "Java", "Rust",
	"Machine Learning", "System Design", "Node.js", "Terraform",
}

func seedTrends(ctx context.Context, pg *store.PostgresStore, companies []models.Company, logger *slog.Logger) {
	pool := pg.Pool()
	inserted := 0
	now := time.Now()

	companyNames := make([]string, len(companies))
	for i, c := range companies {
		companyNames[i] = c.Name
	}

	for day := 0; day < 14; day++ {
		snapshotDate := now.AddDate(0, 0, -day)

		for _, skill := range trendSkills {
			// Simulate trending with some variation
			baseCount := 10 + rand.IntN(30)
			// More recent days have slightly higher counts (growth trend)
			growth := (14 - day) / 3
			jobCount := baseCount + growth

			avgSalaryMin := 90000 + rand.IntN(60000)
			avgSalaryMax := avgSalaryMin + 40000 + rand.IntN(30000)

			// Top companies for this skill
			numCompanies := 3 + rand.IntN(3)
			if numCompanies > len(companyNames) {
				numCompanies = len(companyNames)
			}
			topCompanies := make([]map[string]any, numCompanies)
			for j := 0; j < numCompanies; j++ {
				topCompanies[j] = map[string]any{
					"name":  companyNames[(j+day)%len(companyNames)],
					"count": 2 + rand.IntN(8),
				}
			}

			seniorityDist := map[string]int{
				"intern": 1 + rand.IntN(4),
				"junior": 3 + rand.IntN(8),
				"mid":    5 + rand.IntN(10),
				"senior": 3 + rand.IntN(7),
				"staff":  rand.IntN(3),
			}

			topCompaniesJSON, _ := json.Marshal(topCompanies)
			seniorityDistJSON, _ := json.Marshal(seniorityDist)

			_, err := pool.Exec(ctx, `
				INSERT INTO trend_snapshots (id, snapshot_date, skill_name, job_count, avg_salary_min, avg_salary_max, top_companies, seniority_dist)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (snapshot_date, skill_name) DO UPDATE SET
					job_count = EXCLUDED.job_count,
					avg_salary_min = EXCLUDED.avg_salary_min,
					avg_salary_max = EXCLUDED.avg_salary_max,
					top_companies = EXCLUDED.top_companies,
					seniority_dist = EXCLUDED.seniority_dist
			`, uuid.New(), snapshotDate, skill, jobCount, avgSalaryMin, avgSalaryMax, topCompaniesJSON, seniorityDistJSON)
			if err != nil {
				logger.Warn("failed to seed trend", "skill", skill, "day", day, "error", err)
				continue
			}
			inserted++
		}
	}

	logger.Info("trend snapshots seeded", "inserted", inserted, "days", 14, "skills", len(trendSkills))
}

// ─────────────────────────────────────────────
// Demo User
// ─────────────────────────────────────────────

func seedDemoUser(ctx context.Context, pg *store.PostgresStore, logger *slog.Logger) {
	pool := pg.Pool()

	// Check if demo user already exists
	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email = $1", "demo@jobcrawl.dev").Scan(&count)
	if err != nil {
		logger.Warn("failed to check demo user", "error", err)
		return
	}
	if count > 0 {
		logger.Info("demo user already exists", "email", "demo@jobcrawl.dev")
		return
	}

	hash, err := auth.HashPassword("demo1234")
	if err != nil {
		logger.Error("failed to hash demo password", "error", err)
		return
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, role, target_roles, target_seniority, target_locations, known_skills, learning_skills, resume_text, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		uuid.New(),
		"demo@jobcrawl.dev",
		hash,
		"Demo User",
		"user",
		[]string{"Backend Engineer", "Software Engineer", "Platform Engineer"},
		[]string{"junior", "mid"},
		[]string{"remote", "San Francisco, CA"},
		[]string{"Go", "Python", "PostgreSQL", "Docker", "REST APIs"},
		[]string{"Kubernetes", "Kafka", "System Design"},
		"Computer Science graduate with 2 years of experience in Go and Python. Built distributed systems at scale, familiar with cloud infrastructure (AWS), containerization (Docker), and relational databases (PostgreSQL). Strong foundations in data structures and algorithms. Looking for backend-focused roles at product-driven companies.",
		time.Now(), time.Now(),
	)
	if err != nil {
		logger.Warn("failed to seed demo user", "error", err)
		return
	}

	logger.Info("demo user seeded", "email", "demo@jobcrawl.dev", "password", "demo1234")
}
