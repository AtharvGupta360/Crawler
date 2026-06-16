package models

import (
	"time"

	"github.com/google/uuid"
)

// Company represents a company whose job postings we crawl.
type Company struct {
	ID          uuid.UUID         `json:"id" db:"id"`
	Name        string            `json:"name" db:"name"`
	Slug        string            `json:"slug" db:"slug"`
	Website     string            `json:"website,omitempty" db:"website"`
	LogoURL     string            `json:"logo_url,omitempty" db:"logo_url"`
	Industry    string            `json:"industry,omitempty" db:"industry"`
	SizeRange   string            `json:"size_range,omitempty" db:"size_range"`
	ATSPlatform string            `json:"ats_platform" db:"ats_platform"` // "greenhouse", "lever", "ashby"
	CareersURL  string            `json:"careers_url" db:"careers_url"`
	CrawlConfig map[string]string `json:"crawl_config,omitempty" db:"crawl_config"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" db:"updated_at"`
}

// ATSPlatform constants
const (
	ATSGreenhouse = "greenhouse"
	ATSLever      = "lever"
	ATSAshby      = "ashby"
	ATSGeneric    = "generic"
)
