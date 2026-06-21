# AI-Powered Job Intelligence Platform — Implementation Plan

## Problem Statement

CS students and job seekers waste 5-10 hours/week manually scanning job boards, comparing requirements, and trying to understand hiring trends. This platform crawls job postings from major ATS platforms (Greenhouse, Lever, Ashby), normalizes the data, uses AI to extract structured skills/requirements, and provides semantic search, resume matching, trend analytics, and real-time alerts — all through a clean React dashboard.

## Architecture Decisions

### Modular Monolith (Not Microservices)

> [!IMPORTANT]
> **Why no microservices?** A solo developer maintaining 6+ microservices means 6x deployment pipelines, distributed tracing nightmares, network serialization overhead where function calls would do, and eventual consistency problems that add weeks of debugging. A **modular monolith** gives you clean package boundaries, testable interfaces, and a single deployable binary — with the option to extract services later if needed. Every major company (Shopify, Basecamp, GitHub) started as a monolith.

### Kafka (Event Backbone)

> [!NOTE]
> **Why Kafka?** Kafka serves as the decoupling layer between the crawl pipeline and the processing/intelligence pipeline. This gives us:
> - **Backpressure handling**: If the AI extraction API is slow or down, crawled jobs queue up in Kafka and get processed when capacity returns. No data loss.
> - **Independent consumers**: Indexing (ES), AI extraction, and alert evaluation all consume from the same topic independently. Adding a new consumer (e.g., analytics) requires zero changes to the crawler.
> - **Replay**: If we fix a bug in the normalizer, we can replay the topic to reprocess all jobs.
> - **Ordering guarantees**: Partition by company ID to ensure jobs from the same company are processed in order.
>
> **Topics:**
> | Topic | Producer | Consumers | Partitions |
> |---|---|---|---|
> | `jobs.raw` | Crawlers | Normalizer + AI Extractor | 6 (by company hash) |
> | `jobs.processed` | AI Extractor | ES Indexer, Alert Evaluator, Trend Aggregator | 6 |
> | `jobs.alerts` | Alert Evaluator | WebSocket Notifier | 3 |
>
> **KRaft mode** (no ZooKeeper) — simpler operational footprint.

### No Kubernetes

> [!IMPORTANT]
> Docker Compose gives us multi-container orchestration for development and single-node production. K8s makes sense at 10+ services with auto-scaling needs. Docker Compose → single VPS deployment is the right call for this stage.

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        React Frontend                           │
│  Dashboard │ Search │ Resume Match │ Trends │ Alerts │ Profile  │
└──────────────────────────┬──────────────────────────────────────┘
                           │ REST + WebSocket
┌──────────────────────────▼──────────────────────────────────────┐
│                     Go API Server (Chi Router)                   │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐          │
│  │ Jobs API │ │Search API│ │Match API │ │Trends API │          │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └─────┬─────┘          │
│       │             │            │              │                │
│  ┌────▼─────────────▼────────────▼──────────────▼─────┐         │
│  │              Internal Service Layer                 │         │
│  │  CrawlService │ AIService │ MatchService │ AlertSvc│         │
│  └────┬──────────────┬───────────┬──────────────┬─────┘         │
│       │              │           │              │                │
│  ┌────▼────┐   ┌─────▼────┐ ┌───▼───┐   ┌─────▼─────┐         │
│  │Crawlers │   │OpenAI/   │ │ Redis │   │PostgreSQL │         │
│  │(per ATS)│   │Gemini API│ │ Cache │   │  + PgVec  │         │
│  └────┬────┘   └──────────┘ └───────┘   └───────────┘         │
│       │                                                         │
│  ┌────▼───────┐  ┌───────────────────┐                          │
│  │   Kafka    │  │  Elasticsearch    │                          │
│  │ (KRaft)   │  │ (Search + Vector) │                          │
│  │            │  │                   │                          │
│  │ jobs.raw ──┼──▶ Consumers:        │                          │
│  │ jobs.proc  │  │  AI Extract       │                          │
│  │ jobs.alert │  │  ES Index         │                          │
│  └────────────┘  │  Alert Eval       │                          │
│                  └───────────────────┘                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## Technology Stack (Justified)

