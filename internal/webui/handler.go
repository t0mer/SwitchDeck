package webui

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

// UIHandler serves the frontend templates and static assets.
type UIHandler struct {
	tmplFS fs.FS
	static http.Handler
}

// NewHandler creates a UIHandler with embedded templates and static files.
func NewHandler() (*UIHandler, error) {
	tmplSub, err := fs.Sub(FS, "templates")
	if err != nil {
		return nil, err
	}
	staticSub, err := fs.Sub(FS, "static")
	if err != nil {
		return nil, err
	}
	return &UIHandler{
		tmplFS: tmplSub,
		static: http.FileServer(http.FS(staticSub)),
	}, nil
}

// ServeStatic handles GET /static/*.
func (h *UIHandler) ServeStatic(w http.ResponseWriter, r *http.Request) {
	r2 := r.Clone(r.Context())
	r2.URL.Path = strings.TrimPrefix(r2.URL.Path, "/static")
	if r2.URL.Path == "" {
		r2.URL.Path = "/"
	}
	h.static.ServeHTTP(w, r2)
}

// Dashboard handles GET /.
func (h *UIHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, "dashboard.html", nil)
}

// SwitchDetail handles GET /switches/{id}.
func (h *UIHandler) SwitchDetail(w http.ResponseWriter, r *http.Request) {
	// Extract switch ID from path /switches/{id}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	id := ""
	if len(parts) >= 2 {
		id = parts[1]
	}
	h.renderPage(w, "switch.html", map[string]string{"SwitchID": id})
}

// Settings handles GET /settings.
func (h *UIHandler) Settings(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, "settings.html", nil)
}

func (h *UIHandler) renderPage(w http.ResponseWriter, page string, data any) {
	tmpl, err := template.ParseFS(h.tmplFS, "layout.html", page)
	if err != nil {
		log.Printf("template parse error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("template execute error: %v", err)
	}
}
