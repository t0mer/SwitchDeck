package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/t0mer/SwitchDeck/internal/auth"
	"github.com/t0mer/SwitchDeck/internal/store"
)

const sessionCookie = "sd_session"

// AuthAPI returns middleware for API routes: accepts session cookie OR Bearer token.
// On failure returns 401 JSON.
func AuthAPI(st store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authRequired(r.Context(), st) {
				next.ServeHTTP(w, r)
				return
			}
			if validateRequest(r, st) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		})
	}
}

// AuthUI returns middleware for UI routes: accepts session cookie.
// On failure redirects to /login?next=<original-path>.
func AuthUI(st store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authRequired(r.Context(), st) {
				next.ServeHTTP(w, r)
				return
			}
			if validateSession(r, st) {
				next.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusFound)
		})
	}
}

// SetSessionCookie writes a signed session cookie for username (24h TTL).
func SetSessionCookie(w http.ResponseWriter, st store.Store, username string) error {
	secret, err := sessionSecret(context.Background(), st)
	if err != nil {
		return err
	}
	token, err := auth.SignSession(secret, username, time.Now().Add(24*time.Hour))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ValidateSessionRequest returns true if r carries a valid session cookie.
func ValidateSessionRequest(r *http.Request, st store.Store) bool {
	return validateSession(r, st)
}

// ── helpers ───────────────────────────────────────────────────────────────

func authRequired(ctx context.Context, st store.Store) bool {
	enabled, _ := st.GetSetting(ctx, "auth_enabled")
	if enabled != "true" {
		return false
	}
	username, _ := st.GetSetting(ctx, "auth_username")
	return username != ""
}

func validateSession(r *http.Request, st store.Store) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	secret, err := sessionSecret(r.Context(), st)
	if err != nil {
		return false
	}
	_, err = auth.ParseSession(secret, cookie.Value)
	return err == nil
}

func validateRequest(r *http.Request, st store.Store) bool {
	if validateSession(r, st) {
		return true
	}
	presented := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if presented == "" {
		return false
	}
	_, err := st.MatchApiToken(r.Context(), presented)
	return err == nil
}

func sessionSecret(ctx context.Context, st store.Store) ([]byte, error) {
	existing, _ := st.GetSetting(ctx, "session_secret")
	if existing != "" {
		return base64.StdEncoding.DecodeString(existing)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	st.SetSetting(ctx, "session_secret", encoded)
	return raw, nil
}

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
