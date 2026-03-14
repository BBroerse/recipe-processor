package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bbroerse/recipe-processor/internal/application"
	handler "github.com/bbroerse/recipe-processor/internal/infrastructure/http"
	"github.com/bbroerse/recipe-processor/internal/infrastructure/ollama"
	"github.com/bbroerse/recipe-processor/internal/infrastructure/postgres"
	"github.com/bbroerse/recipe-processor/internal/shared/config"
	"github.com/bbroerse/recipe-processor/internal/shared/eventbus"
)

func main() {
	// Bootstrap with info level, reconfigure after config loads
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Reconfigure logger with the configured level
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.Log.Level),
	})))

	slog.Info("starting recipe-processor", "env", cfg.Env, "port", cfg.Server.Port)

	// Database
	pool, err := postgres.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()
	slog.Info("connected to database")

	// Run migrations
	if err := postgres.Migrate(cfg.Database.DSN()); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	slog.Info("migrations complete")

	// Dependencies
	repo := postgres.NewRecipeRepository(pool)
	eventLogRepo := postgres.NewEventLogRepository(pool)
	llmClient := ollama.NewClient(cfg.Ollama.URL, cfg.Ollama.Model)
	bus := eventbus.New(100, eventLogRepo)

	// Application service
	service := application.NewRecipeService(repo, llmClient, bus)

	// Subscribe event handlers
	bus.Subscribe("recipe.submitted", service.HandleRecipeSubmitted)

	// Start event bus
	if err := bus.Start(ctx); err != nil {
		return fmt.Errorf("starting event bus: %w", err)
	}

	// HTTP server
	mux := http.NewServeMux()
	h := handler.NewHandler(service)
	h.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      handler.RecoveryMiddleware(handler.SecurityHeadersMiddleware(handler.LoggingMiddleware(mux))),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	if err := bus.Stop(); err != nil {
		return fmt.Errorf("event bus shutdown: %w", err)
	}

	slog.Info("shutdown complete")
	return nil
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
