package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ─────────────────────────────────────────────
// User operations
// ─────────────────────────────────────────────

// CreateUser inserts a new user. PasswordHash must already be set.
func (s *PostgresStore) CreateUser(ctx context.Context, u *models.User) error {
	u.ID = uuid.New()
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	if u.Role == "" {
		u.Role = "user"
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (
			id, email, password_hash, name, role,
			target_roles, target_seniority, target_locations,
			known_skills, learning_skills, resume_text,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		u.ID, u.Email, u.PasswordHash, u.Name, u.Role,
		u.TargetRoles, u.TargetSeniority, u.TargetLocations,
		u.KnownSkills, u.LearningSkills, u.ResumeText,
		u.CreatedAt, u.UpdatedAt,
	)
	return err
}

// GetUserByEmail fetches a user by email (for login).
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.scanUser(ctx, `
		SELECT id, email, password_hash, name, role,
		       target_roles, target_seniority, target_locations,
		       known_skills, learning_skills, resume_text,
		       created_at, updated_at
		FROM users WHERE email = $1
	`, email)
}

// GetUserByID fetches a user by UUID.
func (s *PostgresStore) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.scanUser(ctx, `
		SELECT id, email, password_hash, name, role,
		       target_roles, target_seniority, target_locations,
		       known_skills, learning_skills, resume_text,
		       created_at, updated_at
		FROM users WHERE id = $1
	`, id)
}

// UpdateUser updates mutable profile fields.
func (s *PostgresStore) UpdateUser(ctx context.Context, u *models.User) error {
	u.UpdatedAt = time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET
			name               = $2,
			target_roles       = $3,
			target_seniority   = $4,
			target_locations   = $5,
			known_skills       = $6,
			learning_skills    = $7,
			resume_text        = $8,
			updated_at         = $9
		WHERE id = $1
	`,
		u.ID, u.Name,
		u.TargetRoles, u.TargetSeniority, u.TargetLocations,
		u.KnownSkills, u.LearningSkills, u.ResumeText,
		u.UpdatedAt,
	)
	return err
}

func (s *PostgresStore) scanUser(ctx context.Context, query string, args ...any) (*models.User, error) {
	u := &models.User{}
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.TargetRoles, &u.TargetSeniority, &u.TargetLocations,
		&u.KnownSkills, &u.LearningSkills, &u.ResumeText,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// ─────────────────────────────────────────────
// Alert operations
// ─────────────────────────────────────────────

// CreateAlert inserts a new alert for a user.
func (s *PostgresStore) CreateAlert(ctx context.Context, a *models.Alert) error {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	if a.NotifyVia == "" {
		a.NotifyVia = "websocket"
	}
	a.IsActive = true

	filtersJSON, err := json.Marshal(a.Filters)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO alerts (id, user_id, name, filters, is_active, notify_via, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, a.ID, a.UserID, a.Name, filtersJSON, a.IsActive, a.NotifyVia, a.CreatedAt)
	return err
}

// ListAlerts returns all alerts for a user.
func (s *PostgresStore) ListAlerts(ctx context.Context, userID uuid.UUID) ([]models.Alert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, filters, is_active, notify_via, last_triggered, created_at
		FROM alerts WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanAlerts(rows)
}

// ListActiveAlerts returns all active alerts across all users (for the evaluator).
func (s *PostgresStore) ListActiveAlerts(ctx context.Context) ([]models.Alert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, filters, is_active, notify_via, last_triggered, created_at
		FROM alerts WHERE is_active = TRUE ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanAlerts(rows)
}

// GetAlertByID fetches a single alert.
func (s *PostgresStore) GetAlertByID(ctx context.Context, id uuid.UUID) (*models.Alert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, filters, is_active, notify_via, last_triggered, created_at
		FROM alerts WHERE id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alerts, err := s.scanAlerts(rows)
	if err != nil || len(alerts) == 0 {
		return nil, err
	}
	return &alerts[0], nil
}

// UpdateAlert updates name, filters, is_active, and notify_via.
func (s *PostgresStore) UpdateAlert(ctx context.Context, a *models.Alert) error {
	filtersJSON, err := json.Marshal(a.Filters)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE alerts SET name=$2, filters=$3, is_active=$4, notify_via=$5
		WHERE id=$1 AND user_id=$6
	`, a.ID, a.Name, filtersJSON, a.IsActive, a.NotifyVia, a.UserID)
	return err
}

// DeleteAlert hard-deletes an alert owned by the given user.
func (s *PostgresStore) DeleteAlert(ctx context.Context, alertID, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM alerts WHERE id=$1 AND user_id=$2`, alertID, userID)
	return err
}

// UpdateAlertTriggered stamps last_triggered = now for the given alert.
func (s *PostgresStore) UpdateAlertTriggered(ctx context.Context, alertID uuid.UUID) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `UPDATE alerts SET last_triggered=$2 WHERE id=$1`, alertID, now)
	return err
}

func (s *PostgresStore) scanAlerts(rows pgx.Rows) ([]models.Alert, error) {
	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		var filtersJSON []byte
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.Name, &filtersJSON,
			&a.IsActive, &a.NotifyVia, &a.LastTriggered, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		if filtersJSON != nil {
			json.Unmarshal(filtersJSON, &a.Filters) //nolint:errcheck
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// ─────────────────────────────────────────────
// Notification operations
// ─────────────────────────────────────────────

// CreateNotification persists an in-app notification.
func (s *PostgresStore) CreateNotification(ctx context.Context, n *models.Notification) error {
	n.ID = uuid.New()
	n.CreatedAt = time.Now()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notifications (id, user_id, alert_id, job_id, title, company, apply_url, is_read, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, n.ID, n.UserID, n.AlertID, n.JobID, n.Title, n.Company, n.ApplyURL, false, n.CreatedAt)
	return err
}

// ListNotifications returns paginated notifications for a user, newest first.
func (s *PostgresStore) ListNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Notification, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, alert_id, job_id, title, company, apply_url, is_read, created_at
		FROM notifications WHERE user_id=$1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifs []models.Notification
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.AlertID, &n.JobID,
			&n.Title, &n.Company, &n.ApplyURL, &n.IsRead, &n.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		notifs = append(notifs, n)
	}
	return notifs, total, rows.Err()
}

// MarkNotificationRead marks a single notification as read (ownership checked).
func (s *PostgresStore) MarkNotificationRead(ctx context.Context, notifID, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE notifications SET is_read=TRUE WHERE id=$1 AND user_id=$2`, notifID, userID)
	return err
}

// MarkAllNotificationsRead marks all unread notifications as read for a user.
func (s *PostgresStore) MarkAllNotificationsRead(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE notifications SET is_read=TRUE WHERE user_id=$1 AND is_read=FALSE`, userID)
	return err
}

// UnreadNotificationCount returns the count of unread notifications for a user.
func (s *PostgresStore) UnreadNotificationCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND is_read=FALSE`, userID).Scan(&count)
	return count, err
}
