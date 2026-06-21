package matcher

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// Profile contains the user inputs used to rank jobs.
type Profile struct {
	ResumeText      string
	KnownSkills     []string
	LearningSkills  []string
	TargetRoles     []string
	TargetSeniority []string
	TargetLocations []string
}

// Result is a scored job recommendation.
type Result struct {
	Job                   models.Job `json:"job"`
	Score                 int        `json:"score"`
	SkillScore            int        `json:"skill_score"`
	PreferenceScore       int        `json:"preference_score"`
	FreshnessScore         int        `json:"freshness_score"`
	MatchedSkills         []string   `json:"matched_skills"`
	MissingRequiredSkills []string   `json:"missing_required_skills"`
	Reasons               []string   `json:"reasons"`
}

// Rank scores and sorts jobs for the supplied profile.
func Rank(profile Profile, jobs []models.Job, limit int) []Result {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	results := make([]Result, 0, len(jobs))
	for _, job := range jobs {
		result := Score(profile, job)
		if result.Score <= 0 {
			continue
		}
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Job.FirstSeenAt.After(results[j].Job.FirstSeenAt)
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// Score calculates a deterministic 0-100 match score for one job.
func Score(profile Profile, job models.Job) Result {
	required := skillNames(job.SkillsRequired)
	preferred := skillNames(job.SkillsPreferred)
	known := profileSkillSet(profile, append(required, preferred...))

	matchedRequired := matchingSkills(required, known)
	matchedPreferred := matchingSkills(preferred, known)
	missingRequired := missingSkills(required, known)

	requiredScore := coverageScore(len(matchedRequired), len(required), 42)
	preferredScore := coverageScore(len(matchedPreferred), len(preferred), 18)
	if len(required) == 0 && len(preferred) > 0 {
		preferredScore = coverageScore(len(matchedPreferred), len(preferred), 60)
	}
	skillScore := requiredScore + preferredScore

	preferenceScore, preferenceReasons := scorePreferences(profile, job)
	freshnessScore := scoreFreshness(job.FirstSeenAt)

	score := clamp(skillScore+preferenceScore+freshnessScore, 0, 100)
	reasons := buildReasons(matchedRequired, matchedPreferred, missingRequired, preferenceReasons, freshnessScore)

	return Result{
		Job:                   job,
		Score:                 score,
		SkillScore:            skillScore,
		PreferenceScore:       preferenceScore,
		FreshnessScore:         freshnessScore,
		MatchedSkills:         appendUnique(matchedRequired, matchedPreferred...),
		MissingRequiredSkills: missingRequired,
		Reasons:               reasons,
	}
}

func profileSkillSet(profile Profile, candidateSkills []string) map[string]string {
	skills := make(map[string]string)
	for _, skill := range profile.KnownSkills {
		addSkill(skills, skill)
	}

	resume := strings.ToLower(profile.ResumeText)
	for _, skill := range candidateSkills {
		normalized := normalize(skill)
		if normalized == "" {
			continue
		}
		if _, ok := skills[normalized]; ok {
			continue
		}
		if strings.Contains(resume, normalized) {
			skills[normalized] = canonical(skill)
		}
	}

	return skills
}

func scorePreferences(profile Profile, job models.Job) (int, []string) {
	score := 0
	var reasons []string

	title := strings.ToLower(job.Title + " " + job.NormalizedTitle)
	if matchedAny(title, profile.TargetRoles) {
		score += 12
		reasons = append(reasons, "target role match")
	}

	if containsCI(profile.TargetSeniority, job.SeniorityLevel) {
		score += 8
		reasons = append(reasons, "target seniority match")
	}

	locationText := strings.ToLower(job.Location + " " + job.LocationType)
	if matchedAny(locationText, profile.TargetLocations) {
		score += 8
		reasons = append(reasons, "target location match")
	}

	if len(matchingSkills(profile.LearningSkills, profileSkillSet(Profile{ResumeText: job.DescriptionClean + " " + job.DescriptionRaw}, profile.LearningSkills))) > 0 {
		score += 2
		reasons = append(reasons, "learning-goal overlap")
	}

	return score, reasons
}

func scoreFreshness(firstSeen time.Time) int {
	if firstSeen.IsZero() {
		return 0
	}
	ageDays := time.Since(firstSeen).Hours() / 24
	switch {
	case ageDays <= 3:
		return 10
	case ageDays <= 14:
		return 7
	case ageDays <= 30:
		return 4
	case ageDays <= 60:
		return 2
	default:
		return 0
	}
}

func buildReasons(required, preferred, missing, preferenceReasons []string, freshnessScore int) []string {
	var reasons []string
	if len(required) > 0 {
		reasons = append(reasons, "required skill overlap")
	}
	if len(preferred) > 0 {
		reasons = append(reasons, "preferred skill overlap")
	}
	reasons = append(reasons, preferenceReasons...)
	if freshnessScore >= 7 {
		reasons = append(reasons, "recent posting")
	}
	if len(missing) == 0 && len(required) > 0 {
		reasons = append(reasons, "no required skill gaps detected")
	}
	return appendUnique(nil, reasons...)
}

func skillNames(entries []models.SkillEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name != "" {
			names = append(names, entry.Name)
		}
	}
	return appendUnique(nil, names...)
}

func matchingSkills(skills []string, known map[string]string) []string {
	var matched []string
	for _, skill := range skills {
		if display, ok := known[normalize(skill)]; ok {
			matched = append(matched, display)
		}
	}
	return appendUnique(nil, matched...)
}

func missingSkills(required []string, known map[string]string) []string {
	var missing []string
	for _, skill := range required {
		if _, ok := known[normalize(skill)]; !ok {
			missing = append(missing, canonical(skill))
		}
	}
	return appendUnique(nil, missing...)
}

func coverageScore(matched, total, maxScore int) int {
	if total <= 0 || maxScore <= 0 {
		return 0
	}
	return int(math.Round(float64(matched) / float64(total) * float64(maxScore)))
}

func matchedAny(text string, values []string) bool {
	for _, value := range values {
		needle := normalize(value)
		if needle != "" && strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func containsCI(values []string, target string) bool {
	target = normalize(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if normalize(value) == target {
			return true
		}
	}
	return false
}

func addSkill(skills map[string]string, skill string) {
	normalized := normalize(skill)
	if normalized != "" {
		skills[normalized] = canonical(skill)
	}
}

func normalize(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func canonical(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func appendUnique(base []string, values ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(values))
	out := make([]string, 0, len(base)+len(values))
	for _, value := range append(base, values...) {
		normalized := normalize(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, canonical(value))
	}
	return out
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
