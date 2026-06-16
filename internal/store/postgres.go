package store

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// PostgresStore wraps a pgx connection pool and provides data access methods.
type PostgresStore struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewPostgresStore creates a new PostgreSQL store and runs migrations.
func NewPostgresStore(ctx context.Context, databaseURL string, logger *slog.Logger) (*PostgresStore, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	// Connection pool settings
	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	store := &PostgresStore{
		pool:   pool,
		logger: logger,
	}

	// Run migrations
	if err := store.runMigrations(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	logger.Info("PostgreSQL connected",
		"max_conns", config.MaxConns,
		"min_conns", config.MinConns,
	)

	return store, nil
}

// runMigrations applies embedded SQL migration files in order.
func (s *PostgresStore) runMigrations(ctx context.Context) error {
	// Create migrations tracking table
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Read all migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version := entry.Name()

		// Check if already applied
		var count int
		err := s.pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("checking migration %s: %w", version, err)
		}
		if count > 0 {
			continue
		}

		// Read and execute migration
		content, err := migrationsFS.ReadFile("migrations/" + version)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", version, err)
		}

		s.logger.Info("Applying migration", "version", version)

		_, err = s.pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("applying migration %s: %w", version, err)
		}

		// Record migration
		_, err = s.pool.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)", version,
		)
		if err != nil {
			return fmt.Errorf("recording migration %s: %w", version, err)
		}

		s.logger.Info("Migration applied", "version", version)
	}

	return nil
}

// Pool returns the underlying connection pool for direct access when needed.
func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}

// Close closes the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// HealthCheck verifies the database connection is alive.
func (s *PostgresStore) HealthCheck(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
