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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	if migrate {
		return store.Migrate(ctx, pool, cfg.MigrationsDir)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fail := make(chan error, 1)
	go func() { fail <- srv.Serve(listener) }()
	obs.NewLogger().Info("server listening", slog.String("address", listener.Addr().String()))
	select {
	case err = <-fail:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
	}
	return nil
}
