package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/t0mer/SwitchDeck/internal/models"
)

type switchRequest struct {
	Name           string `json:"name"`
	IP             string `json:"ip"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	InsecureTLS    bool   `json:"insecure_tls"`
	Enabled        bool   `json:"enabled"`
	PollStatsSecs  int    `json:"poll_stats_secs"`
	PollConfigSecs int    `json:"poll_config_secs"`
}

type switchResponse struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	IP             string              `json:"ip"`
	Username       string              `json:"username"`
	InsecureTLS    bool                `json:"insecure_tls"`
	Enabled        bool                `json:"enabled"`
	PollStatsSecs  int                 `json:"poll_stats_secs"`
	PollConfigSecs int                 `json:"poll_config_secs"`
	Status         models.SwitchStatus `json:"status"`
	Model          string              `json:"model,omitempty"`
	PortsTotal     int                 `json:"ports_total"`
	PortsUp        int                 `json:"ports_up"`
	PortsDown      int                 `json:"ports_down"`
}

func cfgToResponse(cfg models.SwitchConfig, status models.SwitchStatus) switchResponse {
	return switchResponse{
		ID:             cfg.ID,
		Name:           cfg.Name,
		IP:             cfg.IP,
		Username:       cfg.Username,
		InsecureTLS:    cfg.InsecureTLS,
		Enabled:        cfg.Enabled,
		PollStatsSecs:  cfg.PollStatsSecs,
		PollConfigSecs: cfg.PollConfigSecs,
		Status:         status,
	}
}

// ListSwitches handles GET /api/v1/switches.
func (h *Handlers) ListSwitches(w http.ResponseWriter, r *http.Request) {
	cfgs, err := h.Store.ListSwitches(r.Context(), h.EncKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]switchResponse, len(cfgs))
	for i, cfg := range cfgs {
		resp[i] = cfgToResponse(cfg, h.Manager.Status(cfg.ID))
		if snap, err := h.Store.LatestSnapshot(r.Context(), cfg.ID); err == nil {
			resp[i].Model = snap.Switch.Model
			for _, p := range snap.Ports {
				resp[i].PortsTotal++
				switch p.Status {
				case models.PortStatusUp:
					resp[i].PortsUp++
				case models.PortStatusDown:
					resp[i].PortsDown++
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// AddSwitch handles POST /api/v1/switches.
func (h *Handlers) AddSwitch(w http.ResponseWriter, r *http.Request) {
	var req switchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" || req.IP == "" || req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "name, ip, username, password are required")
		return
	}
	if req.PollStatsSecs == 0 {
		req.PollStatsSecs = 60
	}
	if req.PollConfigSecs == 0 {
		req.PollConfigSecs = 300
	}
	cfg := models.SwitchConfig{
		ID:             uuid.New().String(),
		Name:           req.Name,
		IP:             req.IP,
		Username:       req.Username,
		Password:       req.Password,
		InsecureTLS:    req.InsecureTLS,
		Enabled:        true,
		PollStatsSecs:  req.PollStatsSecs,
		PollConfigSecs: req.PollConfigSecs,
	}
	if err := h.Store.AddSwitch(r.Context(), cfg, h.EncKey); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.Manager.Add(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "stored but could not start worker: "+err.Error())
		return
	}
	// Kick off an immediate collection in the background so data populates
	// without the user having to click "Collect Now".
	go func() {
		snap, err := h.Manager.CollectNow(context.Background(), cfg.ID)
		if err != nil {
			log.Printf("auto-collect[%s]: %v", cfg.ID, err)
			return
		}
		if err := h.Store.UpsertSnapshot(context.Background(), snap); err != nil {
			log.Printf("auto-collect[%s]: store: %v", cfg.ID, err)
		}
	}()
	writeJSON(w, http.StatusCreated, cfgToResponse(cfg, models.SwitchStatusUnknown))
}

// GetSwitch handles GET /api/v1/switches/{id}.
func (h *Handlers) GetSwitch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cfg, err := h.Store.GetSwitch(r.Context(), id, h.EncKey)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfgToResponse(*cfg, h.Manager.Status(id)))
}

// UpdateSwitch handles PUT /api/v1/switches/{id}.
func (h *Handlers) UpdateSwitch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req switchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.PollStatsSecs == 0 {
		req.PollStatsSecs = 60
	}
	if req.PollConfigSecs == 0 {
		req.PollConfigSecs = 300
	}
	cfg := models.SwitchConfig{
		ID:             id,
		Name:           req.Name,
		IP:             req.IP,
		Username:       req.Username,
		Password:       req.Password,
		InsecureTLS:    req.InsecureTLS,
		Enabled:        req.Enabled,
		PollStatsSecs:  req.PollStatsSecs,
		PollConfigSecs: req.PollConfigSecs,
	}
	if err := h.Store.UpdateSwitch(r.Context(), cfg, h.EncKey); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.Manager.Update(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "updated but could not restart worker: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfgToResponse(cfg, h.Manager.Status(id)))
}

// DeleteSwitch handles DELETE /api/v1/switches/{id}.
func (h *Handlers) DeleteSwitch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_ = h.Manager.Remove(id)
	if err := h.Store.DeleteSwitch(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TriggerCollect handles POST /api/v1/switches/{id}/collect.
func (h *Handlers) TriggerCollect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snap, err := h.Manager.CollectNow(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "collection failed: "+err.Error())
		return
	}
	if err := h.Store.UpsertSnapshot(r.Context(), snap); err != nil {
		writeError(w, http.StatusInternalServerError, "store failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "collected"})
}