| Tech | Role | Why This, Not That |
|---|---|---|
| **Go 1.22+** | Backend | Goroutines for concurrent crawling, single binary deployment, excellent HTTP stdlib |
| **PostgreSQL 16** | Primary DB | JSONB for flexible job data, `pgvector` for embeddings, LISTEN/NOTIFY for events |
| **Redis 7** | Cache + Rate Limit | Crawl deduplication, API rate limiting, session cache, real-time pub/sub for alerts |
| **Elasticsearch 8** | Search | Full-text search over job descriptions, aggregation queries for trend analytics |
| **React 18 + Vite** | Frontend | Fast dev experience, component ecosystem, good enough for a dashboard |
| **OpenAI / Gemini** | AI Layer | Skill extraction, job description summarization, resume-job scoring |
| **Docker Compose** | Deployment | Multi-container orchestration without K8s overhead |
| **Chi** | HTTP Router | Lightweight, idiomatic Go, middleware support |

---

## Project Structure (Go Monolith)

```
crawler/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point
├── internal/
│   ├── config/
│   │   └── config.go               # Env-based configuration
│   ├── crawler/
│   │   ├── crawler.go              # Core crawler interface
│   │   ├── greenhouse.go           # Greenhouse ATS crawler
│   │   ├── lever.go                # Lever ATS crawler
│   │   ├── ashby.go                # Ashby ATS crawler
│   │   ├── generic.go              # Generic career page crawler
│   │   ├── scheduler.go            # Cron-based crawl scheduling
│   │   └── ratelimiter.go          # Per-domain rate limiting
│   ├── models/
│   │   ├── job.go                  # Job domain model
│   │   ├── company.go              # Company model
│   │   ├── skill.go                # Skill taxonomy
│   │   ├── user.go                 # User profile + preferences
│   │   └── alert.go                # Alert rules
│   ├── store/
│   │   ├── postgres.go             # PostgreSQL repository
│   │   ├── redis.go                # Redis cache layer
│   │   ├── elasticsearch.go        # ES indexing + search
│   │   └── migrations/
│   │       ├── 001_initial.sql
│   │       ├── 002_skills.sql
│   │       └── 003_alerts.sql
│   ├── ai/
│   │   ├── extractor.go            # Skill extraction from job descriptions
│   │   ├── matcher.go              # Resume-job matching scorer
│   │   ├── embeddings.go           # Text → vector embeddings
│   │   └── provider.go             # OpenAI/Gemini abstraction
│   ├── api/
│   │   ├── router.go               # Chi router setup
│   │   ├── middleware.go           # Auth, rate limit, CORS, logging
│   │   ├── jobs_handler.go         # Job CRUD + search endpoints
│   │   ├── match_handler.go        # Resume matching endpoints
│   │   ├── trends_handler.go       # Analytics endpoints
│   │   ├── alerts_handler.go       # Alert management endpoints
│   │   ├── user_handler.go         # User profile endpoints
│   │   └── ws_handler.go           # WebSocket for real-time alerts
│   └── worker/
│       ├── pipeline.go             # Crawl → Normalize → AI Extract → Index pipeline
│       └── notifier.go             # Alert evaluation + notification
├── web/                            # React frontend (Vite)
│   ├── src/
│   │   ├── pages/
│   │   ├── components/
│   │   └── hooks/
│   ├── package.json
│   └── vite.config.js
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

---

## Database Schema (PostgreSQL)

```sql
-- Companies table
CREATE TABLE companies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    website         TEXT,
    logo_url        TEXT,
    industry        TEXT,
    size_range      TEXT,           -- "1-50", "51-200", "201-500", etc.
    ats_platform    TEXT NOT NULL,  -- 'greenhouse', 'lever', 'ashby', 'generic'
    careers_url     TEXT NOT NULL,
    crawl_config    JSONB,         -- per-company crawl settings
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Jobs table (core entity)
CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID NOT NULL REFERENCES companies(id),
    external_id     TEXT,          -- ATS-specific job ID
    title           TEXT NOT NULL,
    normalized_title TEXT,         -- AI-normalized: "SDE II" → "Software Engineer"
    description_raw TEXT NOT NULL, -- Original HTML/markdown
    description_clean TEXT,       -- Cleaned plaintext
    location        TEXT,
    location_type   TEXT,         -- 'remote', 'hybrid', 'onsite'
    employment_type TEXT,         -- 'full_time', 'part_time', 'intern', 'contract'
    seniority_level TEXT,         -- 'intern', 'junior', 'mid', 'senior', 'lead', 'staff'
    salary_min      INTEGER,
    salary_max      INTEGER,
    salary_currency TEXT DEFAULT 'USD',
    department      TEXT,
    team            TEXT,
    apply_url       TEXT NOT NULL,
    source_url      TEXT NOT NULL, -- Where we crawled it from
    
    -- AI-extracted fields (populated by the AI pipeline)
    skills_required JSONB,        -- [{"name": "Go", "category": "language", "importance": "required"}]
    skills_preferred JSONB,       -- Same structure
    experience_years_min INTEGER,
    experience_years_max INTEGER,
    education_level TEXT,
    ai_summary      TEXT,         -- 2-3 sentence AI summary
    embedding       vector(1536), -- pgvector for semantic search
    
    -- Metadata
    first_seen_at   TIMESTAMPTZ DEFAULT NOW(),
    last_seen_at    TIMESTAMPTZ DEFAULT NOW(),
    is_active       BOOLEAN DEFAULT TRUE,
    content_hash    TEXT NOT NULL, -- For deduplication / change detection
    
    UNIQUE(company_id, external_id)
);

