package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore wraps a Redis client for caching, rate limiting, and dedup.
type RedisStore struct {
	client *redis.Client
	logger *slog.Logger
}

// NewRedisStore creates a new Redis store.
func NewRedisStore(ctx context.Context, redisURL string, logger *slog.Logger) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	logger.Info("Redis connected", "addr", opts.Addr)

	return &RedisStore{client: client, logger: logger}, nil
}

// Close closes the Redis connection.
func (r *RedisStore) Close() error {
	return r.client.Close()
}

// HealthCheck verifies the Redis connection.
func (r *RedisStore) HealthCheck(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// ─────────────────────────────────────────────
// Crawl Deduplication
// ─────────────────────────────────────────────

// IsContentSeen checks if a content hash has been seen recently.
// Returns true if already seen (skip processing), false if new.
func (r *RedisStore) IsContentSeen(ctx context.Context, contentHash string) (bool, error) {
	key := fmt.Sprintf("crawl:seen:%s", contentHash)
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// MarkContentSeen marks a content hash as seen with a 24h TTL.
func (r *RedisStore) MarkContentSeen(ctx context.Context, contentHash string) error {
	key := fmt.Sprintf("crawl:seen:%s", contentHash)
	return r.client.Set(ctx, key, "1", 24*time.Hour).Err()
}

// ─────────────────────────────────────────────
// API Rate Limiting (sliding window)
// ─────────────────────────────────────────────

// CheckRateLimit returns true if the request is allowed under the rate limit.
// Uses a sliding window counter.
func (r *RedisStore) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	rlKey := fmt.Sprintf("ratelimit:%s", key)

	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, rlKey)
	pipe.Expire(ctx, rlKey, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	return incr.Val() <= int64(limit), nil
}

// ─────────────────────────────────────────────
// Generic Cache
// ─────────────────────────────────────────────

// CacheGet retrieves a cached value and unmarshals it into dest.
func (r *RedisStore) CacheGet(ctx context.Context, key string, dest any) error {
	val, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(val, dest)
}

// CacheSet stores a value with the given TTL.
func (r *RedisStore) CacheSet(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, ttl).Err()
}

// CacheDelete removes a cached value.
func (r *RedisStore) CacheDelete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// ─────────────────────────────────────────────
// Crawler Health (Circuit Breaker State)
// ─────────────────────────────────────────────

// SetCrawlerHealth stores the health status for an ATS crawler.
func (r *RedisStore) SetCrawlerHealth(ctx context.Context, atsName string, healthy bool, msg string) error {
	key := fmt.Sprintf("crawler:health:%s", atsName)
	data := map[string]any{
		"healthy":    healthy,
		"message":    msg,
		"checked_at": time.Now().Unix(),
	}
	jsonData, _ := json.Marshal(data)
	return r.client.Set(ctx, key, jsonData, 5*time.Minute).Err()
}

// GetCrawlerHealth retrieves the health status for an ATS crawler.
func (r *RedisStore) GetCrawlerHealth(ctx context.Context, atsName string) (bool, string, error) {
	key := fmt.Sprintf("crawler:health:%s", atsName)
	val, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return true, "no health data", nil // assume healthy if no data
	}
	if err != nil {
		return false, "", err
	}

	var data map[string]any
	if err := json.Unmarshal(val, &data); err != nil {
		return false, "", err
	}

	healthy, _ := data["healthy"].(bool)
	msg, _ := data["message"].(string)
	return healthy, msg, nil
}

// ErrCacheMiss is returned when a cache lookup finds no value.
var ErrCacheMiss = fmt.Errorf("cache miss")

// ─────────────────────────────────────────────
// Raw JSON helpers (used by alert evaluator cache)
// ─────────────────────────────────────────────

// GetJSON returns the raw JSON bytes stored at key, or nil+nil on cache miss.
func (r *RedisStore) GetJSON(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

// SetJSON stores raw JSON bytes with a TTL. Best-effort — errors are logged but not fatal.
func (r *RedisStore) SetJSON(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, data, ttl).Err()
}

// DeleteCache removes a key from the cache (used to invalidate alert cache on writes).
func (r *RedisStore) DeleteCache(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
