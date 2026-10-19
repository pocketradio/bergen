package store

import (
	"context"
	"os"
	"path/filepath"
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	dir := filepath.Join("..", "..", "migrations")
	for range 2 {
		if err := Migrate(ctx, pool, dir); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations WHERE version='0001'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration applied %d times", count)
	}
}
