package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/t0mer/SwitchDeck/internal/api/handlers"
	"github.com/t0mer/SwitchDeck/internal/config"
	"github.com/t0mer/SwitchDeck/internal/manager"
)

// Server is the HTTP server for SwitchDeck.
type Server struct {
	cfg    *config.Config
	router *chi.Mux
}

// New creates a Server wired up with routes.
func New(cfg *config.Config, mgr *manager.Manager) *Server {
	h := handlers.New(mgr)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.HealthCheck)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/switches", h.ListSwitches)
		r.Get("/switches/{id}", h.GetSwitch)
	})

	return &Server{cfg: cfg, router: r}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	return http.ListenAndServe(addr, s.router)
}
