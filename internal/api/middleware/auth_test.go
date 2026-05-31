package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/t0mer/SwitchDeck/internal/api/middleware"
	"github.com/t0mer/SwitchDeck/internal/store"
)

func openTestStore(t *testing.T) store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthAPI_disabled(t *testing.T) {
	st := openTestStore(t)
	h := middleware.AuthAPI(st)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when disabled, got %d", rec.Code)
	}
}

func TestAuthAPI_enabledNoCredentials(t *testing.T) {
	st := openTestStore(t)
	st.SetSetting(context.Background(), "auth_enabled", "true")
	// No username set → bootstrap mode → pass through
	h := middleware.AuthAPI(st)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 in bootstrap mode, got %d", rec.Code)
	}
}

func TestAuthAPI_bearerToken(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	st.SetSetting(ctx, "auth_enabled", "true")
	st.SetSetting(ctx, "auth_username", "admin")
	// Create a token in the api_tokens table
	if _, err := st.CreateApiToken(ctx, "test", "secret-token", 0); err != nil {
		t.Fatalf("CreateApiToken: %v", err)
	}

	h := middleware.AuthAPI(st)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: expected 401, got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Authorization", "Bearer secret-token")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("correct token: expected 200, got %d", rec2.Code)
	}
}

func TestAuthAPI_tokenExpiry(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	st.SetSetting(ctx, "auth_enabled", "true")
	st.SetSetting(ctx, "auth_username", "admin")
	past := time.Now().Add(-time.Hour).Unix()
	if _, err := st.CreateApiToken(ctx, "expired", "tok", past); err != nil {
		t.Fatalf("CreateApiToken: %v", err)
	}

	h := middleware.AuthAPI(st)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expired token: expected 401, got %d", rec.Code)
	}
}

func TestAuthUI_redirectsToLogin(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	st.SetSetting(ctx, "auth_enabled", "true")
	st.SetSetting(ctx, "auth_username", "admin")

	h := middleware.AuthUI(st)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings", nil))
	if rec.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", rec.Code)
	}
}

func TestAuthAPI_sessionCookie(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	st.SetSetting(ctx, "auth_enabled", "true")
	st.SetSetting(ctx, "auth_username", "admin")

	rec := httptest.NewRecorder()
	if err := middleware.SetSessionCookie(rec, st, "admin"); err != nil {
		t.Fatalf("SetSessionCookie: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie set")
	}

	h := middleware.AuthAPI(st)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Errorf("valid session cookie: expected 200, got %d", rec2.Code)
	}
}
