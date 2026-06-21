package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/google/uuid"
)

// TrendRefreshResult describes one materialization run.
type TrendRefreshResult struct {
	SnapshotDate time.Time `json:"snapshot_date"`
	Skills       int       `json:"skills"`
}

// SkillTrend is a materialized skill-demand snapshot row.
type SkillTrend struct {
	SnapshotDate  time.Time             `json:"snapshot_date"`
	SkillName     string                `json:"skill_name"`
	JobCount      int                   `json:"job_count"`
	AvgSalaryMin  *int                  `json:"avg_salary_min,omitempty"`
	AvgSalaryMax  *int                  `json:"avg_salary_max,omitempty"`
	TopCompanies  []models.CompanyCount `json:"top_companies,omitempty"`
	SeniorityDist map[string]int        `json:"seniority_dist,omitempty"`
}

// CompanyTrend describes current hiring activity by company.
type CompanyTrend struct {
	CompanyID    uuid.UUID    `json:"company_id"`
	CompanyName  string       `json:"company_name"`
	CompanySlug  string       `json:"company_slug"`
	ActiveJobs   int          `json:"active_jobs"`
	NewJobs      int          `json:"new_jobs"`
	AvgSalaryMin *int         `json:"avg_salary_min,omitempty"`
	AvgSalaryMax *int         `json:"avg_salary_max,omitempty"`
	TopSkills    []SkillCount `json:"top_skills,omitempty"`
}

// SalaryTrend describes salary aggregates by skill and seniority.
type SalaryTrend struct {
	SkillName      string `json:"skill_name"`
	SeniorityLevel string `json:"seniority_level"`
	JobCount       int    `json:"job_count"`
	AvgSalaryMin   *int   `json:"avg_salary_min,omitempty"`
	AvgSalaryMax   *int   `json:"avg_salary_max,omitempty"`
	SalaryCurrency string `json:"salary_currency"`
}

// SkillCount is a simple skill/count pair for analytics responses.
type SkillCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// RefreshTrendSnapshots materializes active job demand into trend_snapshots.
func (s *PostgresStore) RefreshTrendSnapshots(ctx context.Context, snapshotDate time.Time, limit int) (*TrendRefreshResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	dateOnly := time.Date(snapshotDate.Year(), snapshotDate.Month(), snapshotDate.Day(), 0, 0, 0, 0, time.UTC)

	rows, err := s.pool.Query(ctx, `
		WITH job_skill AS (
			SELECT DISTINCT
				j.id AS job_id,
				j.company_id,
				j.seniority_level,
				j.salary_min,
				j.salary_max,
				NULLIF(TRIM(skill->>'name'), '') AS skill_name
			FROM jobs j
			CROSS JOIN LATERAL jsonb_array_elements(
				COALESCE(j.skills_required, '[]'::jsonb) ||
				COALESCE(j.skills_preferred, '[]'::jsonb)
			) AS skill
			WHERE j.is_active = TRUE
		)
		SELECT
			skill_name,
			COUNT(DISTINCT job_id)::int AS job_count,
			AVG(salary_min)::int AS avg_salary_min,
			AVG(salary_max)::int AS avg_salary_max
		FROM job_skill
		WHERE skill_name IS NOT NULL
		GROUP BY skill_name
		ORDER BY job_count DESC, skill_name
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type aggregate struct {
		skillName    string
		jobCount     int
		avgSalaryMin *int
		avgSalaryMax *int
	}

	var aggregates []aggregate
	for rows.Next() {
		var a aggregate
		if err := rows.Scan(&a.skillName, &a.jobCount, &a.avgSalaryMin, &a.avgSalaryMax); err != nil {
			return nil, err
		}
		aggregates = append(aggregates, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM trend_snapshots WHERE snapshot_date = $1`, dateOnly); err != nil {
		return nil, err
	}

	for _, a := range aggregates {
		topCompanies, err := s.topCompaniesForSkill(ctx, a.skillName, 5)
		if err != nil {
			return nil, err
		}
		seniorityDist, err := s.seniorityDistForSkill(ctx, a.skillName)
		if err != nil {
			return nil, err
		}

		topCompaniesJSON, err := json.Marshal(topCompanies)
		if err != nil {
			return nil, err
		}
		seniorityJSON, err := json.Marshal(seniorityDist)
		if err != nil {
			return nil, err
		}

		_, err = s.pool.Exec(ctx, `
			INSERT INTO trend_snapshots (
				snapshot_date, skill_name, job_count,
				avg_salary_min, avg_salary_max, top_companies, seniority_dist
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (snapshot_date, skill_name) DO UPDATE SET
				job_count = EXCLUDED.job_count,
				avg_salary_min = EXCLUDED.avg_salary_min,
				avg_salary_max = EXCLUDED.avg_salary_max,
				top_companies = EXCLUDED.top_companies,
				seniority_dist = EXCLUDED.seniority_dist
		`, dateOnly, a.skillName, a.jobCount, a.avgSalaryMin, a.avgSalaryMax, topCompaniesJSON, seniorityJSON)
		if err != nil {
			return nil, err
		}
	}

	return &TrendRefreshResult{SnapshotDate: dateOnly, Skills: len(aggregates)}, nil
}

