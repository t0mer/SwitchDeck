package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// TokenProvider is called per-request to get current auth config.
// Returns (enabled bool, token string).
type TokenProvider func() (bool, string)

// Auth returns middleware that optionally enforces bearer token authentication.
// When provider returns enabled=false, all requests pass through.
func Auth(provider TokenProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			enabled, token := provider()
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}
			presented := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if presented == "" || !safeEqual(presented, token) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// safeEqual compares two strings in constant time.
func safeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
