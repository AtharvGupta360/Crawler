package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ─────────────────────────────────────────────
// Company operations
// ─────────────────────────────────────────────

// CreateCompany inserts a new company.
func (s *PostgresStore) CreateCompany(ctx context.Context, c *models.Company) error {
	c.ID = uuid.New()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()

	configJSON, _ := json.Marshal(c.CrawlConfig)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO companies (id, name, slug, website, logo_url, industry, size_range, ats_platform, careers_url, crawl_config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, c.ID, c.Name, c.Slug, c.Website, c.LogoURL, c.Industry, c.SizeRange, c.ATSPlatform, c.CareersURL, configJSON, c.CreatedAt, c.UpdatedAt)

	return err
}

// GetCompanyBySlug fetches a company by its slug.
func (s *PostgresStore) GetCompanyBySlug(ctx context.Context, slug string) (*models.Company, error) {
	c := &models.Company{}
	var configJSON []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, website, logo_url, industry, size_range, ats_platform, careers_url, crawl_config, created_at, updated_at
		FROM companies WHERE slug = $1
	`, slug).Scan(&c.ID, &c.Name, &c.Slug, &c.Website, &c.LogoURL, &c.Industry, &c.SizeRange, &c.ATSPlatform, &c.CareersURL, &configJSON, &c.CreatedAt, &c.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if configJSON != nil {
		json.Unmarshal(configJSON, &c.CrawlConfig)
	}

	return c, nil
}

// ListCompanies returns all companies.
func (s *PostgresStore) ListCompanies(ctx context.Context) ([]models.Company, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, slug, website, logo_url, industry, size_range, ats_platform, careers_url, crawl_config, created_at, updated_at
		FROM companies ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []models.Company
	for rows.Next() {
		var c models.Company
		var configJSON []byte
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Website, &c.LogoURL, &c.Industry, &c.SizeRange, &c.ATSPlatform, &c.CareersURL, &configJSON, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if configJSON != nil {
			json.Unmarshal(configJSON, &c.CrawlConfig)
		}
		companies = append(companies, c)
	}

	return companies, rows.Err()
}

// ListCompaniesByATS returns companies using a specific ATS platform.
func (s *PostgresStore) ListCompaniesByATS(ctx context.Context, ats string) ([]models.Company, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, slug, website, logo_url, industry, size_range, ats_platform, careers_url, crawl_config, created_at, updated_at
		FROM companies WHERE ats_platform = $1 ORDER BY name
	`, ats)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []models.Company
	for rows.Next() {
		var c models.Company
		var configJSON []byte
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Website, &c.LogoURL, &c.Industry, &c.SizeRange, &c.ATSPlatform, &c.CareersURL, &configJSON, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if configJSON != nil {
			json.Unmarshal(configJSON, &c.CrawlConfig)
		}
		companies = append(companies, c)
	}

	return companies, rows.Err()
}

// ─────────────────────────────────────────────
// Job operations
// ─────────────────────────────────────────────

// UpsertJob inserts or updates a job based on company_id + external_id.
// Returns true if the job was newly created, false if updated.
func (s *PostgresStore) UpsertJob(ctx context.Context, j *models.Job) (bool, error) {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	now := time.Now()
	j.LastSeenAt = now

	skillsReqJSON, _ := json.Marshal(j.SkillsRequired)
	skillsPrefJSON, _ := json.Marshal(j.SkillsPreferred)

	var isNew bool
	err := s.pool.QueryRow(ctx, `
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
			description_raw = EXCLUDED.description_raw,
			description_clean = EXCLUDED.description_clean,
			location = EXCLUDED.location,
			location_type = EXCLUDED.location_type,
			employment_type = EXCLUDED.employment_type,
			seniority_level = EXCLUDED.seniority_level,
			salary_min = EXCLUDED.salary_min,
			salary_max = EXCLUDED.salary_max,
			department = EXCLUDED.department,
			team = EXCLUDED.team,
			last_seen_at = EXCLUDED.last_seen_at,
			content_hash = EXCLUDED.content_hash,
			is_active = TRUE
		RETURNING (xmax = 0) AS is_new
	`, j.ID, j.CompanyID, j.ExternalID, j.Title, j.NormalizedTitle,
		j.DescriptionRaw, j.DescriptionClean, j.Location, j.LocationType,
		j.EmploymentType, j.SeniorityLevel, j.SalaryMin, j.SalaryMax,
		j.SalaryCurrency, j.Department, j.Team, j.ApplyURL, j.SourceURL,
		skillsReqJSON, skillsPrefJSON, j.ExperienceYearsMin, j.ExperienceYearsMax,
		j.EducationLevel, j.AISummary, now, now,
		true, j.ContentHash,
	).Scan(&isNew)

	return isNew, err
}

