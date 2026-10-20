package config

import (
	"strings"
	"testing"
)

// TestLoad validates required fields and avoids exposing secrets in errors.
func TestLoad(t *testing.T) {
	// Load hardcoded variables into env for tests
	for _, kv := range []struct{ key, value string }{
		{"BERGEN_DATABASE_URL", "postgres://user:secret@localhost/db"},
		{"BERGEN_S3_ENDPOINT", "localhost:9000"},
		{"BERGEN_S3_ACCESS_KEY", "test"}, {"BERGEN_S3_SECRET_KEY", "secret"},
		{"BERGEN_S3_BUCKET", "test"}, {"BERGEN_S3_SECURE", "false"},
	} {
		t.Setenv(kv.key, kv.value)
	}

	// Load the config
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	// Check our listening address
	if c.ListenAddr != ":8080" {
		t.Fatalf("unexpected listen address: %s", c.ListenAddr)
	}

	// Check loading with an invalid secure flag
	// Should throw an error
	// The error should also not expose credentials
	t.Setenv("BERGEN_S3_SECURE", "invalid")
	_, err = Load()
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unexpected error: %v", err)
	}
}
