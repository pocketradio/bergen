package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrate exercises real PostgreSQL and verifies repeat migration is safe.
func TestMigrate(t *testing.T) {
	url := os.Getenv("BERGEN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set BERGEN_TEST_DATABASE_URL for PostgreSQL integration")
	}

	// Acquire a context
	// timeout the context after 15s to prevent hanging indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// Run migrations repeatedly after the application schema exists to catch ledger shadowing.
	dir := filepath.Join("..", "..", "migrations")
	for range 3 {
		if err := Migrate(ctx, pool, dir); err != nil {
			t.Fatal(err)
		}
	}

	// Get all the migration files in the directory
	files, err := filepath.Glob(filepath.Join(dir, "[0-9][0-9][0-9][0-9]_*.sql"))
	if err != nil {
		t.Fatal(err)
	}

	// Iterate over all migration files
	for _, file := range files {
		version, _, _ := strings.Cut(filepath.Base(file), "_")
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations WHERE version=$1", version).Scan(&count); err != nil {
			t.Fatal(err)
		}
		// Throw an error if any migration was applied more than once
		if count != 1 {
			t.Fatalf("migration %s applied %d times, want 1", version, count)
		}
	}
}
