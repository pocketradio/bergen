// Package config loads server infrastructure configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds non-secret server settings and credentials loaded at startup.
type Config struct {
	ListenAddr    string
	DatabaseURL   string
	S3Endpoint    string
	S3AccessKey   string
	S3SecretKey   string
	S3Bucket      string
	S3Secure      bool
	MigrationsDir string
}

// Load validates environment configuration; secrets are never included in errors.
func Load() (Config, error) {
	// Check if connection is secure
	secure, err := strconv.ParseBool(value("BERGEN_S3_SECURE", "false"))

	if err != nil {
		return Config{}, fmt.Errorf("BERGEN_S3_SECURE: expected boolean: %w", err)
	}

	// load config from environment
	c := Config{
		ListenAddr:    value("BERGEN_LISTEN_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("BERGEN_DATABASE_URL"),
		S3Endpoint:    os.Getenv("BERGEN_S3_ENDPOINT"),
		S3AccessKey:   os.Getenv("BERGEN_S3_ACCESS_KEY"),
		S3SecretKey:   os.Getenv("BERGEN_S3_SECRET_KEY"),
		S3Bucket:      os.Getenv("BERGEN_S3_BUCKET"),
		S3Secure:      secure,
		MigrationsDir: value("BERGEN_MIGRATIONS_DIR", "migrations"),
	}

	// Loop over the required env vars and check that they're not empty
	for _, field := range []struct{ name, value string }{
		{"BERGEN_DATABASE_URL", c.DatabaseURL}, {"BERGEN_S3_ENDPOINT", c.S3Endpoint},
		{"BERGEN_S3_ACCESS_KEY", c.S3AccessKey}, {"BERGEN_S3_SECRET_KEY", c.S3SecretKey},
		{"BERGEN_S3_BUCKET", c.S3Bucket},
	} {
		if strings.TrimSpace(field.value) == "" {
			return Config{}, fmt.Errorf("%s is required", field.name)
		}
	}
	return c, nil
}

// value returns the environment value or a non-secret default.
func value(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
