package handlers

import (
	"encoding/json"
	"net/http"
)

type settingsResponse struct {
	AuthEnabled bool `json:"auth_enabled"`
}

// GetSettings handles GET /api/v1/settings.
func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	enabled, _ := h.Store.GetSetting(r.Context(), "auth_enabled")
	writeJSON(w, http.StatusOK, settingsResponse{AuthEnabled: enabled == "true"})
}

// UpdateSettings handles PUT /api/v1/settings.
func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthEnabled *bool   `json:"auth_enabled"`
		AuthToken   *string `json:"auth_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.AuthEnabled != nil {
		val := "false"
		if *req.AuthEnabled {
			val = "true"
		}
		h.Store.SetSetting(r.Context(), "auth_enabled", val)
	}
	if req.AuthToken != nil && *req.AuthToken != "" {
		h.Store.SetSetting(r.Context(), "auth_token", *req.AuthToken)
	}
	h.GetSettings(w, r)
}
