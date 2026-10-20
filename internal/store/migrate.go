// Package store manages database changes over time.
// It runs new SQL migration files in order and remembers which ones are complete.
// Concurrent migration attempts are prevented from running at the same time.

package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate applies pending database migrations in order.
// It records completed migrations and prevents concurrent migration runs.
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "[0-9][0-9][0-9][0-9]_*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no migrations in %s", dir)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	// Prevent multiple application instances from running migrations at once.
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(312089551)"); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}

	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock(312089551)")
	}()

	// Track completed migrations so rerunning the command is safe.
	if _, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}

	for _, file := range files {
		version, _, _ := strings.Cut(filepath.Base(file), "_")

		var applied bool

		if err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}

		// Skip migrations that are already recorded as applied.
		if applied {
			continue
		}

		body, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		// Apply the migration and record its version in one transaction.
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", version, err)
		}

		if _, err = tx.Exec(ctx, string(body), pgx.QueryExecModeSimpleProtocol); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", version, err)
		}

		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
	}

	return nil
}
