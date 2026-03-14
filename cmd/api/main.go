// Package main is the entrypoint for the recipe-processor API server.
//
//	@title						Recipe Processor API
//	@version					1.0
//	@description				Async recipe processing API that uses LLM to extract structured data from raw recipe text.
//	@host						localhost:8080
//	@BasePath					/
//	@accept						json
//	@produce					json
//	@securityDefinitions.apikey	ApiKeyAuth
//	@in							header
//	@name						X-API-Key
//	@description				API key for authentication (optional in development mode)
//
//go:generate swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
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
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"golang.org/x/time/rate"

	// Import generated swagger docs so the spec is registered at init time.
	_ "github.com/bbroerse/recipe-processor/docs"
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

	if cfg.APIKey == "" {
		slog.Warn("API_KEY not set — authentication disabled (development mode)")
	} else {
		slog.Info("API key authentication enabled")
	}

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

	// Swagger docs — only available in development mode
	if cfg.Env == "development" {
		mux.Handle("GET /docs/", swaggerDocsHandler())
		slog.Info("swagger UI enabled at /docs/")
	}

	// Rate limiter — per-IP, configurable via RATE_LIMIT / RATE_LIMIT_BURST env vars
	var rateLimiter *handler.RateLimiter
	if cfg.Server.RateLimit > 0 {
		rateLimiter = handler.NewRateLimiter(
			rate.Limit(cfg.Server.RateLimit),
			cfg.Server.RateLimitBurst,
		)
		slog.Info("rate limiting enabled",
			"rate", cfg.Server.RateLimit,
			"burst", cfg.Server.RateLimitBurst,
		)
	} else {
		slog.Warn("rate limiting disabled (RATE_LIMIT=0)")
	}

	// Build middleware chain (outermost \u2192 innermost):
	// RequestID \u2192 Recovery \u2192 SecurityHeaders \u2192 Auth \u2192 RateLimit \u2192 Logging \u2192 Handler
	var chain http.Handler = handler.LoggingMiddleware(mux)
	if rateLimiter != nil {
		chain = rateLimiter.Middleware(chain)
	}
	chain = handler.AuthMiddleware(cfg.APIKey)(chain)
	chain = handler.SecurityHeadersMiddleware(chain)
	chain = handler.RecoveryMiddleware(chain)
	chain = handler.RequestIDMiddleware(chain)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      chain,
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

// swaggerDocsHandler returns an HTTP handler that serves the Swagger UI.
// It overrides the restrictive Content-Security-Policy set by SecurityHeadersMiddleware
// so the UI assets (inline scripts/styles) can load correctly.
func swaggerDocsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		httpSwagger.Handler(
			httpSwagger.URL("/docs/doc.json"),
		).ServeHTTP(w, r)
	})
}
