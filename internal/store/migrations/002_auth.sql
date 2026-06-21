-- ─────────────────────────────────────────────
-- Phase 4b: User Auth + Alert System
-- ─────────────────────────────────────────────

-- Add auth fields to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';

-- ─────────────────────────────────────────────
-- Notifications inbox
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS notifications (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_id    UUID REFERENCES alerts(id) ON DELETE SET NULL,
    job_id      UUID REFERENCES jobs(id) ON DELETE SET NULL,
    title       TEXT NOT NULL,
    company     TEXT NOT NULL,
    apply_url   TEXT,
    is_read     BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_unread
    ON notifications(user_id, is_read)
    WHERE is_read = FALSE;
CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications(created_at DESC);