// GetJobByID fetches a single job with its company.
func (s *PostgresStore) GetJobByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	j := &models.Job{Company: &models.Company{}}
	var skillsReqJSON, skillsPrefJSON []byte

	err := s.pool.QueryRow(ctx, `
		SELECT j.id, j.company_id, j.external_id, j.title, j.normalized_title,
			j.description_raw, j.description_clean, j.location, j.location_type,
			j.employment_type, j.seniority_level, j.salary_min, j.salary_max,
			j.salary_currency, j.department, j.team, j.apply_url, j.source_url,
			j.skills_required, j.skills_preferred, j.experience_years_min, j.experience_years_max,
			j.education_level, j.ai_summary, j.first_seen_at, j.last_seen_at,
			j.is_active, j.content_hash,
			c.id, c.name, c.slug, c.website, c.logo_url, c.ats_platform
		FROM jobs j
		JOIN companies c ON c.id = j.company_id
		WHERE j.id = $1
	`, id).Scan(
		&j.ID, &j.CompanyID, &j.ExternalID, &j.Title, &j.NormalizedTitle,
		&j.DescriptionRaw, &j.DescriptionClean, &j.Location, &j.LocationType,
		&j.EmploymentType, &j.SeniorityLevel, &j.SalaryMin, &j.SalaryMax,
		&j.SalaryCurrency, &j.Department, &j.Team, &j.ApplyURL, &j.SourceURL,
		&skillsReqJSON, &skillsPrefJSON, &j.ExperienceYearsMin, &j.ExperienceYearsMax,
		&j.EducationLevel, &j.AISummary, &j.FirstSeenAt, &j.LastSeenAt,
		&j.IsActive, &j.ContentHash,
		&j.Company.ID, &j.Company.Name, &j.Company.Slug, &j.Company.Website, &j.Company.LogoURL, &j.Company.ATSPlatform,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(skillsReqJSON, &j.SkillsRequired)
	json.Unmarshal(skillsPrefJSON, &j.SkillsPreferred)

	return j, nil
}

// ListJobs returns paginated jobs with optional filters.
func (s *PostgresStore) ListJobs(ctx context.Context, filter JobFilter) ([]models.Job, int, error) {
	// Count total
	countQuery := "SELECT COUNT(*) FROM jobs WHERE is_active = TRUE"
	args := []any{}
	argIdx := 1

	if filter.SeniorityLevel != "" {
		countQuery += fmt.Sprintf(" AND seniority_level = $%d", argIdx)
		args = append(args, filter.SeniorityLevel)
		argIdx++
	}
	if filter.LocationType != "" {
		countQuery += fmt.Sprintf(" AND location_type = $%d", argIdx)
		args = append(args, filter.LocationType)
		argIdx++
	}
	if filter.CompanyID != uuid.Nil {
		countQuery += fmt.Sprintf(" AND company_id = $%d", argIdx)
		args = append(args, filter.CompanyID)
		argIdx++
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Query jobs
	query := `
		SELECT j.id, j.company_id, j.title, j.normalized_title, j.location,
			j.location_type, j.employment_type, j.seniority_level,
			j.salary_min, j.salary_max, j.department, j.apply_url,
			j.ai_summary, j.first_seen_at, j.last_seen_at, j.is_active,
			c.name, c.slug, c.logo_url
		FROM jobs j
		JOIN companies c ON c.id = j.company_id
		WHERE j.is_active = TRUE`

	queryArgs := []any{}
	queryArgIdx := 1

	if filter.SeniorityLevel != "" {
		query += fmt.Sprintf(" AND j.seniority_level = $%d", queryArgIdx)
		queryArgs = append(queryArgs, filter.SeniorityLevel)
		queryArgIdx++
	}
	if filter.LocationType != "" {
		query += fmt.Sprintf(" AND j.location_type = $%d", queryArgIdx)
		queryArgs = append(queryArgs, filter.LocationType)
		queryArgIdx++
	}
	if filter.CompanyID != uuid.Nil {
		query += fmt.Sprintf(" AND j.company_id = $%d", queryArgIdx)
		queryArgs = append(queryArgs, filter.CompanyID)
		queryArgIdx++
	}

	query += fmt.Sprintf(" ORDER BY j.first_seen_at DESC LIMIT $%d OFFSET $%d", queryArgIdx, queryArgIdx+1)
	queryArgs = append(queryArgs, filter.Limit, filter.Offset)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		j := models.Job{Company: &models.Company{}}
		if err := rows.Scan(
			&j.ID, &j.CompanyID, &j.Title, &j.NormalizedTitle, &j.Location,
			&j.LocationType, &j.EmploymentType, &j.SeniorityLevel,
			&j.SalaryMin, &j.SalaryMax, &j.Department, &j.ApplyURL,
			&j.AISummary, &j.FirstSeenAt, &j.LastSeenAt, &j.IsActive,
			&j.Company.Name, &j.Company.Slug, &j.Company.LogoURL,
		); err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, j)
	}

	return jobs, total, rows.Err()
}

