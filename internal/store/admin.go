package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────
// Admin queries
// ─────────────────────────────────────────────

// SystemStats holds aggregate counts for the admin dashboard.
type SystemStats struct {
	TotalJobs         int `json:"total_jobs"`
	ActiveJobs        int `json:"active_jobs"`
	TotalCompanies    int `json:"total_companies"`
	TotalUsers        int `json:"total_users"`
	TotalAlerts       int `json:"total_alerts"`
	ActiveAlerts      int `json:"active_alerts"`
	TotalNotifications int `json:"total_notifications"`
	UnreadNotifications int `json:"unread_notifications"`
	TotalCrawlRuns    int `json:"total_crawl_runs"`
}

// GetSystemStats returns aggregate counts across all major tables.
func (s *PostgresStore) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	stats := &SystemStats{}
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM jobs),
			(SELECT COUNT(*) FROM jobs WHERE is_active = TRUE),
			(SELECT COUNT(*) FROM companies),
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM alerts),
			(SELECT COUNT(*) FROM alerts WHERE is_active = TRUE),
			(SELECT COUNT(*) FROM notifications),
			(SELECT COUNT(*) FROM notifications WHERE is_read = FALSE),
			(SELECT COUNT(*) FROM crawl_runs)
	`).Scan(
		&stats.TotalJobs,
		&stats.ActiveJobs,
		&stats.TotalCompanies,
		&stats.TotalUsers,
		&stats.TotalAlerts,
		&stats.ActiveAlerts,
		&stats.TotalNotifications,
		&stats.UnreadNotifications,
		&stats.TotalCrawlRuns,
	)
	return stats, err
}

// CrawlSummary holds aggregate crawl run statistics.
type CrawlSummary struct {
	TotalRuns       int     `json:"total_runs"`
	SuccessfulRuns  int     `json:"successful_runs"`
	FailedRuns      int     `json:"failed_runs"`
	TotalJobsFound  int     `json:"total_jobs_found"`
	TotalJobsNew    int     `json:"total_jobs_new"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`
	LastCrawlAt     *time.Time `json:"last_crawl_at,omitempty"`
}

// GetCrawlSummary returns aggregate stats over crawl runs from the last N days.
func (s *PostgresStore) GetCrawlSummary(ctx context.Context, days int) (*CrawlSummary, error) {
	if days <= 0 {
		days = 30
	}
	summary := &CrawlSummary{}
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'completed'),
			COUNT(*) FILTER (WHERE status = 'failed'),
			COALESCE(SUM(jobs_found), 0),
			COALESCE(SUM(jobs_new), 0),
			COALESCE(AVG(duration_ms) FILTER (WHERE duration_ms > 0), 0),
			MAX(started_at)
		FROM crawl_runs
		WHERE started_at >= NOW() - ($1 || ' days')::interval
	`, days).Scan(
		&summary.TotalRuns,
		&summary.SuccessfulRuns,
		&summary.FailedRuns,
		&summary.TotalJobsFound,
		&summary.TotalJobsNew,
		&summary.AvgDurationMs,
		&summary.LastCrawlAt,
	)
	return summary, err
}

// AdminUser is a user summary for the admin user list (no password hash).
type AdminUser struct {
	ID               uuid.UUID  `json:"id"`
	Email            string     `json:"email"`
	Name             string     `json:"name,omitempty"`
	Role             string     `json:"role"`
	AlertCount       int        `json:"alert_count"`
	NotificationCount int       `json:"notification_count"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ListUsersAdmin returns a paginated list of users with their alert and notification counts.
func (s *PostgresStore) ListUsersAdmin(ctx context.Context, limit, offset int) ([]AdminUser, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			u.id, u.email, u.name, u.role,
			(SELECT COUNT(*) FROM alerts WHERE user_id = u.id) AS alert_count,
			(SELECT COUNT(*) FROM notifications WHERE user_id = u.id) AS notification_count,
			u.created_at, u.updated_at
		FROM users u
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []AdminUser
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.Role,
			&u.AlertCount, &u.NotificationCount,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}
