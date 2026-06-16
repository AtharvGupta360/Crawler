package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered user with their career preferences.
type User struct {
	ID               uuid.UUID `json:"id" db:"id"`
	Email            string    `json:"email" db:"email"`
	Name             string    `json:"name,omitempty" db:"name"`
	TargetRoles      []string  `json:"target_roles,omitempty" db:"target_roles"`
	TargetSeniority  []string  `json:"target_seniority,omitempty" db:"target_seniority"`
	TargetLocations  []string  `json:"target_locations,omitempty" db:"target_locations"`
	KnownSkills      []string  `json:"known_skills,omitempty" db:"known_skills"`
	LearningSkills   []string  `json:"learning_skills,omitempty" db:"learning_skills"`
	ResumeText       string    `json:"resume_text,omitempty" db:"resume_text"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// Alert represents a user-defined job alert rule.
type Alert struct {
	ID            uuid.UUID         `json:"id" db:"id"`
	UserID        uuid.UUID         `json:"user_id" db:"user_id"`
	Name          string            `json:"name" db:"name"`
	Filters       map[string]any    `json:"filters" db:"filters"` // {"skills": ["Go"], "seniority": ["junior"]}
	IsActive      bool              `json:"is_active" db:"is_active"`
	NotifyVia     string            `json:"notify_via" db:"notify_via"` // "websocket", "email"
	LastTriggered *time.Time        `json:"last_triggered,omitempty" db:"last_triggered"`
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
}

// Skill represents an entry in the skills taxonomy.
type Skill struct {
	ID       uuid.UUID `json:"id" db:"id"`
	Name     string    `json:"name" db:"name"`
	Category string    `json:"category" db:"category"` // "language", "framework", "tool", "concept"
	Aliases  []string  `json:"aliases,omitempty" db:"aliases"`
	ParentID *uuid.UUID `json:"parent_id,omitempty" db:"parent_id"`
}

// CrawlRun tracks a single crawl execution against a company.
type CrawlRun struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	CompanyID    uuid.UUID  `json:"company_id" db:"company_id"`
	StartedAt    time.Time  `json:"started_at" db:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	Status       string     `json:"status" db:"status"` // "running", "completed", "failed"
	JobsFound    int        `json:"jobs_found" db:"jobs_found"`
	JobsNew      int        `json:"jobs_new" db:"jobs_new"`
	JobsUpdated  int        `json:"jobs_updated" db:"jobs_updated"`
	JobsRemoved  int        `json:"jobs_removed" db:"jobs_removed"`
	ErrorMessage string     `json:"error_message,omitempty" db:"error_message"`
	DurationMs   int64      `json:"duration_ms,omitempty" db:"duration_ms"`
}

// TrendSnapshot represents a daily aggregate for a skill's demand.
type TrendSnapshot struct {
	ID             uuid.UUID      `json:"id" db:"id"`
	SnapshotDate   time.Time      `json:"snapshot_date" db:"snapshot_date"`
	SkillName      string         `json:"skill_name" db:"skill_name"`
	JobCount       int            `json:"job_count" db:"job_count"`
	AvgSalaryMin   *int           `json:"avg_salary_min,omitempty" db:"avg_salary_min"`
	AvgSalaryMax   *int           `json:"avg_salary_max,omitempty" db:"avg_salary_max"`
	TopCompanies   []CompanyCount `json:"top_companies,omitempty" db:"top_companies"`
	SeniorityDist  map[string]int `json:"seniority_dist,omitempty" db:"seniority_dist"`
}

// CompanyCount is used in trend snapshots.
type CompanyCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
