package http_test

import (
"context"
"encoding/json"
"net/http"
"net/http/httptest"
"testing"

"github.com/google/uuid"
"github.com/prometheus/client_golang/prometheus"
dto "github.com/prometheus/client_model/go"
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


// --- Metrics Middleware Tests ---

func getCounterValue(t *testing.T, name string, labels prometheus.Labels) float64 {
t.Helper()
metrics, err := prometheus.DefaultGatherer.Gather()
require.NoError(t, err)
for _, mf := range metrics {
if mf.GetName() != name {
continue
}
for _, m := range mf.GetMetric() {
if matchLabels(m.GetLabel(), labels) {
return m.GetCounter().GetValue()
}
}
}
return 0
}

func getHistogramCount(t *testing.T, name string, labels prometheus.Labels) uint64 {
t.Helper()
metrics, err := prometheus.DefaultGatherer.Gather()
require.NoError(t, err)
for _, mf := range metrics {
if mf.GetName() != name {
continue
}
for _, m := range mf.GetMetric() {
if matchLabels(m.GetLabel(), labels) {
return m.GetHistogram().GetSampleCount()
}
}
}
return 0
}

func matchLabels(pairs []*dto.LabelPair, expected prometheus.Labels) bool {
if len(pairs) != len(expected) {
return false
}
for _, lp := range pairs {
v, ok := expected[lp.GetName()]
if !ok || v != lp.GetValue() {
return false
}
}
return true
}

func TestMetricsMiddleware_CounterIncrements(t *testing.T) {
h := httpinfra.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusOK)
}))
before := getCounterValue(t, "http_requests_total", prometheus.Labels{
"method": "GET", "path": "/health", "status": "200",
})
req := httptest.NewRequest(http.MethodGet, "/health", nil)
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
after := getCounterValue(t, "http_requests_total", prometheus.Labels{
"method": "GET", "path": "/health", "status": "200",
})
assert.Equal(t, before+1, after, "counter should increment by 1 after a request")
}

func TestMetricsMiddleware_HistogramObservesDuration(t *testing.T) {
h := httpinfra.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusOK)
}))
before := getHistogramCount(t, "http_request_duration_seconds", prometheus.Labels{
"method": "GET", "path": "/health",
})
req := httptest.NewRequest(http.MethodGet, "/health", nil)
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
after := getHistogramCount(t, "http_request_duration_seconds", prometheus.Labels{
"method": "GET", "path": "/health",
})
assert.Equal(t, before+1, after, "histogram sample count should increment by 1")
}

func TestMetricsMiddleware_PathNormalizesUUID(t *testing.T) {
h := httpinfra.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusOK)
}))
before := getCounterValue(t, "http_requests_total", prometheus.Labels{
"method": "GET", "path": "/recipes/{id}", "status": "200",
})
req := httptest.NewRequest(http.MethodGet, "/recipes/550e8400-e29b-41d4-a716-446655440000", nil)
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
after := getCounterValue(t, "http_requests_total", prometheus.Labels{
"method": "GET", "path": "/recipes/{id}", "status": "200",
})
assert.Equal(t, before+1, after, "UUID path segments should be normalized to {id}")
}

func TestMetricsMiddleware_PathNormalizesNumeric(t *testing.T) {
h := httpinfra.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusOK)
}))
before := getCounterValue(t, "http_requests_total", prometheus.Labels{
"method": "GET", "path": "/recipes/{id}", "status": "200",
})
req := httptest.NewRequest(http.MethodGet, "/recipes/42", nil)
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
after := getCounterValue(t, "http_requests_total", prometheus.Labels{
"method": "GET", "path": "/recipes/{id}", "status": "200",
})
assert.Equal(t, before+1, after, "numeric path segments should be normalized to {id}")
}

func TestMetricsMiddleware_CapturesNon200Status(t *testing.T) {
h := httpinfra.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusNotFound)
}))
before := getCounterValue(t, "http_requests_total", prometheus.Labels{
"method": "GET", "path": "/recipes/{id}", "status": "404",
})
req := httptest.NewRequest(http.MethodGet, "/recipes/550e8400-e29b-41d4-a716-446655440000", nil)
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
after := getCounterValue(t, "http_requests_total", prometheus.Labels{
"method": "GET", "path": "/recipes/{id}", "status": "404",
})
assert.Equal(t, before+1, after, "404 status should be recorded correctly")
}

func TestAuthMiddleware_MetricsBypassesAuth(t *testing.T) {
middleware := httpinfra.AuthMiddleware("test-secret-key")
h := middleware(okHandler())
req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code, "/metrics should bypass auth")
}
