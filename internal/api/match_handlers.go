package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/AtharvGupta360/JobCrawl/internal/matcher"
)

type resumeMatchRequest struct {
	ResumeText      string   `json:"resume_text"`
	KnownSkills     []string `json:"known_skills"`
	LearningSkills  []string `json:"learning_skills"`
	TargetRoles     []string `json:"target_roles"`
	TargetSeniority []string `json:"target_seniority"`
	TargetLocations []string `json:"target_locations"`
	Limit           int      `json:"limit"`
	CandidateLimit  int      `json:"candidate_limit"`
	SaveToProfile   bool     `json:"save_to_profile"`
}

func (s *Server) handleMatchResume(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req resumeMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	req.ResumeText = strings.TrimSpace(req.ResumeText)
	if req.ResumeText == "" && len(req.KnownSkills) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "resume_text or known_skills is required"})
		return
	}

	user, err := s.pg.GetUserByID(ctx, claims.UserID)
	if err != nil || user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	profile := matcher.Profile{
		ResumeText:      firstNonEmpty(req.ResumeText, user.ResumeText),
		KnownSkills:     firstNonEmptySlice(req.KnownSkills, user.KnownSkills),
		LearningSkills:  firstNonEmptySlice(req.LearningSkills, user.LearningSkills),
		TargetRoles:     firstNonEmptySlice(req.TargetRoles, user.TargetRoles),
		TargetSeniority: firstNonEmptySlice(req.TargetSeniority, user.TargetSeniority),
		TargetLocations: firstNonEmptySlice(req.TargetLocations, user.TargetLocations),
	}

	if req.SaveToProfile {
		user.ResumeText = profile.ResumeText
		user.KnownSkills = profile.KnownSkills
		user.LearningSkills = profile.LearningSkills
		user.TargetRoles = profile.TargetRoles
		user.TargetSeniority = profile.TargetSeniority
		user.TargetLocations = profile.TargetLocations
		if err := s.pg.UpdateUser(ctx, user); err != nil {
			s.logger.Error("match-resume: failed to update profile", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
	}

	results, candidateCount, err := s.matchJobs(ctx, profile, req.Limit, req.CandidateLimit)
	if err != nil {
		s.logger.Error("match-resume: failed to match jobs", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"matches":         results,
		"total":           len(results),
		"candidates":      candidateCount,
		"embedding_model": nil,
		"scoring":         "deterministic_v1",
	})
}

func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	user, err := s.pg.GetUserByID(ctx, claims.UserID)
	if err != nil || user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	profile := matcher.Profile{
		ResumeText:      user.ResumeText,
		KnownSkills:     user.KnownSkills,
		LearningSkills:  user.LearningSkills,
		TargetRoles:     user.TargetRoles,
		TargetSeniority: user.TargetSeniority,
		TargetLocations: user.TargetLocations,
	}

	results, candidateCount, err := s.matchJobs(
		ctx,
		profile,
		parseBoundedInt(r.URL.Query().Get("limit"), 20, 1, 100),
		parseBoundedInt(r.URL.Query().Get("candidate_limit"), 500, 50, 2000),
	)
	if err != nil {
		s.logger.Error("recommendations: failed to match jobs", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recommendations": results,
		"total":           len(results),
		"candidates":      candidateCount,
		"scoring":         "deterministic_v1",
	})
}

func (s *Server) matchJobs(ctx context.Context, profile matcher.Profile, limit, candidateLimit int) ([]matcher.Result, int, error) {
	limit = boundInt(limit, 20, 1, 100)
	candidateLimit = boundInt(candidateLimit, 500, 50, 2000)

	jobs, err := s.pg.ListMatchCandidates(ctx, candidateLimit)
	if err != nil {
		return nil, 0, err
	}

	return matcher.Rank(profile, jobs, limit), len(jobs), nil
}

func firstNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func firstNonEmptySlice(primary, fallback []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

func boundInt(value, fallback, min, max int) int {
	if value == 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
