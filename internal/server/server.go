package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/t0mer/SwitchDeck/internal/api/handlers"
	mw "github.com/t0mer/SwitchDeck/internal/api/middleware"
	"github.com/t0mer/SwitchDeck/internal/config"
	"github.com/t0mer/SwitchDeck/internal/store"
	"github.com/t0mer/SwitchDeck/internal/webui"
)

// Server is the HTTP server for SwitchDeck.
type Server struct {
	cfg    *config.Config
	router *chi.Mux
}

// New creates a Server with all routes wired up.
func New(cfg *config.Config, h *handlers.Handlers, st store.Store) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	ui, err := webui.NewHandler()
	if err != nil {
		log.Fatalf("webui: %v", err)
	}

	// ── Always public ───────────────────────────────────────────────────────
	r.Get("/static/*", ui.ServeStatic)
	r.Get("/health", h.HealthCheck)
	r.Get("/login", ui.Login)
	r.Post("/api/v1/auth/login", h.Login)
	r.Post("/api/v1/auth/logout", h.Logout)
	r.Get("/api/v1/auth/session", h.Session)

	// ── UI routes (redirect to /login on auth failure) ─────────────────────
	r.Group(func(r chi.Router) {
		r.Use(mw.AuthUI(st))
		r.Get("/", ui.Dashboard)
		r.Get("/switches/{id}", ui.SwitchDetail)
		r.Get("/settings", ui.Settings)
	})

	// ── API routes (401 JSON on auth failure) ──────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(mw.AuthAPI(st))

		r.Get("/switches", h.ListSwitches)
		r.Post("/switches", h.AddSwitch)
		r.Get("/switches/{id}", h.GetSwitch)
		r.Put("/switches/{id}", h.UpdateSwitch)
		r.Delete("/switches/{id}", h.DeleteSwitch)
		r.Post("/switches/{id}/collect", h.TriggerCollect)

		r.Get("/switches/{id}/snapshot", h.GetSnapshot)
		r.Get("/switches/{id}/ports", h.GetPorts)
		r.Get("/switches/{id}/stats", h.GetStats)
		r.Get("/switches/{id}/vlans", h.GetVLANs)
		r.Get("/switches/{id}/lag", h.GetLAG)

		r.Patch("/switches/{id}/ports/{port}", h.PatchPort)
		r.Post("/switches/{id}/stats/reset", h.ResetStats)
		r.Patch("/switches/{id}/vlans", h.PatchVLANs)
		r.Patch("/switches/{id}/mirror", h.PatchMirror)
		r.Patch("/switches/{id}/qos", h.PatchQoS)
		r.Patch("/switches/{id}/storm-control", h.PatchStormControl)
		r.Patch("/switches/{id}/loop-prevention", h.PatchLoopPrevention)
		r.Patch("/switches/{id}/igmp", h.PatchIGMP)
		r.Patch("/switches/{id}/lag", h.PatchLAG)
		r.Post("/switches/{id}/reboot", h.Reboot)

		r.Get("/settings", h.GetSettings)
		r.Put("/settings", h.UpdateSettings)
		r.Get("/settings/tokens", h.ListTokens)
		r.Post("/settings/tokens", h.CreateToken)
		r.Delete("/settings/tokens/{id}", h.DeleteToken)

		r.Get("/notifications", h.ListNotifications)
		r.Post("/notifications", h.CreateNotification)
		r.Post("/notifications/test", h.TestNotification)
		r.Put("/notifications/{id}", h.UpdateNotification)
		r.Delete("/notifications/{id}", h.DeleteNotification)
	})

	return &Server{cfg: cfg, router: r}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	return http.ListenAndServe(addr, s.router)
}
