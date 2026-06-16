-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- ─────────────────────────────────────────────
-- Companies
-- ─────────────────────────────────────────────
CREATE TABLE companies (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    website         TEXT,
    logo_url        TEXT,
    industry        TEXT,
    size_range      TEXT,
    ats_platform    TEXT NOT NULL,
    careers_url     TEXT NOT NULL,
    crawl_config    JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_companies_ats ON companies(ats_platform);
CREATE INDEX idx_companies_slug ON companies(slug);

-- ─────────────────────────────────────────────
-- Jobs (core entity)
-- ─────────────────────────────────────────────
CREATE TABLE jobs (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id          UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    external_id         TEXT,
    title               TEXT NOT NULL,
    normalized_title    TEXT,
    description_raw     TEXT NOT NULL,
    description_clean   TEXT,
    location            TEXT,
    location_type       TEXT,
    employment_type     TEXT,
    seniority_level     TEXT,
    salary_min          INTEGER,
    salary_max          INTEGER,
    salary_currency     TEXT DEFAULT 'USD',
    department          TEXT,
    team                TEXT,
    apply_url           TEXT NOT NULL,
    source_url          TEXT NOT NULL,

    -- AI-extracted fields
    skills_required     JSONB DEFAULT '[]',
    skills_preferred    JSONB DEFAULT '[]',
    experience_years_min INTEGER,
    experience_years_max INTEGER,
    education_level     TEXT,
    ai_summary          TEXT,
    embedding           vector(1536),

    -- Metadata
    first_seen_at       TIMESTAMPTZ DEFAULT NOW(),
    last_seen_at        TIMESTAMPTZ DEFAULT NOW(),
    is_active           BOOLEAN DEFAULT TRUE,
    content_hash        TEXT NOT NULL,

    UNIQUE(company_id, external_id)
);

CREATE INDEX idx_jobs_company ON jobs(company_id);
CREATE INDEX idx_jobs_active ON jobs(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_jobs_seniority ON jobs(seniority_level);
CREATE INDEX idx_jobs_location_type ON jobs(location_type);
CREATE INDEX idx_jobs_first_seen ON jobs(first_seen_at);
CREATE INDEX idx_jobs_content_hash ON jobs(content_hash);

-- ─────────────────────────────────────────────
-- Skills taxonomy
-- ─────────────────────────────────────────────
CREATE TABLE skills (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT UNIQUE NOT NULL,
    category    TEXT NOT NULL,
    aliases     TEXT[] DEFAULT '{}',
    parent_id   UUID REFERENCES skills(id),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ─────────────────────────────────────────────
-- Job-Skill junction (for analytics queries)
-- ─────────────────────────────────────────────
CREATE TABLE job_skills (
    job_id      UUID REFERENCES jobs(id) ON DELETE CASCADE,
    skill_id    UUID REFERENCES skills(id) ON DELETE CASCADE,
    importance  TEXT NOT NULL,
    PRIMARY KEY (job_id, skill_id)
);

CREATE INDEX idx_job_skills_skill ON job_skills(skill_id);

-- ─────────────────────────────────────────────
-- Crawl tracking
-- ─────────────────────────────────────────────
CREATE TABLE crawl_runs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id      UUID REFERENCES companies(id) ON DELETE CASCADE,
    started_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'running',
    jobs_found      INTEGER DEFAULT 0,
    jobs_new        INTEGER DEFAULT 0,
    jobs_updated    INTEGER DEFAULT 0,
    jobs_removed    INTEGER DEFAULT 0,
    error_message   TEXT,
    duration_ms     BIGINT
);

CREATE INDEX idx_crawl_runs_company ON crawl_runs(company_id);
CREATE INDEX idx_crawl_runs_status ON crawl_runs(status);

-- ─────────────────────────────────────────────
-- Users
-- ─────────────────────────────────────────────
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email           TEXT UNIQUE NOT NULL,
    name            TEXT,
    target_roles    TEXT[] DEFAULT '{}',
    target_seniority TEXT[] DEFAULT '{}',
    target_locations TEXT[] DEFAULT '{}',
    known_skills    TEXT[] DEFAULT '{}',
    learning_skills TEXT[] DEFAULT '{}',
    resume_text     TEXT,
    resume_embedding vector(1536),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ─────────────────────────────────────────────
-- Alerts
-- ─────────────────────────────────────────────
CREATE TABLE alerts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    filters         JSONB NOT NULL,
    is_active       BOOLEAN DEFAULT TRUE,
    notify_via      TEXT DEFAULT 'websocket',
    last_triggered  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_alerts_user ON alerts(user_id);
CREATE INDEX idx_alerts_active ON alerts(is_active) WHERE is_active = TRUE;

-- ─────────────────────────────────────────────
-- Trend snapshots (materialized daily)
-- ─────────────────────────────────────────────
CREATE TABLE trend_snapshots (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_date   DATE NOT NULL,
    skill_name      TEXT NOT NULL,
    job_count       INTEGER NOT NULL,
    avg_salary_min  INTEGER,
    avg_salary_max  INTEGER,
    top_companies   JSONB,
    seniority_dist  JSONB,
    UNIQUE(snapshot_date, skill_name)
);

CREATE INDEX idx_trends_date ON trend_snapshots(snapshot_date);
CREATE INDEX idx_trends_skill ON trend_snapshots(skill_name);
