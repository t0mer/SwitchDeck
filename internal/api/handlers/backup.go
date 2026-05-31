package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/t0mer/SwitchDeck/internal/backup"
)

// ExportBackup handles GET /api/v1/backup.
// It exports all switches, settings, API tokens, and notification channels
// as a downloadable JSON file. Credentials are included in plaintext.
func (h *Handlers) ExportBackup(w http.ResponseWriter, r *http.Request) {
	f, err := backup.Export(r.Context(), h.Store.DB(), h.Store, h.EncKey, h.NotifStore)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export failed: "+err.Error())
		return
	}

	data, err := backup.Marshal(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode failed: "+err.Error())
		return
	}

	filename := fmt.Sprintf("switchdeck-backup-%s.json", time.Now().UTC().Format("2006-01-02T150405Z"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// RestoreBackup handles POST /api/v1/backup/restore.
// It accepts a backup JSON file (multipart field "file" or raw request body),
// wipes the current configuration, and restores from the backup.
func (h *Handlers) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	var data []byte

	ct := r.Header.Get("Content-Type")
	if len(ct) >= 19 && ct[:19] == "multipart/form-data" {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "parse form: "+err.Error())
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing 'file' field: "+err.Error())
			return
		}
		defer file.Close()
		data, err = io.ReadAll(io.LimitReader(file, 8<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "read file: "+err.Error())
			return
		}
	} else {
		var err error
		data, err = io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "read body: "+err.Error())
			return
		}
	}

	f, err := backup.Unmarshal(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup file: "+err.Error())
		return
	}

	if err := backup.Restore(r.Context(), h.Store.DB(), h.Store, h.EncKey, h.NotifStore, f); err != nil {
		writeError(w, http.StatusInternalServerError, "restore failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
