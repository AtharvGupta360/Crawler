package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	jwtauth "github.com/AtharvGupta360/JobCrawl/internal/auth"
	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// ─────────────────────────────────────────────
// Register
// ─────────────────────────────────────────────

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}

	// Check uniqueness
	existing, err := s.pg.GetUserByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Error("register: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
		return
	}

	hash, err := jwtauth.HashPassword(req.Password)
	if err != nil {
		s.logger.Error("register: bcrypt error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	user := &models.User{
		Email:        req.Email,
		PasswordHash: hash,
		Name:         req.Name,
		Role:         "user",
	}
	if err := s.pg.CreateUser(ctx, user); err != nil {
		s.logger.Error("register: create user error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	token, err := jwtauth.GenerateToken(s.jwtSecret, jwtauth.DefaultTTL, user)
	if err != nil {
		s.logger.Error("register: token generation error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"expires_in": int(jwtauth.DefaultTTL.Seconds()),
		"user":       user,
	})
}

// ─────────────────────────────────────────────
// Login
// ─────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	user, err := s.pg.GetUserByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Error("login: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	// Return generic error to avoid user enumeration
	if user == nil || !jwtauth.CheckPassword(user.PasswordHash, req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	token, err := jwtauth.GenerateToken(s.jwtSecret, jwtauth.DefaultTTL, user)
	if err != nil {
		s.logger.Error("login: token generation error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_in": int(jwtauth.DefaultTTL.Seconds()),
		"expires_at": time.Now().Add(jwtauth.DefaultTTL).UTC(),
		"user":       user,
	})
}

// ─────────────────────────────────────────────
// Get current user (GET /auth/me)
// ─────────────────────────────────────────────

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, user)
}

// ─────────────────────────────────────────────
// Update current user (PUT /auth/me)
// ─────────────────────────────────────────────

type updateMeRequest struct {
	Name            string   `json:"name"`
	TargetRoles     []string `json:"target_roles"`
	TargetSeniority []string `json:"target_seniority"`
	TargetLocations []string `json:"target_locations"`
	KnownSkills     []string `json:"known_skills"`
	LearningSkills  []string `json:"learning_skills"`
	ResumeText      string   `json:"resume_text"`
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req updateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	user, err := s.pg.GetUserByID(ctx, claims.UserID)
	if err != nil || user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	user.Name = req.Name
	user.TargetRoles = req.TargetRoles
	user.TargetSeniority = req.TargetSeniority
	user.TargetLocations = req.TargetLocations
	user.KnownSkills = req.KnownSkills
	user.LearningSkills = req.LearningSkills
	user.ResumeText = req.ResumeText

	if err := s.pg.UpdateUser(ctx, user); err != nil {
		s.logger.Error("update-me: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, user)
}