CREATE INDEX idx_jobs_company ON jobs(company_id);
CREATE INDEX idx_jobs_active ON jobs(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_jobs_seniority ON jobs(seniority_level);
CREATE INDEX idx_jobs_location_type ON jobs(location_type);
CREATE INDEX idx_jobs_first_seen ON jobs(first_seen_at);
CREATE INDEX idx_jobs_embedding ON jobs USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Skills taxonomy (reference table)
CREATE TABLE skills (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT UNIQUE NOT NULL,     -- "Go", "React", "Kubernetes"
    category    TEXT NOT NULL,            -- "language", "framework", "tool", "concept"
    aliases     TEXT[],                   -- ["Golang", "Go lang"]
    parent_id   UUID REFERENCES skills(id), -- For hierarchy: "React" → "Frontend"
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Job-Skill junction (denormalized from JSONB for analytics)
CREATE TABLE job_skills (
    job_id      UUID REFERENCES jobs(id) ON DELETE CASCADE,
    skill_id    UUID REFERENCES skills(id),
    importance  TEXT NOT NULL,            -- 'required', 'preferred', 'nice_to_have'
    PRIMARY KEY (job_id, skill_id)
);

CREATE INDEX idx_job_skills_skill ON job_skills(skill_id);

-- Crawl tracking
CREATE TABLE crawl_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID REFERENCES companies(id),
    started_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'running',  -- 'running', 'completed', 'failed'
    jobs_found      INTEGER DEFAULT 0,
    jobs_new        INTEGER DEFAULT 0,
    jobs_updated    INTEGER DEFAULT 0,
    jobs_removed    INTEGER DEFAULT 0,
    error_message   TEXT,
    duration_ms     INTEGER
);

-- User profiles
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT UNIQUE NOT NULL,
    name            TEXT,
    target_roles    TEXT[],               -- ["backend engineer", "SDE"]
    target_seniority TEXT[],              -- ["junior", "mid"]
    target_locations TEXT[],              -- ["remote", "San Francisco"]
    known_skills    TEXT[],               -- ["Go", "Python", "PostgreSQL"]
    learning_skills TEXT[],              -- ["Kubernetes", "System Design"]
    resume_text     TEXT,                 -- Parsed resume plaintext
    resume_embedding vector(1536),        -- Resume vector
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Alert rules
CREATE TABLE alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    filters         JSONB NOT NULL,       -- {"skills": ["Go"], "seniority": ["junior"], "location_type": ["remote"]}
    is_active       BOOLEAN DEFAULT TRUE,
    notify_via      TEXT DEFAULT 'websocket', -- 'websocket', 'email'
    last_triggered  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Trend snapshots (materialized daily)
CREATE TABLE trend_snapshots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_date   DATE NOT NULL,
    skill_name      TEXT NOT NULL,
    job_count       INTEGER NOT NULL,     -- # of active jobs requiring this skill
    avg_salary_min  INTEGER,
    avg_salary_max  INTEGER,
    top_companies   JSONB,                -- [{"name": "Google", "count": 15}]
    seniority_dist  JSONB,                -- {"junior": 10, "mid": 25, "senior": 15}
    UNIQUE(snapshot_date, skill_name)
);

