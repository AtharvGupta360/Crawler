package store

import (
	"context"
	"encoding/json"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// ListMatchCandidates returns recent active jobs with enough detail for in-app scoring.
func (s *PostgresStore) ListMatchCandidates(ctx context.Context, limit int) ([]models.Job, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}

	rows, err := s.pool.Query(ctx, `
		SELECT j.id, j.company_id, j.title, j.normalized_title,
		       j.description_raw, j.description_clean, j.location, j.location_type,
		       j.employment_type, j.seniority_level, j.salary_min, j.salary_max,
		       j.salary_currency, j.department, j.team, j.apply_url, j.source_url,
		       j.skills_required, j.skills_preferred, j.experience_years_min, j.experience_years_max,
		       j.education_level, j.ai_summary, j.first_seen_at, j.last_seen_at,
		       j.is_active, j.content_hash,
		       c.id, c.name, c.slug, c.website, c.logo_url, c.ats_platform
		FROM jobs j
		JOIN companies c ON c.id = j.company_id
		WHERE j.is_active = TRUE
		ORDER BY j.first_seen_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		job := models.Job{Company: &models.Company{}}
		var skillsRequiredJSON, skillsPreferredJSON []byte
		if err := rows.Scan(
			&job.ID, &job.CompanyID, &job.Title, &job.NormalizedTitle,
			&job.DescriptionRaw, &job.DescriptionClean, &job.Location, &job.LocationType,
			&job.EmploymentType, &job.SeniorityLevel, &job.SalaryMin, &job.SalaryMax,
			&job.SalaryCurrency, &job.Department, &job.Team, &job.ApplyURL, &job.SourceURL,
			&skillsRequiredJSON, &skillsPreferredJSON, &job.ExperienceYearsMin, &job.ExperienceYearsMax,
			&job.EducationLevel, &job.AISummary, &job.FirstSeenAt, &job.LastSeenAt,
			&job.IsActive, &job.ContentHash,
			&job.Company.ID, &job.Company.Name, &job.Company.Slug,
			&job.Company.Website, &job.Company.LogoURL, &job.Company.ATSPlatform,
		); err != nil {
			return nil, err
		}

		json.Unmarshal(skillsRequiredJSON, &job.SkillsRequired)  //nolint:errcheck
		json.Unmarshal(skillsPreferredJSON, &job.SkillsPreferred) //nolint:errcheck
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}
