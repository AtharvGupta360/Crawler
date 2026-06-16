package models

import (
	"time"

	"github.com/google/uuid"
)

// Job represents a normalized job posting from any ATS platform.
type Job struct {
	ID              uuid.UUID `json:"id" db:"id"`
	CompanyID       uuid.UUID `json:"company_id" db:"company_id"`
	ExternalID      string    `json:"external_id,omitempty" db:"external_id"`
	Title           string    `json:"title" db:"title"`
	NormalizedTitle string    `json:"normalized_title,omitempty" db:"normalized_title"`
	DescriptionRaw  string    `json:"description_raw" db:"description_raw"`
	DescriptionClean string   `json:"description_clean,omitempty" db:"description_clean"`
	Location        string    `json:"location,omitempty" db:"location"`
	LocationType    string    `json:"location_type,omitempty" db:"location_type"` // remote, hybrid, onsite
	EmploymentType  string    `json:"employment_type,omitempty" db:"employment_type"` // full_time, part_time, intern, contract
	SeniorityLevel  string    `json:"seniority_level,omitempty" db:"seniority_level"` // intern, junior, mid, senior, lead, staff
	SalaryMin       *int      `json:"salary_min,omitempty" db:"salary_min"`
	SalaryMax       *int      `json:"salary_max,omitempty" db:"salary_max"`
	SalaryCurrency  string    `json:"salary_currency,omitempty" db:"salary_currency"`
	Department      string    `json:"department,omitempty" db:"department"`
	Team            string    `json:"team,omitempty" db:"team"`
	ApplyURL        string    `json:"apply_url" db:"apply_url"`
	SourceURL       string    `json:"source_url" db:"source_url"`

	// AI-extracted fields
	SkillsRequired    []SkillEntry `json:"skills_required,omitempty" db:"skills_required"`
	SkillsPreferred   []SkillEntry `json:"skills_preferred,omitempty" db:"skills_preferred"`
	ExperienceYearsMin *int        `json:"experience_years_min,omitempty" db:"experience_years_min"`
	ExperienceYearsMax *int        `json:"experience_years_max,omitempty" db:"experience_years_max"`
	EducationLevel    string       `json:"education_level,omitempty" db:"education_level"`
	AISummary         string       `json:"ai_summary,omitempty" db:"ai_summary"`

	// Metadata
	FirstSeenAt time.Time `json:"first_seen_at" db:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at" db:"last_seen_at"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	ContentHash string    `json:"content_hash" db:"content_hash"`

	// Relations (populated on read, not stored in jobs table)
	Company *Company `json:"company,omitempty" db:"-"`
}

// SkillEntry represents a skill extracted from a job description.
type SkillEntry struct {
	Name       string `json:"name"`
	Category   string `json:"category"`   // "language", "framework", "tool", "concept"
	Importance string `json:"importance"` // "required", "preferred", "nice_to_have"
}

// LocationType constants
const (
	LocationRemote = "remote"
	LocationHybrid = "hybrid"
	LocationOnsite = "onsite"
)

// EmploymentType constants
const (
	EmploymentFullTime = "full_time"
	EmploymentPartTime = "part_time"
	EmploymentIntern   = "intern"
	EmploymentContract = "contract"
)

// SeniorityLevel constants
const (
	SeniorityIntern  = "intern"
	SeniorityJunior  = "junior"
	SeniorityMid     = "mid"
	SenioritySenior  = "senior"
	SeniorityLead    = "lead"
	SeniorityStaff   = "staff"
)