// ListSkillTrends returns materialized skill demand snapshots.
func (s *PostgresStore) ListSkillTrends(ctx context.Context, skill string, days, limit int) ([]SkillTrend, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT snapshot_date, skill_name, job_count, avg_salary_min, avg_salary_max,
		       top_companies, seniority_dist
		FROM trend_snapshots
		WHERE snapshot_date >= CURRENT_DATE - ($1::int * INTERVAL '1 day')`
	args := []any{days}
	argIdx := 2

	if skill != "" {
		query += fmt.Sprintf(" AND LOWER(skill_name) = LOWER($%d)", argIdx)
		args = append(args, skill)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY snapshot_date DESC, job_count DESC, skill_name LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []SkillTrend
	for rows.Next() {
		var t SkillTrend
		var topCompaniesJSON, seniorityJSON []byte
		if err := rows.Scan(
			&t.SnapshotDate, &t.SkillName, &t.JobCount, &t.AvgSalaryMin, &t.AvgSalaryMax,
			&topCompaniesJSON, &seniorityJSON,
		); err != nil {
			return nil, err
		}
		if len(topCompaniesJSON) > 0 {
			json.Unmarshal(topCompaniesJSON, &t.TopCompanies) //nolint:errcheck
		}
		if len(seniorityJSON) > 0 {
			json.Unmarshal(seniorityJSON, &t.SeniorityDist) //nolint:errcheck
		}
		trends = append(trends, t)
	}

	return trends, rows.Err()
}

// ListCompanyTrends returns current company hiring activity.
func (s *PostgresStore) ListCompanyTrends(ctx context.Context, days, limit int) ([]CompanyTrend, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			c.id,
			c.name,
			c.slug,
			COUNT(j.id)::int AS active_jobs,
			COUNT(j.id) FILTER (
				WHERE j.first_seen_at >= NOW() - ($1::int * INTERVAL '1 day')
			)::int AS new_jobs,
			AVG(j.salary_min)::int AS avg_salary_min,
			AVG(j.salary_max)::int AS avg_salary_max,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object('name', skill_name, 'count', skill_count))
				FROM (
					SELECT skill_name, COUNT(DISTINCT job_id)::int AS skill_count
					FROM (
						SELECT js.id AS job_id, NULLIF(TRIM(skill->>'name'), '') AS skill_name
						FROM jobs js
						CROSS JOIN LATERAL jsonb_array_elements(
							COALESCE(js.skills_required, '[]'::jsonb) ||
							COALESCE(js.skills_preferred, '[]'::jsonb)
						) AS skill
						WHERE js.company_id = c.id AND js.is_active = TRUE
					) job_skill
					WHERE skill_name IS NOT NULL
					GROUP BY skill_name
					ORDER BY skill_count DESC, skill_name
					LIMIT 10
				) ranked
			), '[]'::jsonb) AS top_skills
		FROM companies c
		JOIN jobs j ON j.company_id = c.id AND j.is_active = TRUE
		GROUP BY c.id, c.name, c.slug
		ORDER BY active_jobs DESC, new_jobs DESC, c.name
		LIMIT $2
	`, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []CompanyTrend
	for rows.Next() {
		var t CompanyTrend
		var topSkillsJSON []byte
		if err := rows.Scan(
			&t.CompanyID, &t.CompanyName, &t.CompanySlug,
			&t.ActiveJobs, &t.NewJobs, &t.AvgSalaryMin, &t.AvgSalaryMax,
			&topSkillsJSON,
		); err != nil {
			return nil, err
		}
		if len(topSkillsJSON) > 0 {
			json.Unmarshal(topSkillsJSON, &t.TopSkills) //nolint:errcheck
		}
		trends = append(trends, t)
	}

	return trends, rows.Err()
}