// MarkJobsInactive marks jobs that were not seen in the latest crawl as inactive.
func (s *PostgresStore) MarkJobsInactive(ctx context.Context, companyID uuid.UUID, seenBefore time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET is_active = FALSE
		WHERE company_id = $1 AND last_seen_at < $2 AND is_active = TRUE
	`, companyID, seenBefore)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// JobFilter holds filtering/pagination parameters for listing jobs.
type JobFilter struct {
	SeniorityLevel string
	LocationType   string
	CompanyID      uuid.UUID
	Limit          int
	Offset         int
}

// ─────────────────────────────────────────────
// Crawl Run operations
// ─────────────────────────────────────────────

// CreateCrawlRun inserts a new crawl run record.
func (s *PostgresStore) CreateCrawlRun(ctx context.Context, run *models.CrawlRun) error {
	run.ID = uuid.New()
	run.StartedAt = time.Now()
	run.Status = "running"

	_, err := s.pool.Exec(ctx, `
		INSERT INTO crawl_runs (id, company_id, started_at, status)
		VALUES ($1, $2, $3, $4)
	`, run.ID, run.CompanyID, run.StartedAt, run.Status)

	return err
}

// CompleteCrawlRun updates a crawl run with its results.
func (s *PostgresStore) CompleteCrawlRun(ctx context.Context, run *models.CrawlRun) error {
	now := time.Now()
	run.CompletedAt = &now
	run.DurationMs = now.Sub(run.StartedAt).Milliseconds()

	_, err := s.pool.Exec(ctx, `
		UPDATE crawl_runs SET
			completed_at = $2, status = $3, jobs_found = $4,
			jobs_new = $5, jobs_updated = $6, jobs_removed = $7,
			error_message = $8, duration_ms = $9
		WHERE id = $1
	`, run.ID, run.CompletedAt, run.Status, run.JobsFound,
		run.JobsNew, run.JobsUpdated, run.JobsRemoved,
		run.ErrorMessage, run.DurationMs)

	return err
}

// GetRecentCrawlRuns returns the most recent crawl runs.
func (s *PostgresStore) GetRecentCrawlRuns(ctx context.Context, limit int) ([]models.CrawlRun, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, company_id, started_at, completed_at, status,
			jobs_found, jobs_new, jobs_updated, jobs_removed,
			error_message, duration_ms
		FROM crawl_runs ORDER BY started_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []models.CrawlRun
	for rows.Next() {
		var r models.CrawlRun
		if err := rows.Scan(
			&r.ID, &r.CompanyID, &r.StartedAt, &r.CompletedAt, &r.Status,
			&r.JobsFound, &r.JobsNew, &r.JobsUpdated, &r.JobsRemoved,
			&r.ErrorMessage, &r.DurationMs,
		); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}

	return runs, rows.Err()
}

// GetJobStats returns aggregate statistics about the jobs table.
func (s *PostgresStore) GetJobStats(ctx context.Context) (*JobStats, error) {
	stats := &JobStats{}

	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE is_active = TRUE) as active_jobs,
			COUNT(*) as total_jobs,
			COUNT(DISTINCT company_id) as companies,
			MIN(first_seen_at) as earliest_job,
			MAX(first_seen_at) as latest_job
		FROM jobs
	`).Scan(&stats.ActiveJobs, &stats.TotalJobs, &stats.Companies, &stats.EarliestJob, &stats.LatestJob)

	if err != nil {
		return nil, err
	}

	// Seniority distribution
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(seniority_level, 'unknown'), COUNT(*)
		FROM jobs WHERE is_active = TRUE
		GROUP BY seniority_level ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats.BySeniority = make(map[string]int)
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		stats.BySeniority[level] = count
	}

	return stats, nil
}

// JobStats holds aggregate statistics.
type JobStats struct {
	ActiveJobs  int            `json:"active_jobs"`
	TotalJobs   int            `json:"total_jobs"`
	Companies   int            `json:"companies"`
	EarliestJob *time.Time     `json:"earliest_job"`
	LatestJob   *time.Time     `json:"latest_job"`
	BySeniority map[string]int `json:"by_seniority"`
}

