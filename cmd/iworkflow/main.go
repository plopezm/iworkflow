// Command iworkflow starts the iworkflow web server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iworkflow/internal/app"
	"iworkflow/internal/core/config"
	"iworkflow/internal/core/db"
)

const (
	defaultAddr     = "127.0.0.1:3000"
	staticDir       = "web/static"
	shutdownTimeout = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("iworkflow stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	handle, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = handle.Close() }()
	if err := db.Migrate(ctx, handle); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	addr := defaultAddr
	if value := os.Getenv("LEPTOS_SITE_ADDR"); value != "" {
		addr = value
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           app.New(handle, cfg, logger, staticDir).Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	logger.Info("listening", "addr", addr)

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("shutdown server: %w", err)
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}
}
