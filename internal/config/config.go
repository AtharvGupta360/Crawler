package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	AppEnv  string // "development", "production"
	AppPort int

	// PostgreSQL
	DatabaseURL string

	// Redis
	RedisURL string

	// Elasticsearch
	ElasticsearchURL string

	// Kafka
	KafkaBrokers []string

	// AI Providers
	OpenAIAPIKey string
	GeminiAPIKey string

	// Crawl Settings
	CrawlRateLimitPerSecond float64
	CrawlMaxConcurrent      int
	CrawlUserAgent          string

	// JWT
	JWTSecret string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:                  getEnv("APP_ENV", "development"),
		AppPort:                 getEnvInt("APP_PORT", 8080),
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://jobcrawl:jobcrawl_dev_pass@localhost:5432/jobcrawl?sslmode=disable"),
		RedisURL:                getEnv("REDIS_URL", "redis://localhost:6379"),
		ElasticsearchURL:        getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),
		KafkaBrokers:            strings.Split(getEnv("KAFKA_BROKERS", ""), ","),
		OpenAIAPIKey:            getEnv("OPENAI_API_KEY", ""),
		GeminiAPIKey:            getEnv("GEMINI_API_KEY", ""),
		CrawlRateLimitPerSecond: getEnvFloat("CRAWL_RATE_LIMIT_PER_SECOND", 1.0),
		CrawlMaxConcurrent:     getEnvInt("CRAWL_MAX_CONCURRENT", 5),
		CrawlUserAgent:          getEnv("CRAWL_USER_AGENT", "JobCrawl/1.0"),
		JWTSecret:               getEnv("JWT_SECRET", "dev-secret-change-in-production"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.AppPort < 1 || c.AppPort > 65535 {
		return fmt.Errorf("APP_PORT must be between 1 and 65535")
	}
	return nil
}

// IsDev returns true if running in development mode.
func (c *Config) IsDev() bool {
	return c.AppEnv == "development"
}

// HasOpenAI returns true if an OpenAI API key is configured.
func (c *Config) HasOpenAI() bool {
	return c.OpenAIAPIKey != ""
}

// HasGemini returns true if a Gemini API key is configured.
func (c *Config) HasGemini() bool {
	return c.GeminiAPIKey != ""
}

// ─────────────────────────────────────────────
// Helper functions
// ─────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