CREATE INDEX idx_trends_date ON trend_snapshots(snapshot_date);
CREATE INDEX idx_trends_skill ON trend_snapshots(skill_name);
```

---

## Crawler Architecture

### Per-ATS Crawler Design

Each ATS has a different structure. The crawler interface normalizes them:

```go
// internal/crawler/crawler.go
type JobListing struct {
    ExternalID    string
    Title         string
    Description   string // HTML
    Location      string
    Department    string
    Team          string
    ApplyURL      string
    SourceURL     string
    EmploymentType string
}

type Crawler interface {
    // Name returns the ATS platform name
    Name() string
    // CrawlCompany fetches all job listings for a company
    CrawlCompany(ctx context.Context, company Company) ([]JobListing, error)
    // HealthCheck verifies the crawler can reach the ATS
    HealthCheck(ctx context.Context) error
}
```

### ATS-Specific Strategies

| ATS | Strategy | Why |
|---|---|---|
| **Greenhouse** | Hit `https://boards.greenhouse.io/{company}/jobs` — returns structured JSON via their embed API. **Easiest to crawl.** | Public JSON API, no auth needed, well-structured |
| **Lever** | Hit `https://jobs.lever.co/{company}` — HTML parsing with Go `goquery`. Also has a JSON endpoint at `/v0/postings/{company}?mode=json`. | Semi-structured, JSON endpoint available |
| **Ashby** | Hit `https://jobs.ashbyhq.com/{company}` — newer ATS, GraphQL-based. Need to reverse-engineer their API calls. | GraphQL API, need to capture network requests |
| **Generic** | Configurable CSS selector-based extraction for company career pages. | Fallback for companies not on major ATS platforms |

### Rate Limiting Strategy

```go
// Per-domain rate limiter using token bucket
type RateLimiter struct {
    limiters map[string]*rate.Limiter  // domain → limiter
    mu       sync.RWMutex
}

// Default: 1 request/second per domain, burst of 3
func (rl *RateLimiter) Wait(ctx context.Context, domain string) error
```

### Crawl Pipeline

```
Company List → Scheduler (cron) → Crawler → Normalizer → Deduplicator → AI Extractor → Indexer
                                                                              ↓
                                                              PostgreSQL + Elasticsearch
                                                                              ↓
                                                                      Alert Evaluator
                                                                              ↓
                                                                   WebSocket Notifier
```

---

## Redis Usage

| Use Case | Key Pattern | TTL | Why |
|---|---|---|---|
| **Crawl dedup** | `crawl:seen:{content_hash}` | 24h | Skip re-processing identical job descriptions |
| **API rate limiting** | `ratelimit:{ip}:{endpoint}` | 1 min | Sliding window rate limiter |
| **Job cache** | `job:{id}` | 1h | Avoid DB hits for frequently accessed jobs |
| **Search cache** | `search:{query_hash}` | 15 min | Cache popular search results |
| **Trend cache** | `trends:{skill}:{date}` | 6h | Expensive aggregation queries |
| **Crawler health** | `crawler:health:{ats_name}` | 5 min | Circuit breaker state for each ATS |

---

## Elasticsearch Index Design

