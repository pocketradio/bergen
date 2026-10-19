// Package obs configures safe structured logging.
package obs

import (
	"log/slog"
	"os"
)

// NewLogger creates a JSON logger. Callers must log identifiers, never tokens or URLs.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
