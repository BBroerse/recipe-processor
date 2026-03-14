package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpinfra "github.com/bbroerse/recipe-processor/internal/infrastructure/http"
)

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
