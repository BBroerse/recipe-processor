package http_test

import (
"context"
"encoding/json"
"net/http"
"net/http/httptest"
"testing"

"github.com/google/uuid"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"

httpinfra "github.com/bbroerse/recipe-processor/internal/infrastructure/http"
)

// okHandler is a simple handler that always returns 200 OK.
func okHandler() http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusOK)
_, _ = w.Write([]byte(`{"status":"ok"}`))
})
}

// --- Request ID Middleware Tests ---

func TestRequestIDMiddleware_NoHeader_GeneratesUUID(t *testing.T) {
handler := httpinfra.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
}))

req := httptest.NewRequest(http.MethodGet, "/test", nil)
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

got := rec.Header().Get("X-Request-ID")
require.NotEmpty(t, got, "response must contain X-Request-ID header")

_, err := uuid.Parse(got)
assert.NoError(t, err, "X-Request-ID must be a valid UUID")
}

func TestRequestIDMiddleware_ValidHeader_UsesProvided(t *testing.T) {
clientID := uuid.New().String()

handler := httpinfra.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
}))

req := httptest.NewRequest(http.MethodGet, "/test", nil)
req.Header.Set("X-Request-ID", clientID)
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

assert.Equal(t, clientID, rec.Header().Get("X-Request-ID"),
"middleware must reuse the valid client-provided request ID")
}

func TestRequestIDMiddleware_InvalidHeader_GeneratesNewUUID(t *testing.T) {
handler := httpinfra.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
}))

req := httptest.NewRequest(http.MethodGet, "/test", nil)
req.Header.Set("X-Request-ID", "not-a-uuid")
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

got := rec.Header().Get("X-Request-ID")
require.NotEmpty(t, got, "response must contain X-Request-ID header")
assert.NotEqual(t, "not-a-uuid", got, "invalid client ID must be replaced")

_, err := uuid.Parse(got)
assert.NoError(t, err, "replacement X-Request-ID must be a valid UUID")
}

func TestRequestIDFromContext_ReturnsCorrectValue(t *testing.T) {
var captured string

handler := httpinfra.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
captured = httpinfra.RequestIDFromContext(r.Context())
w.WriteHeader(http.StatusOK)
}))

req := httptest.NewRequest(http.MethodGet, "/test", nil)
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

assert.NotEmpty(t, captured, "RequestIDFromContext must return the ID from context")
assert.Equal(t, rec.Header().Get("X-Request-ID"), captured,
"context value must match response header")
}

func TestRequestIDFromContext_EmptyContext_ReturnsEmpty(t *testing.T) {
got := httpinfra.RequestIDFromContext(context.Background())
assert.Empty(t, got, "must return empty string for context without request ID")
}

// --- Auth Middleware Tests ---

func TestAuthMiddleware_NoKeyHeader_Returns401(t *testing.T) {
middleware := httpinfra.AuthMiddleware("test-secret-key")
handler := middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

if rec.Code != http.StatusUnauthorized {
t.Errorf("expected status 401, got %d", rec.Code)
}

var body map[string]string
if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
t.Fatalf("failed to decode response body: %v", err)
}
if body["error"] != "unauthorized" {
t.Errorf("expected error 'unauthorized', got %q", body["error"])
}
if body["code"] != "UNAUTHORIZED" {
t.Errorf("expected code 'UNAUTHORIZED', got %q", body["code"])
}
}

func TestAuthMiddleware_WrongKey_Returns401(t *testing.T) {
middleware := httpinfra.AuthMiddleware("test-secret-key")
handler := middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.Header.Set("X-API-Key", "wrong-key")
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

if rec.Code != http.StatusUnauthorized {
t.Errorf("expected status 401, got %d", rec.Code)
}

var body map[string]string
if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
t.Fatalf("failed to decode response body: %v", err)
}
if body["error"] != "unauthorized" {
t.Errorf("expected error 'unauthorized', got %q", body["error"])
}
if body["code"] != "UNAUTHORIZED" {
t.Errorf("expected code 'UNAUTHORIZED', got %q", body["code"])
}
}

func TestAuthMiddleware_CorrectKey_PassesThrough(t *testing.T) {
middleware := httpinfra.AuthMiddleware("test-secret-key")
handler := middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.Header.Set("X-API-Key", "test-secret-key")
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

if rec.Code != http.StatusOK {
t.Errorf("expected status 200, got %d", rec.Code)
}
}

