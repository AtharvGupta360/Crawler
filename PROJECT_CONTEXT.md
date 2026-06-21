# JobCrawl — Project Context

> **Purpose**: Feed this file to any AI coding session so it has instant context.
> **Last updated**: 2026-06-19

## What Is This?

A Go backend that crawls job postings from ATS platforms (Greenhouse, Lever, Ashby), stores them in PostgreSQL, indexes them in Elasticsearch, and exposes a REST API for search and management. Uses Kafka for event-driven processing.

## Tech Stack

| Layer | Tech | Notes |
|-------|------|-------|
| Language | Go 1.26 | `go.mod` module: `github.com/AtharvGupta360/JobCrawl` |
| HTTP Router | chi/v5 | REST API on `:8080` |
| Database | PostgreSQL 16 + pgvector | pgx/v5 driver, embedded migrations |
| Cache/Dedup | Redis 7 | go-redis/v9 |
| Search | Elasticsearch 8 | go-elasticsearch/v8, optional (graceful degradation) |
| Events | Kafka (KRaft) | segmentio/kafka-go, optional (falls back to sync) |
| Config | Environment vars | `.env` + `godotenv`, `internal/config/config.go` |

## Directory Structure

```
cmd/server/main.go          — Entrypoint, wires everything
internal/
  api/
    router.go               — Chi router, Server struct, ServerConfig
    handlers.go             — REST handlers (jobs, companies, crawl, search)
    middleware.go           — Structured logging, rate limiting middleware
  config/
    config.go              — Env-based config with defaults
  crawler/
    crawler.go             — Crawler interface, RawJobListing, helpers
    greenhouse.go          — Greenhouse ATS crawler
    lever.go               — Lever ATS crawler
    ashby.go               — Ashby ATS crawler
    scheduler.go           — Periodic crawl scheduler, EventPublisher interface
    ratelimiter.go         — Per-domain rate limiter + circuit breaker
    seed.go                — Default companies seeder
  kafka/
    topics.go              — Topic + consumer group constants
    events.go              — Event types (CrawlEvent, ProcessedEvent, AlertEvent)
    producer.go            — Kafka writer for jobs.raw + jobs.processed
    processor.go           — Consumer: jobs.raw → dedup → PG upsert → jobs.processed
    indexer.go             — Consumer: jobs.processed → bulk ES index
    adapter.go             — PublisherAdapter (bridges crawler.EventPublisher → Producer)
  models/
    job.go                 — Job, SkillEntry, constants
    company.go             — Company, ATS constants
    user.go                — User, Alert, Skill, CrawlRun, TrendSnapshot
  store/
    postgres.go            — PG connection pool, embedded migration runner
    jobs.go                — Company/Job/CrawlRun CRUD, JobFilter, JobStats
    redis.go               — Dedup, rate limiting, caching, health tracking
    elasticsearch.go       — ES client, index mappings, search with facets/highlights
    migrations/
      001_initial.sql      — All tables (companies, jobs, skills, users, alerts, trends)
docker-compose.yml         — Postgres, Redis, ES, Kafka (KRaft), Kafka UI
Makefile                   — dev, build, infra, kafka-topics targets
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check (PG, Redis, ES) |
| GET | `/api/v1/jobs` | List jobs (pagination, filters) |
| GET | `/api/v1/jobs/stats` | Aggregate job stats |
| GET | `/api/v1/jobs/{jobID}` | Get single job |
| GET | `/api/v1/search` | Full-text search (ES-backed) |
| GET | `/api/v1/companies` | List companies |
| POST | `/api/v1/companies` | Add company |
| POST | `/api/v1/crawl/trigger` | Trigger crawl for all companies |
| POST | `/api/v1/crawl/trigger/{slug}` | Trigger crawl for one company |
| GET | `/api/v1/crawl/runs` | List recent crawl runs |
| GET | `/api/v1/crawl/health` | Crawler health status |

## Key Patterns

- **EventPublisher interface** in `crawler/scheduler.go` — avoids circular imports between crawler ↔ kafka
- **Graceful degradation** — Kafka and ES are optional; system falls back to synchronous processing
- **Content-hash dedup** — SHA256 of title+description+location+department, cached in Redis 24h
- **Circuit breaker** — Per-domain failure tracking with configurable threshold and cooldown
- **Embedded migrations** — SQL files in `store/migrations/` via `//go:embed`

## Completed Phases

| Phase | What |
|-------|------|
| 1 ✅ | Go foundation, PostgreSQL, Redis, config, models, API, migrations |
| 2 ✅ | Greenhouse/Lever/Ashby crawlers, rate limiter, circuit breaker, scheduler |
| 3 ✅ | Kafka event pipeline, Elasticsearch search, scheduler refactor |

## What's Next (Phase 4 candidates)

- AI enrichment (skills extraction, seniority detection, salary parsing via OpenAI/Gemini)
- User authentication (JWT)
- Alert system (Kafka consumer matching jobs to user alert rules → WebSocket/email notifications)
- Trend analytics (daily snapshots of skill demand, stored in trend_snapshots table)
