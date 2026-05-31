package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/t0mer/SwitchDeck/internal/api/middleware"
	"github.com/t0mer/SwitchDeck/internal/auth"
)

// Login handles POST /api/v1/auth/login.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	storedUser, _ := h.Store.GetSetting(r.Context(), "auth_username")
	storedHash, _ := h.Store.GetSetting(r.Context(), "auth_password_hash")
	if req.Username != storedUser || !auth.VerifyPassword(storedHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := middleware.SetSessionCookie(w, h.Store, req.Username); err != nil {
		writeError(w, http.StatusInternalServerError, "session error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	middleware.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Session handles GET /api/v1/auth/session.
func (h *Handlers) Session(w http.ResponseWriter, r *http.Request) {
	enabled, _ := h.Store.GetSetting(r.Context(), "auth_enabled")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"auth_enabled":  enabled == "true",
		"authenticated": middleware.ValidateSessionRequest(r, h.Store),
	})
}
