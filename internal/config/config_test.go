package config

import (
	"strings"
	"testing"
)

// TestLoad validates required fields and avoids exposing secrets in errors.
func TestLoad(t *testing.T) {
	for _, kv := range []struct{ key, value string }{
		{"BERGEN_DATABASE_URL", "postgres://user:secret@localhost/db"},
		{"BERGEN_S3_ENDPOINT", "localhost:9000"},
		{"BERGEN_S3_ACCESS_KEY", "test"}, {"BERGEN_S3_SECRET_KEY", "secret"},
		{"BERGEN_S3_BUCKET", "test"}, {"BERGEN_S3_SECURE", "false"},
	} {
		t.Setenv(kv.key, kv.value)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != ":8080" {
		t.Fatalf("unexpected listen address: %s", c.ListenAddr)
	}
	t.Setenv("BERGEN_S3_SECURE", "invalid")
	_, err = Load()
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unexpected error: %v", err)
	}
}
