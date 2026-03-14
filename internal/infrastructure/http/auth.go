package http

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// AuthMiddleware enforces API key authentication via the X-API-Key header.
// If apiKey is empty, authentication is disabled (development mode).
// The /health endpoint always bypasses authentication.
func AuthMiddleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if no API key is configured (development mode)
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Health and metrics endpoints always bypass auth
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			provided := r.Header.Get("X-API-Key")
			if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "unauthorized",
					"code":  "UNAUTHORIZED",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
