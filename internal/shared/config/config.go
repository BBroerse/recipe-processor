package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Env                string // development, production
	Server             ServerConfig
	Database           DatabaseConfig
	Ollama             OllamaConfig
	Log                LogConfig
	APIKey             string // optional: if set, all non-health endpoints require X-API-Key header
	EventBusBufferSize int    // buffer size for the in-memory event bus channel
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port               int
	MaxRequestBodySize int64 // maximum allowed request body size in bytes
}

// DatabaseConfig holds PostgreSQL connection configuration.
type DatabaseConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	Name           string
	SSLMode        string
	MaxConnections int32 // maximum number of connections in the pool
	MinConnections int32 // minimum number of idle connections in the pool
}

// DSN returns the PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

// OllamaConfig holds configuration for the Ollama LLM client.
type OllamaConfig struct {
	URL       string
	Model     string
	Timeout   time.Duration // timeout for LLM requests
	RateLimit int           // maximum requests per second
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string // debug, info, warn, error
}

// Load reads configuration from environment variables.
// Required variables will cause a fatal error if missing.
func Load() (*Config, error) {
	var missing []string

	env := getEnv("ENV", "development")

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		missing = append(missing, "DB_PASSWORD")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}

	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}
	serverPort, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}

	maxRequestBodySize, err := strconv.ParseInt(getEnv("MAX_REQUEST_BODY_SIZE", "65536"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_REQUEST_BODY_SIZE: %w", err)
	}

	dbMaxConns, err := strconv.ParseInt(getEnv("DB_MAX_CONNECTIONS", "25"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MAX_CONNECTIONS: %w", err)
	}
	dbMinConns, err := strconv.ParseInt(getEnv("DB_MIN_CONNECTIONS", "2"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MIN_CONNECTIONS: %w", err)
	}

	ollamaTimeout, err := time.ParseDuration(getEnv("OLLAMA_TIMEOUT", "120s"))
	if err != nil {
		return nil, fmt.Errorf("invalid OLLAMA_TIMEOUT: %w", err)
	}
	ollamaRateLimit, err := strconv.Atoi(getEnv("OLLAMA_RATE_LIMIT", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid OLLAMA_RATE_LIMIT: %w", err)
	}

	eventBusBufferSize, err := strconv.Atoi(getEnv("EVENT_BUS_BUFFER_SIZE", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid EVENT_BUS_BUFFER_SIZE: %w", err)
	}

	return &Config{
		Env:                env,
		APIKey:             os.Getenv("API_KEY"),
		EventBusBufferSize: eventBusBufferSize,
		Server: ServerConfig{
			Port:               serverPort,
			MaxRequestBodySize: maxRequestBodySize,
		},
		Database: DatabaseConfig{
			Host:           getEnv("DB_HOST", "localhost"),
			Port:           dbPort,
			User:           getEnv("DB_USER", "postgres"),
			Password:       dbPassword,
			Name:           getEnv("DB_NAME", "recipes"),
			SSLMode:        getEnv("DB_SSLMODE", "require"),
			MaxConnections: int32(dbMaxConns),
			MinConnections: int32(dbMinConns),
		},
		Ollama: OllamaConfig{
			URL:       getEnv("OLLAMA_URL", "http://localhost:11434"),
			Model:     getEnv("OLLAMA_MODEL", "tinyllama"),
			Timeout:   ollamaTimeout,
			RateLimit: ollamaRateLimit,
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
