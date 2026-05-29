package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/SwitchDeck/internal/models"
)

// GetSnapshot handles GET /api/v1/switches/{id}/snapshot.
func (h *Handlers) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snap, err := h.Store.LatestSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// GetPorts handles GET /api/v1/switches/{id}/ports.
func (h *Handlers) GetPorts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snap, err := h.Store.LatestSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap.Ports)
}

// GetStats handles GET /api/v1/switches/{id}/stats.
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snap, err := h.Store.LatestSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap.PortStats)
}

// GetVLANs handles GET /api/v1/switches/{id}/vlans.
func (h *Handlers) GetVLANs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snap, err := h.Store.LatestSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap.VLANs)
}

// GetLAG handles GET /api/v1/switches/{id}/lag.
func (h *Handlers) GetLAG(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snap, err := h.Store.LatestSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap.LAGs)
}

// PatchPort handles PATCH /api/v1/switches/{id}/ports/{port}.
func (h *Handlers) PatchPort(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	portNum, err := strconv.Atoi(chi.URLParam(r, "port"))
	if err != nil || portNum < 1 || portNum > 8 {
		writeError(w, http.StatusBadRequest, "invalid port number")
		return
	}
	var req struct {
		Enabled     *bool             `json:"enabled"`
		Speed       *models.PortSpeed `json:"speed"`
		FlowControl *bool             `json:"flow_control"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	snap, _ := h.Store.LatestSnapshot(r.Context(), id)
	cfg := models.PortConfig{Enabled: true}
	if snap != nil {
		for _, p := range snap.Ports {
			if p.Number == portNum {
				cfg.Enabled = p.Enabled
				cfg.Speed = p.Speed
				cfg.FlowControl = p.FlowControl
				break
			}
		}
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.Speed != nil {
		cfg.Speed = *req.Speed
	}
	if req.FlowControl != nil {
		cfg.FlowControl = *req.FlowControl
	}

	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.SetPort(r.Context(), portNum, cfg); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ResetStats handles POST /api/v1/switches/{id}/stats/reset.
func (h *Handlers) ResetStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.ResetPortCounters(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PatchVLANs handles PATCH /api/v1/switches/{id}/vlans.
func (h *Handlers) PatchVLANs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var vlans []models.VLAN
	if err := json.NewDecoder(r.Body).Decode(&vlans); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.SetVLANs(r.Context(), vlans); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PatchMirror handles PATCH /api/v1/switches/{id}/mirror.
func (h *Handlers) PatchMirror(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var m models.PortMirror
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.SetPortMirror(r.Context(), m); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PatchQoS handles PATCH /api/v1/switches/{id}/qos.
func (h *Handlers) PatchQoS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var qos models.QoSStatus
	if err := json.NewDecoder(r.Body).Decode(&qos); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.SetQoS(r.Context(), qos); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PatchStormControl handles PATCH /api/v1/switches/{id}/storm-control.
func (h *Handlers) PatchStormControl(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sc []models.StormControl
	if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.SetStormControl(r.Context(), sc); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PatchLoopPrevention handles PATCH /api/v1/switches/{id}/loop-prevention.
func (h *Handlers) PatchLoopPrevention(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.SetLoopPrevention(r.Context(), body.Enabled); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PatchIGMP handles PATCH /api/v1/switches/{id}/igmp.
func (h *Handlers) PatchIGMP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.SetIGMP(r.Context(), body.Enabled); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PatchLAG handles PATCH /api/v1/switches/{id}/lag.
func (h *Handlers) PatchLAG(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var groups []models.LAGGroup
	if err := json.NewDecoder(r.Body).Decode(&groups); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.SetLAG(r.Context(), groups); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Reboot handles POST /api/v1/switches/{id}/reboot.
func (h *Handlers) Reboot(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	client, err := h.Manager.GetClient(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := client.Reboot(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rebooting"})
}
