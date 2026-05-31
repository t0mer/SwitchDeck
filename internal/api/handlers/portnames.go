package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ListPortNames handles GET /api/v1/switches/{id}/port-names.
// Returns a JSON object mapping port number (as string key) to display name.
// Ports without a custom name are not included; the caller should default to "Port N".
func (h *Handlers) ListPortNames(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	names, err := h.Store.ListPortNames(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list port names: "+err.Error())
		return
	}
	if names == nil {
		names = map[int]string{}
	}
	writeJSON(w, http.StatusOK, names)
}

// SetPortName handles PUT /api/v1/switches/{id}/port-names/{port}.
// Body: {"name": "Uplink"} — empty name deletes the override (reverts to "Port N").
func (h *Handlers) SetPortName(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	portStr := chi.URLParam(r, "port")
	portNum, err := strconv.Atoi(portStr)
	if err != nil || portNum < 1 {
		writeError(w, http.StatusBadRequest, "invalid port number")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" {
		if err := h.Store.DeletePortName(r.Context(), id, portNum); err != nil {
			writeError(w, http.StatusInternalServerError, "delete port name: "+err.Error())
			return
		}
	} else {
		if err := h.Store.SetPortName(r.Context(), id, portNum, req.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "set port name: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
