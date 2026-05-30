package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/t0mer/SwitchDeck/internal/notification"
)

type channelRequest struct {
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	Config        string `json:"config"`
	Enabled       bool   `json:"enabled"`
	NotifyOffline bool   `json:"notify_offline"`
	NotifyOnline  bool   `json:"notify_online"`
}

func validateProvider(p string) bool {
	return p == notification.ProviderShoutrrr ||
		p == notification.ProviderGreenAPI ||
		p == notification.ProviderWhatsApp
}

// ListNotifications handles GET /api/v1/notifications.
func (h *Handlers) ListNotifications(w http.ResponseWriter, r *http.Request) {
	channels, err := h.NotifStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if channels == nil {
		channels = []notification.Channel{}
	}
	writeJSON(w, http.StatusOK, channels)
}

// CreateNotification handles POST /api/v1/notifications.
func (h *Handlers) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req channelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || !validateProvider(req.Provider) || req.Config == "" {
		writeError(w, http.StatusBadRequest, "name, provider, and config are required; provider must be shoutrrr|greenapi|whatsapp_web")
		return
	}
	ch, err := h.NotifStore.Create(r.Context(), notification.Channel{
		Name: req.Name, Provider: req.Provider, Config: req.Config,
		Enabled: req.Enabled, NotifyOffline: req.NotifyOffline, NotifyOnline: req.NotifyOnline,
	})
	if err != nil {
		if err == notification.ErrDuplicate {
			writeError(w, http.StatusConflict, "a channel with that name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

// UpdateNotification handles PUT /api/v1/notifications/{id}.
func (h *Handlers) UpdateNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req channelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !validateProvider(req.Provider) {
		writeError(w, http.StatusBadRequest, "invalid provider")
		return
	}
	if err := h.NotifStore.Update(r.Context(), notification.Channel{
		ID: id, Name: req.Name, Provider: req.Provider, Config: req.Config,
		Enabled: req.Enabled, NotifyOffline: req.NotifyOffline, NotifyOnline: req.NotifyOnline,
	}); err != nil {
		if err == notification.ErrNotFound {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteNotification handles DELETE /api/v1/notifications/{id}.
func (h *Handlers) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.NotifStore.Delete(r.Context(), id); err != nil {
		if err == notification.ErrNotFound {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestNotification handles POST /api/v1/notifications/test.
func (h *Handlers) TestNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		Config   string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !validateProvider(req.Provider) || req.Config == "" {
		writeError(w, http.StatusBadRequest, "provider and config required")
		return
	}
	ch := &notification.Channel{Provider: req.Provider, Config: req.Config}
	sender, err := notification.NewSender(ch)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid config: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := sender.Send(ctx, "🧪 SwitchDeck test notification"); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
