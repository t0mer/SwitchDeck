package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/t0mer/SwitchDeck/internal/auth"
)

type settingsResponse struct {
	AuthEnabled bool  `json:"auth_enabled"`
	UsernameSet bool  `json:"username_set"`
	TokenSet    bool  `json:"token_set"`
	TokenExpiry int64 `json:"token_expiry"`
}

// GetSettings handles GET /api/v1/settings.
func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	enabled, _ := h.Store.GetSetting(ctx, "auth_enabled")
	username, _ := h.Store.GetSetting(ctx, "auth_username")
	token, _ := h.Store.GetSetting(ctx, "auth_token")
	expiryStr, _ := h.Store.GetSetting(ctx, "auth_token_expiry")
	var expiry int64
	if expiryStr != "" {
		expiry, _ = strconv.ParseInt(expiryStr, 10, 64)
	}
	writeJSON(w, http.StatusOK, settingsResponse{
		AuthEnabled: enabled == "true",
		UsernameSet: username != "",
		TokenSet:    token != "",
		TokenExpiry: expiry,
	})
}

// UpdateSettings handles PUT /api/v1/settings.
func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthEnabled *bool   `json:"auth_enabled"`
		Username    *string `json:"username"`
		Password    *string `json:"password"`
		TokenExpiry *int64  `json:"token_expiry"`
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
	if req.TokenExpiry != nil {
		h.Store.SetSetting(ctx, "auth_token_expiry", strconv.FormatInt(*req.TokenExpiry, 10))
	}
	h.GetSettings(w, r)
}

// RotateToken handles POST /api/v1/settings/rotate-token.
func (h *Handlers) RotateToken(w http.ResponseWriter, r *http.Request) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, "random error")
		return
	}
	token := hex.EncodeToString(raw)
	h.Store.SetSetting(context.Background(), "auth_token", token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "token": token})
}
