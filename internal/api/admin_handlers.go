package api

import (
	"net/http"
	"strconv"
)

// ─────────────────────────────────────────────
// Admin: System Stats
// ─────────────────────────────────────────────

func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := s.pg.GetSystemStats(ctx)
	if err != nil {
		s.logger.Error("admin: failed to get system stats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// ─────────────────────────────────────────────
// Admin: Crawl Summary
// ─────────────────────────────────────────────

func (s *Server) handleAdminCrawlSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 365 {
		days = 30
	}

	summary, err := s.pg.GetCrawlSummary(ctx, days)
	if err != nil {
		s.logger.Error("admin: failed to get crawl summary", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// ─────────────────────────────────────────────
// Admin: User List
// ─────────────────────────────────────────────

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	users, total, err := s.pg.ListUsersAdmin(ctx, limit, offset)
	if err != nil {
		s.logger.Error("admin: failed to list users", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