// ListSalaryTrends returns salary aggregates grouped by skill and seniority.
func (s *PostgresStore) ListSalaryTrends(ctx context.Context, skill, seniority string, limit int) ([]SalaryTrend, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `
		WITH job_skill AS (
			SELECT DISTINCT
				j.id,
				COALESCE(NULLIF(j.seniority_level, ''), 'unknown') AS seniority_level,
				COALESCE(NULLIF(j.salary_currency, ''), 'USD') AS salary_currency,
				j.salary_min,
				j.salary_max,
				NULLIF(TRIM(skill->>'name'), '') AS skill_name
			FROM jobs j
			CROSS JOIN LATERAL jsonb_array_elements(
				COALESCE(j.skills_required, '[]'::jsonb) ||
				COALESCE(j.skills_preferred, '[]'::jsonb)
			) AS skill
			WHERE j.is_active = TRUE
			  AND (j.salary_min IS NOT NULL OR j.salary_max IS NOT NULL)
		)
		SELECT skill_name, seniority_level, COUNT(DISTINCT id)::int AS job_count,
		       AVG(salary_min)::int AS avg_salary_min,
		       AVG(salary_max)::int AS avg_salary_max,
		       salary_currency
		FROM job_skill
		WHERE skill_name IS NOT NULL`
	args := []any{}
	argIdx := 1

	if skill != "" {
		query += fmt.Sprintf(" AND LOWER(skill_name) = LOWER($%d)", argIdx)
		args = append(args, skill)
		argIdx++
	}
	if seniority != "" {
		query += fmt.Sprintf(" AND LOWER(seniority_level) = LOWER($%d)", argIdx)
		args = append(args, seniority)
		argIdx++
	}

	query += fmt.Sprintf(`
		GROUP BY skill_name, seniority_level, salary_currency
		ORDER BY job_count DESC, skill_name, seniority_level
		LIMIT $%d
	`, argIdx)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []SalaryTrend
	for rows.Next() {
		var t SalaryTrend
		if err := rows.Scan(
			&t.SkillName, &t.SeniorityLevel, &t.JobCount,
			&t.AvgSalaryMin, &t.AvgSalaryMax, &t.SalaryCurrency,
		); err != nil {
			return nil, err
		}
		trends = append(trends, t)
	}

	return trends, rows.Err()
}

func (s *PostgresStore) topCompaniesForSkill(ctx context.Context, skillName string, limit int) ([]models.CompanyCount, error) {
	rows, err := s.pool.Query(ctx, `
		WITH matching_jobs AS (
			SELECT DISTINCT j.id, c.name
			FROM jobs j
			JOIN companies c ON c.id = j.company_id
			CROSS JOIN LATERAL jsonb_array_elements(
				COALESCE(j.skills_required, '[]'::jsonb) ||
				COALESCE(j.skills_preferred, '[]'::jsonb)
			) AS skill
			WHERE j.is_active = TRUE
			  AND LOWER(NULLIF(TRIM(skill->>'name'), '')) = LOWER($1)
		)
		SELECT name, COUNT(*)::int AS count
		FROM matching_jobs
		GROUP BY name
		ORDER BY count DESC, name
		LIMIT $2
	`, skillName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []models.CompanyCount
	for rows.Next() {
		var c models.CompanyCount
		if err := rows.Scan(&c.Name, &c.Count); err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, rows.Err()
}

func (s *PostgresStore) seniorityDistForSkill(ctx context.Context, skillName string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		WITH matching_jobs AS (
			SELECT DISTINCT j.id, COALESCE(NULLIF(j.seniority_level, ''), 'unknown') AS seniority_level
			FROM jobs j
			CROSS JOIN LATERAL jsonb_array_elements(
				COALESCE(j.skills_required, '[]'::jsonb) ||
				COALESCE(j.skills_preferred, '[]'::jsonb)
			) AS skill
			WHERE j.is_active = TRUE
			  AND LOWER(NULLIF(TRIM(skill->>'name'), '')) = LOWER($1)
		)
		SELECT seniority_level, COUNT(*)::int AS count
		FROM matching_jobs
		GROUP BY seniority_level
		ORDER BY count DESC, seniority_level
	`, skillName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dist := make(map[string]int)
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		dist[level] = count
	}
	return dist, rows.Err()
}
