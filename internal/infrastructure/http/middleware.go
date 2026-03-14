package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// contextKey is an unexported type for context keys in this package,
// preventing collisions with keys defined in other packages.
type contextKey string

const requestIDKey contextKey = "request_id"

// RequestIDMiddleware assigns a unique request ID to every inbound HTTP request.
// If the client provides a valid UUID in the X-Request-ID header, that value is
// reused; otherwise a new UUID v4 is generated. The ID is stored in the request
// context and echoed back via the X-Request-ID response header.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext extracts the request ID from ctx.
// Returns an empty string when no request ID is present.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// RecoveryMiddleware catches panics and returns a 500 response instead of crashing.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", // #nosec G706 -- structured logging, no injection risk
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "internal error",
					"code":  "INTERNAL",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware sets standard security headers on all responses.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs all HTTP requests with method, path, status, duration, and request ID.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		slog.Info("http request", // #nosec G706 -- structured logging, no injection risk
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", RequestIDFromContext(r.Context()),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// --- Rate Limiting ---

// ipLimiter holds a per-IP rate limiter and the last time it was used.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter manages per-IP rate limiters with automatic cleanup of stale entries.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	rate     rate.Limit
	burst    int
	// bypassPaths are paths that skip rate limiting (e.g. /health, /metrics).
	bypassPaths map[string]struct{}
}

// NewRateLimiter creates a RateLimiter with the given rate (requests/second) and burst size.
// It starts a background goroutine that removes entries unseen for 3 minutes.
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		rate:     r,
		burst:    burst,
		bypassPaths: map[string]struct{}{
			"/health":  {},
			"/metrics": {},
		},
	}

	go rl.cleanup()
	return rl
}

// cleanup removes IP entries that haven't been seen in the last 3 minutes.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, entry := range rl.limiters {
			if time.Since(entry.lastSeen) > 3*time.Minute {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getLimiter returns the rate limiter for the given IP, creating one if needed.
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limiters[ip]
	if !exists {
		entry = &ipLimiter{
			limiter: rate.NewLimiter(rl.rate, rl.burst),
		}
		rl.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// clientIP extracts the client IP address from the request.
// It checks X-Forwarded-For first (first entry), then falls back to RemoteAddr.
func clientIP(r *http.Request) string {
	// Check X-Forwarded-For header (first entry is the original client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Fall back to RemoteAddr, stripping the port
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // already has no port
	}
	return host
}

// Middleware returns an http.Handler middleware that enforces per-IP rate limiting.
// Paths in bypassPaths (e.g. /health, /metrics) are not rate-limited.
// When the limit is exceeded, it returns 429 Too Many Requests with a JSON body
// and a Retry-After header indicating seconds until the next allowed request.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass rate limiting for health/metrics endpoints
		if _, bypass := rl.bypassPaths[r.URL.Path]; bypass {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r)
		limiter := rl.getLimiter(ip)

		reservation := limiter.Reserve()
		if !reservation.OK() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
				"code":  "RATE_LIMITED",
			})
			return
		}

		delay := reservation.Delay()
		if delay > 0 {
			reservation.Cancel()

			retryAfter := int(math.Ceil(delay.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}

			slog.Warn("rate limit exceeded",
				"ip", ip,
				"path", r.URL.Path,
				"retry_after_seconds", retryAfter,
			)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
				"code":  "RATE_LIMITED",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