```json
{
  "mappings": {
    "properties": {
      "job_id":           { "type": "keyword" },
      "title":            { "type": "text", "analyzer": "english", "fields": { "keyword": { "type": "keyword" } } },
      "normalized_title": { "type": "text", "analyzer": "english" },
      "description":      { "type": "text", "analyzer": "english" },
      "company_name":     { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
      "location":         { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
      "location_type":    { "type": "keyword" },
      "seniority_level":  { "type": "keyword" },
      "employment_type":  { "type": "keyword" },
      "department":       { "type": "keyword" },
      "skills_required":  { "type": "keyword" },
      "skills_preferred": { "type": "keyword" },
      "salary_min":       { "type": "integer" },
      "salary_max":       { "type": "integer" },
      "experience_years_min": { "type": "integer" },
      "first_seen_at":    { "type": "date" },
      "last_seen_at":     { "type": "date" },
      "is_active":        { "type": "boolean" },
      "ai_summary":       { "type": "text", "analyzer": "english" },
      "embedding":        { "type": "dense_vector", "dims": 1536, "index": true, "similarity": "cosine" }
    }
  },
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "english": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "english_stop", "english_stemmer"]
        }
      }
    }
  }
}
```

### Search Strategy

1. **Keyword search**: Elasticsearch full-text on title + description + skills
2. **Semantic search**: Dense vector similarity using job embeddings (cosine)
3. **Hybrid search**: Combine keyword BM25 score + vector similarity with reciprocal rank fusion
4. **Faceted filtering**: ES aggregations for skills, locations, seniority, companies

---

## AI Pipeline Design

### Skill Extraction

```
Job Description (raw HTML)
    │
    ▼
Clean to plaintext (strip HTML)
    │
    ▼
Send to OpenAI/Gemini with structured output prompt:
    "Extract skills, seniority, experience years, education from this job posting.
     Return JSON: {skills_required: [...], skills_preferred: [...], 
                   seniority: '...', experience_years: {min, max}, ...}"
    │
    ▼
Validate against skills taxonomy (fuzzy match aliases)
    │
    ▼
Store structured data in PostgreSQL + update Elasticsearch
```

### Resume-Job Matching

```
User Resume (plaintext)
    │
    ▼
Generate embedding (OpenAI text-embedding-3-small)
    │
    ▼
Cosine similarity against all job embeddings (pgvector / ES kNN)
    │
    ▼
Re-rank top 50 with detailed AI scoring prompt:
    "Score this resume against this job 0-100. 
     Explain: skill_match, experience_match, missing_skills, strengths"
    │
    ▼
Return ranked results with explanations
```

### Cost Control

| Operation | Model | Est. Cost |
|---|---|---|
| Skill extraction | GPT-4o-mini / Gemini Flash | ~$0.001/job |
| Embedding | text-embedding-3-small | ~$0.0001/job |
| Resume matching re-rank | GPT-4o-mini | ~$0.005/match |
| Daily budget (5K jobs) | — | ~$5-8/day |

> [!TIP]
> Use **Gemini Flash** for skill extraction (free tier: 1500 req/day) to keep costs near zero during development.

---

## API Specification

### Jobs API

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/jobs` | List jobs with pagination + filters |
| `GET` | `/api/v1/jobs/:id` | Get job details |
| `GET` | `/api/v1/jobs/search` | Full-text + semantic search |
| `GET` | `/api/v1/jobs/stats` | Quick stats (total active, by seniority, etc.) |

### Match API

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/match/resume` | Upload resume, get matched jobs |
| `GET` | `/api/v1/match/recommendations` | Personalized job recommendations |

### Trends API

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/trends/skills` | Skill demand over time |
| `GET` | `/api/v1/trends/companies` | Hiring activity by company |
| `GET` | `/api/v1/trends/salaries` | Salary ranges by skill/role |

### Alerts API

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/alerts` | Create alert rule |
| `GET` | `/api/v1/alerts` | List user's alerts |
| `DELETE` | `/api/v1/alerts/:id` | Delete alert |
| `WS` | `/api/v1/ws/alerts` | WebSocket for real-time notifications |

