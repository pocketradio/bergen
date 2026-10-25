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
	// get our NNNN.sql migration files from dir
	files, err := filepath.Glob(filepath.Join(dir, "[0-9][0-9][0-9][0-9]_*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	// no migrations found
	if len(files) == 0 {
		return fmt.Errorf("no migrations in %s", dir)
	}

	// get a connection from the pool
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	// Prevent multiple application instances from running migrations at once by locking session
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(312089551)"); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}

	// unlock the lock after we're done
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock(312089551)")
	}()

	// Keep the migration ledger in public so creating the bergen schema cannot shadow it through search_path.
	if _, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}

	// iterate over the migration files
	for _, file := range files {
		version, _, _ := strings.Cut(filepath.Base(file), "_")

		var applied bool

		// If migration exists in the database, set applied = true
		if err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
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

		// Execute the migration
		// Rollback if error
		if _, err = tx.Exec(ctx, string(body), pgx.QueryExecModeSimpleProtocol); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", version, err)
		}

		// Insert the migration into the canonical public ledger.
		// Rollback if error
		if _, err = tx.Exec(ctx, `INSERT INTO public.schema_migrations(version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		// Commit our changes
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
	}

	return nil
}
