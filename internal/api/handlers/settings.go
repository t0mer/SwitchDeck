package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/t0mer/SwitchDeck/internal/auth"
	"github.com/t0mer/SwitchDeck/internal/store"
)

type settingsResponse struct {
	AuthEnabled bool `json:"auth_enabled"`
	UsernameSet bool `json:"username_set"`
}

// GetSettings handles GET /api/v1/settings.
func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	enabled, _ := h.Store.GetSetting(ctx, "auth_enabled")
	username, _ := h.Store.GetSetting(ctx, "auth_username")
	writeJSON(w, http.StatusOK, settingsResponse{
		AuthEnabled: enabled == "true",
		UsernameSet: username != "",
	})
}

// UpdateSettings handles PUT /api/v1/settings.
func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthEnabled *bool   `json:"auth_enabled"`
		Username    *string `json:"username"`
		Password    *string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	ctx := r.Context()
	if req.AuthEnabled != nil {
		val := "false"
		if *req.AuthEnabled {
			val = "true"
		}
		h.Store.SetSetting(ctx, "auth_enabled", val)
	}
	if req.Username != nil && req.Password != nil {
		if len(*req.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash error")
			return
		}
		h.Store.SetSetting(ctx, "auth_username", *req.Username)
		h.Store.SetSetting(ctx, "auth_password_hash", hash)
	}
	h.GetSettings(w, r)
}

// ── API token CRUD ────────────────────────────────────────────────────────

type tokenResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Expiry    int64     `json:"expiry"`
	CreatedAt time.Time `json:"created_at"`
}

func tokenToResp(t store.ApiToken) tokenResponse {
	return tokenResponse{ID: t.ID, Name: t.Name, Expiry: t.Expiry, CreatedAt: t.CreatedAt}
}

// ListTokens handles GET /api/v1/settings/tokens.
func (h *Handlers) ListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.Store.ListApiTokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]tokenResponse, len(tokens))
	for i, t := range tokens {
		resp[i] = tokenToResp(t)
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateToken handles POST /api/v1/settings/tokens.
// Returns the plaintext token once in the response — it is never retrievable again.
func (h *Handlers) CreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Expiry int64  `json:"expiry"` // unix timestamp; 0 = never
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		writeError(w, http.StatusInternalServerError, "random error")
		return
	}
	plaintext := hex.EncodeToString(buf)
	tok, err := h.Store.CreateApiToken(r.Context(), req.Name, plaintext, req.Expiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         tok.ID,
		"name":       tok.Name,
		"expiry":     tok.Expiry,
		"created_at": tok.CreatedAt,
		"token":      plaintext, // shown once — never returned again
	})
}

// DeleteToken handles DELETE /api/v1/settings/tokens/{id}.
func (h *Handlers) DeleteToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteApiToken(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
