// Package main starts the Bergen server and provides a migration command.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pocketradio/bergen/internal/config"
	"github.com/pocketradio/bergen/internal/obs"
	"github.com/pocketradio/bergen/internal/store"
)

// main runs the server or a one-shot migration, reporting startup errors without secrets.
func main() {
	migrate := flag.Bool("migrate", false, "apply database migrations and exit")
	flag.Parse()
	if err := run(*migrate); err != nil {
		obs.NewLogger().Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// run loads config, connects to PostgreSQL, and starts a cancellable HTTP server.
func run(migrate bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Setup a listener to gracefully shutdown from OS signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create a pg pool
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure database: %w", err)
	}
	defer pool.Close()

	// Test pinging the pool
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	// The server was started with the migrate flag
	if migrate {
		return store.Migrate(ctx, pool, cfg.MigrationsDir)
	}

	// Instantiate the http server
	mux := http.NewServeMux()

	// Health check for the server
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fail := make(chan error, 1)

	// Start the server in a goroutine
	// so that we don't block the main thread
	go func() { fail <- srv.Serve(listener) }()
	obs.NewLogger().Info("server listening", slog.String("address", listener.Addr().String()))

	select {
	// Encountered a server error
	case err = <-fail:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
	// Encountered a signal to terminate the process
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
	}
	return nil
}
