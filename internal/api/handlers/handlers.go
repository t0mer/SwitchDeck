package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/t0mer/SwitchDeck/internal/manager"
	"github.com/t0mer/SwitchDeck/internal/store"
)

// Handlers holds shared dependencies for all HTTP handlers.
type Handlers struct {
	Manager *manager.Manager
	Store   store.Store
	EncKey  []byte
}

// New creates a Handlers instance.
func New(mgr *manager.Manager, st store.Store, encKey []byte) *Handlers {
	return &Handlers{Manager: mgr, Store: st, EncKey: encKey}
}

// HealthCheck handles GET /health.
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
