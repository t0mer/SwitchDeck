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
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	IP              string              `json:"ip"`
	Username        string              `json:"username"`
	InsecureTLS     bool                `json:"insecure_tls"`
	Enabled         bool                `json:"enabled"`
	PollStatsSecs   int                 `json:"poll_stats_secs"`
	PollConfigSecs  int                 `json:"poll_config_secs"`
	Status          models.SwitchStatus `json:"status"`
	Model           string              `json:"model,omitempty"`
	PortsTotal      int                 `json:"ports_total"`
	PortsUp         int                 `json:"ports_up"`
	PortsDown       int                 `json:"ports_down"`
	CollectingSince int64               `json:"collecting_since,omitempty"` // unix seconds, non-zero while collecting
	PortStates      []string            `json:"port_states,omitempty"`      // per-port status in port-number order: "up"|"down"|"disabled"
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
		if t, ok := h.Manager.CollectingStartedAt(cfg.ID); ok {
			resp[i].CollectingSince = t.Unix()
		}
		if snap, err := h.Store.LatestSnapshot(r.Context(), cfg.ID); err == nil {
			resp[i].Model = snap.Switch.Model
			states := make([]string, len(snap.Ports))
			for j, p := range snap.Ports {
				// Encode enough information for the UI to colour each port LED:
				//   "disabled"        — port is administratively disabled
				//   "down"            — enabled but no link
				//   "10M"|"100M"|"1G" — link up at this speed (2.5G / 10G map to "1G")
				switch p.Status {
				case models.PortStatusUp:
					switch p.Speed {
					case models.PortSpeed10M:
						states[j] = "10M"
					case models.PortSpeed100M:
						states[j] = "100M"
					default: // 1G, 2.5G, 10G, or unknown fast link
						states[j] = "1G"
					}
					resp[i].PortsUp++
				case models.PortStatusDown:
					states[j] = "down"
					resp[i].PortsDown++
				default: // PortStatusDisabled
					states[j] = "disabled"
				}
				resp[i].PortsTotal++
			}
			if len(states) > 0 {
				resp[i].PortStates = states
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
	// Mark as collecting before the goroutine runs so the status is visible
	// immediately when the UI reloads the switch list after this response.
	h.Manager.MarkCollecting(cfg.ID)
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
	r2 := cfgToResponse(cfg, models.SwitchStatusCollecting)
	if t, ok := h.Manager.CollectingStartedAt(cfg.ID); ok {
		r2.CollectingSince = t.Unix()
	}
	writeJSON(w, http.StatusCreated, r2)
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
// Returns 202 immediately and runs the collection in the background so the
// UI can show a live countdown while it progresses.
func (h *Handlers) TriggerCollect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.Store.GetSwitch(r.Context(), id, h.EncKey); err != nil {
		writeError(w, http.StatusNotFound, "switch not found")
		return
	}
	// If already collecting return current status without starting a second run.
	if t, ok := h.Manager.CollectingStartedAt(id); ok {
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"status": "collecting", "collecting_since": t.Unix(),
		})
		return
	}
	h.Manager.MarkCollecting(id)
	go func() {
		snap, err := h.Manager.CollectNow(context.Background(), id)
		if err != nil {
			log.Printf("collect[%s]: %v", id, err)
			return
		}
		if err := h.Store.UpsertSnapshot(context.Background(), snap); err != nil {
			log.Printf("collect[%s]: store: %v", id, err)
		}
	}()
	since := int64(0)
	if t, ok := h.Manager.CollectingStartedAt(id); ok {
		since = t.Unix()
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status": "collecting", "collecting_since": since,
	})
}
