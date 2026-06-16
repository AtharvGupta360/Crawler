package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/AtharvGupta360/JobCrawl/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ─────────────────────────────────────────────
// Health Check
// ─────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	health := map[string]any{
		"status": "ok",
		"checks": map[string]string{},
	}

	checks := health["checks"].(map[string]string)

	// PostgreSQL
	if err := s.pg.HealthCheck(ctx); err != nil {
		checks["postgres"] = "unhealthy: " + err.Error()
		health["status"] = "degraded"
	} else {
		checks["postgres"] = "healthy"
	}

	// Redis
	if err := s.redis.HealthCheck(ctx); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		health["status"] = "degraded"
	} else {
		checks["redis"] = "healthy"
	}

	status := http.StatusOK
	if health["status"] != "ok" {
		status = http.StatusServiceUnavailable
	}

	writeJSON(w, status, health)
}

// ─────────────────────────────────────────────
// Jobs Handlers
// ─────────────────────────────────────────────

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	filter := store.JobFilter{
		SeniorityLevel: r.URL.Query().Get("seniority"),
		LocationType:   r.URL.Query().Get("location_type"),
		Limit:          limit,
		Offset:         offset,
	}

	if companyID := r.URL.Query().Get("company_id"); companyID != "" {
		if id, err := uuid.Parse(companyID); err == nil {
			filter.CompanyID = id
		}
	}

	jobs, total, err := s.pg.ListJobs(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list jobs", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"jobs":   jobs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "jobID")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid job ID"})
		return
	}

	job, err := s.pg.GetJobByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get job", "error", err, "job_id", idStr)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if job == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleJobStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := s.pg.GetJobStats(ctx)
	if err != nil {
		s.logger.Error("failed to get job stats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// ─────────────────────────────────────────────
// Companies Handlers
// ─────────────────────────────────────────────

func (s *Server) handleListCompanies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	companies, err := s.pg.ListCompanies(ctx)
	if err != nil {
		s.logger.Error("failed to list companies", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"companies": companies,
		"total":     len(companies),
	})
}

func (s *Server) handleCreateCompany(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var company models.Company
	if err := json.NewDecoder(r.Body).Decode(&company); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if company.Name == "" || company.Slug == "" || company.ATSPlatform == "" || company.CareersURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "name, slug, ats_platform, and careers_url are required",
		})
		return
	}

	// Check if slug already exists
	existing, err := s.pg.GetCompanyBySlug(ctx, company.Slug)
	if err != nil {
		s.logger.Error("failed to check company slug", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "company with this slug already exists"})
		return
	}

	if err := s.pg.CreateCompany(ctx, &company); err != nil {
		s.logger.Error("failed to create company", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, company)
}

// ─────────────────────────────────────────────
// Crawl Management Handlers
// ─────────────────────────────────────────────

func (s *Server) handleTriggerCrawl(w http.ResponseWriter, r *http.Request) {
	// Placeholder — will be implemented when crawlers are built in Phase 2
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "crawl trigger endpoint ready — crawlers will be implemented in Phase 2",
	})
}

func (s *Server) handleListCrawlRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	runs, err := s.pg.GetRecentCrawlRuns(ctx, limit)
	if err != nil {
		s.logger.Error("failed to list crawl runs", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"runs":  runs,
		"total": len(runs),
	})
}

func (s *Server) handleCrawlerHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	platforms := []string{"greenhouse", "lever", "ashby"}
	health := make(map[string]any)

	for _, platform := range platforms {
		healthy, msg, err := s.redis.GetCrawlerHealth(ctx, platform)
		if err != nil {
			health[platform] = map[string]any{"healthy": false, "message": "check failed: " + err.Error()}
		} else {
			health[platform] = map[string]any{"healthy": healthy, "message": msg}
		}
	}

	writeJSON(w, http.StatusOK, health)
}

// ─────────────────────────────────────────────
// Response helpers
// ─────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
