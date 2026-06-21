package api

import (
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleListSkillTrends(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	days := parseBoundedInt(r.URL.Query().Get("days"), 30, 1, 365)
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 100, 1, 500)
	skill := r.URL.Query().Get("skill")

	trends, err := s.pg.ListSkillTrends(ctx, skill, days, limit)
	if err != nil {
		s.logger.Error("failed to list skill trends", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"trends": trends,
		"total":  len(trends),
		"days":   days,
		"limit":  limit,
		"skill":  skill,
	})
}

func (s *Server) handleListCompanyTrends(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	days := parseBoundedInt(r.URL.Query().Get("days"), 30, 1, 365)
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 25, 1, 100)

	trends, err := s.pg.ListCompanyTrends(ctx, days, limit)
	if err != nil {
		s.logger.Error("failed to list company trends", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"companies": trends,
		"total":     len(trends),
		"days":      days,
		"limit":     limit,
	})
}

func (s *Server) handleListSalaryTrends(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := parseBoundedInt(r.URL.Query().Get("limit"), 50, 1, 200)
	skill := r.URL.Query().Get("skill")
	seniority := r.URL.Query().Get("seniority")

	trends, err := s.pg.ListSalaryTrends(ctx, skill, seniority, limit)
	if err != nil {
		s.logger.Error("failed to list salary trends", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"salaries":  trends,
		"total":     len(trends),
		"limit":     limit,
		"skill":     skill,
		"seniority": seniority,
	})
}

func (s *Server) handleRefreshTrends(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := parseBoundedInt(r.URL.Query().Get("limit"), 100, 1, 500)
	result, err := s.pg.RefreshTrendSnapshots(ctx, time.Now().UTC(), limit)
	if err != nil {
		s.logger.Error("failed to refresh trends", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func parseBoundedInt(raw string, fallback, min, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
