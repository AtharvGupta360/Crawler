package enricher

import (
	"regexp"
	"strings"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// ─────────────────────────────────────────────
// Rule-based enricher
// Fast, zero-cost, no network. Runs on every job.
// ─────────────────────────────────────────────

// EnrichRules populates seniority_level, location_type, employment_type,
// and normalized_title from title/location text using keyword matching.
// It never overwrites fields that already have values.
func EnrichRules(job *models.Job) {
	titleLow := strings.ToLower(job.Title)
	locationLow := strings.ToLower(job.Location)

	if job.SeniorityLevel == "" {
		job.SeniorityLevel = inferSeniority(titleLow)
	}

	if job.LocationType == "" {
		job.LocationType = inferLocationType(locationLow, titleLow)
	}

	if job.EmploymentType == "" {
		job.EmploymentType = inferEmploymentType(titleLow)
	}

	if job.NormalizedTitle == "" {
		job.NormalizedTitle = normalizeTitle(job.Title)
	}
}

// ─────────────────────────────────────────────
// Seniority inference
// ─────────────────────────────────────────────

var seniorityRules = []struct {
	level    string
	keywords []string
}{
	{models.SeniorityIntern, []string{"intern", "internship", "co-op", "coop", "apprentice"}},
	{models.SeniorityLead, []string{"lead ", "tech lead", "team lead", "staff+", "principal+"}},
	{models.SeniorityStaff, []string{"staff ", "principal ", "distinguished ", "fellow "}},
	{models.SenioritySenior, []string{"senior ", "sr.", "sr ", " senior", "snr"}},
	{models.SeniorityJunior, []string{"junior ", "jr.", "jr ", " junior", "entry level", "entry-level", "associate "}},
	{models.SeniorityMid, []string{"mid ", "mid-level", "midlevel", "ii ", "iii ", "iv "}},
}

func inferSeniority(titleLow string) string {
	for _, rule := range seniorityRules {
		for _, kw := range rule.keywords {
			if strings.Contains(titleLow, kw) {
				return rule.level
			}
		}
	}
	// If none matched and title has clear tech words, default to mid
	techWords := []string{"engineer", "developer", "programmer", "analyst", "scientist"}
	for _, w := range techWords {
		if strings.Contains(titleLow, w) {
			return models.SeniorityMid
		}
	}
	return ""
}

// ─────────────────────────────────────────────
// Location type inference
// ─────────────────────────────────────────────

func inferLocationType(locationLow, titleLow string) string {
	combined := locationLow + " " + titleLow

	remoteKws := []string{"remote", "work from home", "wfh", "anywhere", "distributed"}
	for _, kw := range remoteKws {
		if strings.Contains(combined, kw) {
			// Check for hybrid override
			hybridKws := []string{"hybrid", "partial remote", "flexible"}
			for _, h := range hybridKws {
				if strings.Contains(combined, h) {
					return models.LocationHybrid
				}
			}
			return models.LocationRemote
		}
	}

	hybridKws := []string{"hybrid", "partial remote", "2 days", "3 days"}
	for _, kw := range hybridKws {
		if strings.Contains(combined, kw) {
			return models.LocationHybrid
		}
	}

	// If location has a city name, infer onsite
	if locationLow != "" &&
		!strings.Contains(locationLow, "remote") &&
		!strings.Contains(locationLow, "hybrid") {
		return models.LocationOnsite
	}

	return ""
}

// ─────────────────────────────────────────────
// Employment type inference
// ─────────────────────────────────────────────

func inferEmploymentType(titleLow string) string {
	switch {
	case strings.Contains(titleLow, "intern") || strings.Contains(titleLow, "internship"):
		return models.EmploymentIntern
	case strings.Contains(titleLow, "contract") || strings.Contains(titleLow, "contractor") ||
		strings.Contains(titleLow, "freelance") || strings.Contains(titleLow, "consultant"):
		return models.EmploymentContract
	case strings.Contains(titleLow, "part time") || strings.Contains(titleLow, "part-time"):
		return models.EmploymentPartTime
	default:
		return models.EmploymentFullTime
	}
}

// ─────────────────────────────────────────────
// Title normalization
// ─────────────────────────────────────────────

// suffixRe strips parenthetical suffixes like "(Remote)", "[Contract]", "- US Only"
var suffixRe = regexp.MustCompile(`[\(\[\-][^)\]]*[\)\]]?\s*$`)

// tokenReplacements normalises common abbreviations in titles.
var tokenReplacements = []struct{ from, to string }{
	{"sr.", "Senior"},
	{"jr.", "Junior"},
	{"swe", "Software Engineer"},
	{"sde", "Software Development Engineer"},
	{"fe ", "Frontend "},
	{"be ", "Backend "},
}

func normalizeTitle(title string) string {
	t := strings.TrimSpace(title)

	// Strip trailing location/mode parentheticals
	t = suffixRe.ReplaceAllString(t, "")
	t = strings.TrimSpace(t)

	// Apply abbreviation replacements (case-insensitive)
	lower := strings.ToLower(t)
	for _, r := range tokenReplacements {
		if strings.Contains(lower, r.from) {
			t = strings.ReplaceAll(t, r.from, r.to)
			t = strings.ReplaceAll(t, strings.Title(r.from), r.to) //nolint:staticcheck
		}
	}

	return t
}