### User API

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/users/profile` | Create/update profile |
| `GET` | `/api/v1/users/profile` | Get profile |

---

## Deployment Architecture (Docker Compose)

```yaml
services:
  app:
    build: .
    ports: ["8080:8080"]
    depends_on: [postgres, redis, elasticsearch]
    environment:
      - DATABASE_URL=postgres://jobintel:password@postgres:5432/jobintel?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - ELASTICSEARCH_URL=http://elasticsearch:9200
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - GEMINI_API_KEY=${GEMINI_API_KEY}

  postgres:
    image: pgvector/pgvector:pg16
    volumes: ["pgdata:/var/lib/postgresql/data"]
    environment:
      POSTGRES_DB: jobintel
      POSTGRES_USER: jobintel
      POSTGRES_PASSWORD: password

  redis:
    image: redis:7-alpine
    volumes: ["redisdata:/data"]

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.13.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    volumes: ["esdata:/usr/share/elasticsearch/data"]
```

---

## Monitoring Strategy

| Layer | Tool | What to Monitor |
|---|---|---|
| **Application** | Go `expvar` + Prometheus metrics | Request latency, error rates, active crawls, AI API calls |
| **Crawler health** | Custom health check endpoint | Per-ATS success rate, last crawl time, failure count |
| **Infrastructure** | Docker healthchecks | Container status, resource usage |
| **Logging** | Structured JSON logs (`slog`) | All operations with correlation IDs |

---

## Security Design

| Concern | Solution |
|---|---|
| **API auth** | JWT tokens (simple, stateless) |
| **Rate limiting** | Redis sliding window (100 req/min for search, 10 req/min for AI features) |
| **Input sanitization** | bluemonday for HTML, pgx parameterized queries (no SQL injection) |
| **Secrets** | Environment variables, never committed. `.env.example` with placeholders |
| **Crawl ethics** | Respect `robots.txt`, 1 req/sec per domain, identify via User-Agent |

---

## Failure Handling

| Failure | Strategy |
|---|---|
| **ATS returns 429 (rate limited)** | Exponential backoff with jitter. Circuit breaker opens after 5 consecutive failures. |
| **ATS HTML structure changes** | Content validation: if extracted fields < threshold, flag for review. Don't save garbage data. |
| **AI API timeout/error** | Retry 3x with backoff. Queue for batch retry. Job is saved without AI fields initially, enriched later. |
| **Elasticsearch down** | PostgreSQL full-text search fallback. ES is eventually consistent — app works without it. |
| **Redis down** | App works without cache, just slower. No Redis dependency for core path. |
| **Duplicate jobs** | Content hash (SHA256 of title+description+company). Upsert on conflict. |

---

## Scalability Considerations

| Dimension | Current Scale | Strategy to Scale |
|---|---|---|
| **Jobs/day** | ~5,000 | Go goroutines handle this trivially |
| **Concurrent users** | ~100 | Single Go server handles 10K+ concurrent connections |
| **Search latency** | <100ms target | Elasticsearch handles this at single-node scale |
| **Storage** | ~1M jobs/year | PostgreSQL handles billions of rows. Partition by `first_seen_at` if needed. |
| **AI costs** | ~$5/day | Use Gemini Flash free tier for dev. Batch embeddings. Cache AI results. |
| **When to split** | 50K+ users, 500K+ daily events | Extract crawler as separate service first (it's the most independent component) |

---

## User Review Required

> [!IMPORTANT]
> **Language choice**: Your existing project is Java/Spring Boot. This plan proposes rebuilding in **Go**. This means your existing Java code won't be reused directly, but the architectural patterns (concurrent crawling, SSE streaming, session management) carry over. **Do you want to proceed with Go, or should we build this in Java/Spring Boot instead?** Building in Java is also perfectly valid and would let you reuse your existing crawler engine.

> [!WARNING]
> **Existing code**: If you choose Go, the existing Java files in this folder should be moved or archived. We'll restructure the folder as a Go project. If you choose Java, we'll extend your current Spring Boot application.

> [!IMPORTANT]
> **AI API provider**: Do you have an OpenAI API key, a Google Gemini API key, or both? Gemini Flash has a generous free tier (1500 req/day) which is ideal for development.

---

## Open Questions

1. **Which companies do you want to crawl first?** I'd suggest starting with 3-5 well-known companies that use Greenhouse or Lever (e.g., Stripe, Figma, Notion, Airbnb, Coinbase). We can add more later.

2. **Do you have Docker Desktop installed?** We need it for PostgreSQL, Redis, and Elasticsearch locally.

3. **Frontend priority**: How important is a polished React dashboard vs. a functional API-only backend? We could ship a clean API with Swagger docs first, then build the frontend.

---

## Development Roadmap

### Phase 1: Foundation (Days 1-3)
- [x] Project structure, Go modules, Docker Compose
- [ ] PostgreSQL schema + migrations
- [ ] Config management (env vars)
- [ ] Basic Chi router with health endpoint
- [ ] Structured logging with `slog`

### Phase 2: Crawlers (Days 4-7)
- [ ] Crawler interface + Greenhouse crawler
- [ ] Lever crawler
- [ ] Ashby crawler
- [ ] Rate limiter + deduplication
- [ ] Crawl scheduling (cron)
- [ ] Crawl health monitoring

### Phase 3: AI Pipeline (Days 8-11)
- [ ] AI provider abstraction (OpenAI/Gemini)
- [ ] Skill extraction from job descriptions
- [ ] Embedding generation
- [ ] Processing pipeline (crawl → normalize → extract → store)
- [ ] Skills taxonomy seeding

### Phase 4: Search + API (Days 12-15)
- [ ] Elasticsearch indexing
- [ ] Full-text search endpoint
- [ ] Semantic search (vector similarity)
- [ ] Hybrid search with re-ranking
- [ ] Job listing API with filters/pagination
- [ ] Redis caching layer

### Phase 5: Intelligence Features (Days 16-20)
- [ ] User profiles
- [ ] Resume parsing + embedding
- [ ] Resume-job matching
- [ ] Personalized recommendations
- [ ] Real-time alerts (WebSocket)

### Phase 6: Analytics + Frontend (Days 21-28)
- [ ] Trend analytics (skill demand over time)
- [ ] Salary insights
- [ ] Trend snapshot materialization (daily cron)
- [ ] React dashboard — search page
- [ ] React dashboard — job details
- [ ] React dashboard — trends visualization
- [ ] React dashboard — resume matching UI
- [ ] React dashboard — alert management

### Phase 7: Polish (Days 29-32)
- [ ] Error handling hardening
- [ ] API documentation (Swagger/OpenAPI)
- [ ] Monitoring endpoints
- [ ] README with architecture diagram
- [ ] Demo data seeding script
- [ ] Integration tests

---

## Interview Questions This Project Answers

| Question | Your Answer |
|---|---|
| "Design a web crawler" | Multi-ATS crawler with per-domain rate limiting, dedup via content hashing, circuit breakers |
| "How would you handle data consistency?" | Content hash dedup, upsert on conflict, idempotent processing pipeline |
| "SQL vs NoSQL?" | PostgreSQL for structured data + JSONB for flexible fields + pgvector for embeddings. ES for search. Right tool for each job. |
| "How do you handle API failures?" | Retry with exponential backoff + jitter, circuit breaker pattern, graceful degradation |
| "Design a notification system" | Alert rules → evaluated on new job insert → fan-out via WebSocket with Redis pub/sub |
| "Microservices vs Monolith?" | "I built a modular monolith because the scale didn't justify microservices. Here's where I'd split: crawler first (most independent), then AI pipeline (different scaling profile)." |
| "How would you scale this?" | Horizontal: add read replicas for PG, ES cluster expansion. Vertical: Go handles 10K concurrent connections on a single core. Partition jobs by date. |
| "Rate limiting design" | Redis sliding window for API. Token bucket for crawl. Different limits per endpoint based on cost. |
| "How do you test this?" | Unit tests for extractors/normalizers, integration tests with testcontainers, crawl tests with recorded HTTP responses |
| "Caching strategy?" | Multi-layer: Redis for hot data (jobs, search results), HTTP cache headers for API, ES for search (inherently cached). Cache invalidation on new crawl. |
