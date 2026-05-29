package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/api/middleware"
)

func TestAuthMiddleware_disabled(t *testing.T) {
	handler := middleware.Auth(func() (bool, string) {
		return false, ""
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when auth disabled, got %d", rec.Code)
	}
}

func TestAuthMiddleware_notoken(t *testing.T) {
	handler := middleware.Auth(func() (bool, string) {
		return true, "secret"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", rec.Code)
	}
}

func TestAuthMiddleware_validtoken(t *testing.T) {
	handler := middleware.Auth(func() (bool, string) {
		return true, "secret"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", rec.Code)
	}
}

func TestAuthMiddleware_wrongtoken(t *testing.T) {
	handler := middleware.Auth(func() (bool, string) {
		return true, "secret"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong token, got %d", rec.Code)
	}
}
