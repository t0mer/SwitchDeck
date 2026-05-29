package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/t0mer/SwitchDeck/internal/manager"
)

// Handlers holds shared dependencies for all HTTP handlers.
type Handlers struct {
	Manager *manager.Manager
}

// New creates a Handlers instance.
func New(mgr *manager.Manager) *Handlers {
	return &Handlers{Manager: mgr}
}

// HealthCheck handles GET /health.
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ListSwitches handles GET /api/v1/switches.
func (h *Handlers) ListSwitches(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// GetSwitch handles GET /api/v1/switches/{id}.
func (h *Handlers) GetSwitch(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
