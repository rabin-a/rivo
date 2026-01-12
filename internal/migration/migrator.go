package migration

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrator handles database migrations.
type Migrator struct {
	pool *pgxpool.Pool
}

// New creates a new Migrator.
func New(pool *pgxpool.Pool) *Migrator {
	return &Migrator{pool: pool}
}

// Migrate runs all pending migrations.
func (m *Migrator) Migrate(ctx context.Context) error {
	// Acquire advisory lock to prevent concurrent migrations
	var locked bool
	err := m.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock(42424242)").Scan(&locked)
	if err != nil {
		return fmt.Errorf("acquiring advisory lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("another migration is in progress")
	}
	defer m.pool.Exec(ctx, "SELECT pg_advisory_unlock(42424242)")

	// Ensure migrations table exists
	_, err = m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS rivo_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := m.getAppliedVersions(ctx)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	// Get available migrations
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	// Apply pending migrations
	for _, mig := range migrations {
		if applied[mig.version] {
			continue
		}

		if err := m.applyMigration(ctx, mig); err != nil {
			return fmt.Errorf("applying migration %d: %w", mig.version, err)
		}
	}

	return nil
}

type migration struct {
	version int
	upSQL   string
}

func (m *Migrator) getAppliedVersions(ctx context.Context) (map[int]bool, error) {
	rows, err := m.pool.Query(ctx, "SELECT version FROM rivo_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func (m *Migrator) loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Parse version from filename (e.g., "001_create_jobs.sql")
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}

		upSQL := extractUpSQL(string(content))
		if upSQL == "" {
			continue
		}

		migrations = append(migrations, migration{
			version: version,
			upSQL:   upSQL,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

func extractUpSQL(content string) string {
	// Find the content between "-- +migrate Up" and "-- +migrate Down"
	upMarker := "-- +migrate Up"
	downMarker := "-- +migrate Down"

	upIdx := strings.Index(content, upMarker)
	if upIdx == -1 {
		return content // No markers, assume entire file is "up" migration
	}

	content = content[upIdx+len(upMarker):]

	downIdx := strings.Index(content, downMarker)
	if downIdx != -1 {
		content = content[:downIdx]
	}

	return strings.TrimSpace(content)
}

func (m *Migrator) applyMigration(ctx context.Context, mig migration) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Run migration SQL
	_, err = tx.Exec(ctx, mig.upSQL)
	if err != nil {
		return fmt.Errorf("executing SQL: %w", err)
	}

	// Record migration
	_, err = tx.Exec(ctx, "INSERT INTO rivo_migrations (version) VALUES ($1)", mig.version)
	if err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	return tx.Commit(ctx)
}
