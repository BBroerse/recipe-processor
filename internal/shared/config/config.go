package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env      string // development, production
	Server   ServerConfig
	Database DatabaseConfig
	Ollama   OllamaConfig
	Log      LogConfig
	APIKey   string // optional: if set, all non-health endpoints require X-API-Key header
}

type ServerConfig struct {
	Port           int
	RateLimit      float64 // requests per second (0 = disabled)
	RateLimitBurst int     // maximum burst size
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

type OllamaConfig struct {
	URL   string
	Model string
}

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

	rateLimit, err := strconv.ParseFloat(getEnv("RATE_LIMIT", "10"), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT: %w", err)
	}

	rateLimitBurst, err := strconv.Atoi(getEnv("RATE_LIMIT_BURST", "20"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_BURST: %w", err)
	}

	return &Config{
		Env:    env,
		APIKey: os.Getenv("API_KEY"),
		Server: ServerConfig{
			Port:           serverPort,
			RateLimit:      rateLimit,
			RateLimitBurst: rateLimitBurst,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "postgres"),
			Password: dbPassword,
			Name:     getEnv("DB_NAME", "recipes"),
			SSLMode:  getEnv("DB_SSLMODE", "require"),
		},
		Ollama: OllamaConfig{
			URL:   getEnv("OLLAMA_URL", "http://localhost:11434"),
			Model: getEnv("OLLAMA_MODEL", "tinyllama"),
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
