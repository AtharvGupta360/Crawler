package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	jwtauth "github.com/AtharvGupta360/JobCrawl/internal/auth"
	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const alertCacheKey = "cache:alerts:active"

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins in development; lock this down in production
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ─────────────────────────────────────────────
// Alert CRUD
// ─────────────────────────────────────────────

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	alerts, err := s.pg.ListAlerts(ctx, claims.UserID)
	if err != nil {
		s.logger.Error("list-alerts: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if alerts == nil {
		alerts = []models.Alert{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

func (s *Server) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var alert models.Alert
	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if alert.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	alert.UserID = claims.UserID
	if err := s.pg.CreateAlert(ctx, &alert); err != nil {
		s.logger.Error("create-alert: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Invalidate alert cache
	s.redis.DeleteCache(ctx, alertCacheKey) //nolint:errcheck

	writeJSON(w, http.StatusCreated, alert)
}

func (s *Server) handleUpdateAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	alertID, err := uuid.Parse(chi.URLParam(r, "alertID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid alert ID"})
		return
	}

	existing, err := s.pg.GetAlertByID(ctx, alertID)
	if err != nil || existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert not found"})
		return
	}
	if existing.UserID != claims.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	var updates models.Alert
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	existing.Name = updates.Name
	existing.Filters = updates.Filters
	existing.IsActive = updates.IsActive
	existing.NotifyVia = updates.NotifyVia
	if existing.NotifyVia == "" {
		existing.NotifyVia = "websocket"
	}

	if err := s.pg.UpdateAlert(ctx, existing); err != nil {
		s.logger.Error("update-alert: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	s.redis.DeleteCache(ctx, alertCacheKey) //nolint:errcheck
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	alertID, err := uuid.Parse(chi.URLParam(r, "alertID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid alert ID"})
		return
	}

	// Ownership check embedded in delete query
	if err := s.pg.DeleteAlert(ctx, alertID, claims.UserID); err != nil {
		s.logger.Error("delete-alert: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	s.redis.DeleteCache(ctx, alertCacheKey) //nolint:errcheck
	writeJSON(w, http.StatusOK, map[string]string{"message": "alert deleted"})
}

// ─────────────────────────────────────────────
// Notifications Inbox
// ─────────────────────────────────────────────

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	notifs, total, err := s.pg.ListNotifications(ctx, claims.UserID, limit, offset)
	if err != nil {
		s.logger.Error("list-notifications: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	unread, _ := s.pg.UnreadNotificationCount(ctx, claims.UserID)

	if notifs == nil {
		notifs = []models.Notification{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": notifs,
		"total":         total,
		"unread":        unread,
		"limit":         limit,
		"offset":        offset,
	})
}

func (s *Server) handleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if err := s.pg.MarkAllNotificationsRead(ctx, claims.UserID); err != nil {
		s.logger.Error("mark-all-read: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "all notifications marked as read"})
}

func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	notifID, err := uuid.Parse(chi.URLParam(r, "notifID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid notification ID"})
		return
	}

	if err := s.pg.MarkNotificationRead(ctx, notifID, claims.UserID); err != nil {
		s.logger.Error("mark-read: db error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "notification marked as read"})
}

// ─────────────────────────────────────────────
// WebSocket endpoint
// ─────────────────────────────────────────────

// handleWebSocket upgrades an HTTP connection to WebSocket. Authentication is
// performed via the ?token= query parameter since browsers cannot set custom headers.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token query parameter required"})
		return
	}

	claims, err := jwtauth.ValidateToken(s.jwtSecret, tokenStr)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("ws upgrade failed", "error", err, "user_id", claims.UserID)
		return
	}

	s.logger.Info("ws client connected", "user_id", claims.UserID)
	s.wsHub.NewClient(claims.UserID, conn)
}