func TestAuthMiddleware_HealthBypassesAuth(t *testing.T) {
middleware := httpinfra.AuthMiddleware("test-secret-key")
handler := middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/health", nil)
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

if rec.Code != http.StatusOK {
t.Errorf("expected status 200 for /health without auth, got %d", rec.Code)
}
}

func TestAuthMiddleware_NoAPIKeyConfigured_PassesThrough(t *testing.T) {
middleware := httpinfra.AuthMiddleware("")
handler := middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

if rec.Code != http.StatusOK {
t.Errorf("expected status 200 when API_KEY not configured, got %d", rec.Code)
}
}

func TestAuthMiddleware_ContentTypeJSON(t *testing.T) {
middleware := httpinfra.AuthMiddleware("test-secret-key")
handler := middleware(okHandler())

req := httptest.NewRequest(http.MethodPost, "/recipes", nil)
rec := httptest.NewRecorder()

handler.ServeHTTP(rec, req)

ct := rec.Header().Get("Content-Type")
if ct != "application/json" {
t.Errorf("expected Content-Type 'application/json', got %q", ct)
}
}

// --- Rate Limit Middleware Tests ---

func TestRateLimitMiddleware_AllowsRequestsWithinLimit(t *testing.T) {
rl := httpinfra.NewRateLimiter(10, 20) // 10 req/s, burst 20
h := rl.Middleware(okHandler())

for i := 0; i < 5; i++ {
req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "192.168.1.1:12345"
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code, "request %d within limit should return 200", i+1)
}
}

func TestRateLimitMiddleware_RejectsExcessiveRequests(t *testing.T) {
rl := httpinfra.NewRateLimiter(1, 1)
h := rl.Middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.1:12345"
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code, "first request should pass")

req = httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.1:12345"
rec = httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusTooManyRequests, rec.Code, "excess request should return 429")

var body map[string]string
err := json.NewDecoder(rec.Body).Decode(&body)
require.NoError(t, err, "response body must be valid JSON")
assert.Equal(t, "rate limit exceeded", body["error"])
assert.Equal(t, "RATE_LIMITED", body["code"])
}

func TestRateLimitMiddleware_HealthBypassesRateLimit(t *testing.T) {
rl := httpinfra.NewRateLimiter(1, 1)
h := rl.Middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.2:12345"
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code)

for i := 0; i < 5; i++ {
req = httptest.NewRequest(http.MethodGet, "/health", nil)
req.RemoteAddr = "10.0.0.2:12345"
rec = httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code, "/health request %d should bypass rate limit", i+1)
}
}

func TestRateLimitMiddleware_DifferentIPsHaveIndependentLimits(t *testing.T) {
rl := httpinfra.NewRateLimiter(1, 1)
h := rl.Middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.10:12345"
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code, "IP A first request should pass")

req = httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.10:12345"
rec = httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusTooManyRequests, rec.Code, "IP A second request should be limited")

req = httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.20:12345"
rec = httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code, "IP B first request should pass")
}

func TestRateLimitMiddleware_429IncludesRetryAfterHeader(t *testing.T) {
rl := httpinfra.NewRateLimiter(1, 1)
h := rl.Middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.30:12345"
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code)

req = httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.30:12345"
rec = httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusTooManyRequests, rec.Code)

retryAfter := rec.Header().Get("Retry-After")
assert.NotEmpty(t, retryAfter, "429 response must include Retry-After header")
assert.Regexp(t, `^[1-9][0-9]*$`, retryAfter, "Retry-After must be a positive integer")
}

func TestRateLimitMiddleware_XForwardedFor(t *testing.T) {
rl := httpinfra.NewRateLimiter(1, 1)
h := rl.Middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "127.0.0.1:12345"
req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18, 150.172.238.178")
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code)

req = httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "127.0.0.1:12345"
req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
rec = httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusTooManyRequests, rec.Code, "same XFF client IP should share a limiter")

req = httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "127.0.0.2:54321"
req.Header.Set("X-Forwarded-For", "203.0.113.50")
rec = httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusTooManyRequests, rec.Code, "same XFF IP via different proxy should share a limiter")
}

func TestRateLimitMiddleware_MetricsBypassesRateLimit(t *testing.T) {
rl := httpinfra.NewRateLimiter(1, 1)
h := rl.Middleware(okHandler())

req := httptest.NewRequest(http.MethodGet, "/recipes/123", nil)
req.RemoteAddr = "10.0.0.40:12345"
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code)

req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
req.RemoteAddr = "10.0.0.40:12345"
rec = httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code, "/metrics should bypass rate limit")
}
