package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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

// Package-level Prometheus metrics, auto-registered with the default registry.
var (
httpRequestsTotal = promauto.NewCounterVec(
prometheus.CounterOpts{
Name: "http_requests_total",
Help: "Total number of HTTP requests processed, partitioned by method, path, and status code.",
},
[]string{"method", "path", "status"},
)

httpRequestDurationSeconds = promauto.NewHistogramVec(
prometheus.HistogramOpts{
Name:    "http_request_duration_seconds",
Help:    "Histogram of HTTP request durations in seconds, partitioned by method and path.",
Buckets: prometheus.DefBuckets,
},
[]string{"method", "path"},
)
)

// MetricsMiddleware records Prometheus metrics for every HTTP request.
func MetricsMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
start := time.Now()
sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
next.ServeHTTP(sw, r)
path := normalizePath(r.URL.Path)
httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(sw.status)).Inc()
httpRequestDurationSeconds.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
})
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// normalizePath replaces dynamic path segments with {id} to keep Prometheus label cardinality bounded.
func normalizePath(path string) string {
switch path {
case "/health", "/metrics", "/recipes":
return path
}
segments := strings.Split(strings.Trim(path, "/"), "/")
for i, seg := range segments {
if uuidPattern.MatchString(seg) || isNumeric(seg) {
segments[i] = "{id}"
}
}
return "/" + strings.Join(segments, "/")
}

// isNumeric reports whether s consists entirely of ASCII digits.
func isNumeric(s string) bool {
if s == "" {
return false
}
for _, c := range s {
if c < '0' || c > '9' {
return false
}
}
return true
}
