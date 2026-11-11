// Package httpapi wires the HTTP surface together. Every route and its
// middleware stack is declared in exactly one place so the authorisation rules
// are auditable at a glance.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/groupify/groupify/backend/internal/admin"
	"github.com/groupify/groupify/backend/internal/audit"
	"github.com/groupify/groupify/backend/internal/config"
	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
	"github.com/groupify/groupify/backend/internal/middleware"
	"github.com/groupify/groupify/backend/internal/registrations"
	"github.com/groupify/groupify/backend/internal/settings"
	"github.com/groupify/groupify/backend/internal/topics"
)

// Deps is everything the router needs. Passing handlers in (instead of
// constructing them here) keeps the router free of database knowledge.
type Deps struct {
	Config          *config.Config
	Topics          *topics.Handler
	Registrations   *registrations.Handler
	Settings        *settings.Handler
	Admin           *admin.Handler
	Audit           *audit.Handler
	Auth            *middleware.Auth
	LoginLimiter    *middleware.Limiter
	RegisterLimiter *middleware.Limiter
	Ping            func(ctx context.Context) error
}

// New builds the complete HTTP handler.
func New(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	if d.Config.TrustProxyHeaders {
		r.Use(chimw.RealIP)
	}
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(d.Config.CORSOrigins))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Fail(w, r, domain.ErrNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Fail(w, r, domain.ErrMethodNotAllowed)
	})

	// Liveness: the process is up. Readiness: the process can reach the
	// database and serve traffic.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Ping(ctx); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/api", func(r chi.Router) {
		// ---- Public / student surface -------------------------------------
		// Only these three endpoints exist without authentication, and none of
		// them can return student data beyond an aggregate count.
		r.Get("/topics", d.Topics.ListPublic)
		r.Get("/registration-status", d.Settings.PublicStatus)
		r.With(d.RegisterLimiter.Middleware).Post("/registrations", d.Registrations.Create)

		// ---- Admin surface ------------------------------------------------
		r.Route("/admin", func(r chi.Router) {
			r.With(d.LoginLimiter.Middleware).Post("/login", d.Admin.Login)

			r.Group(func(r chi.Router) {
				r.Use(d.Auth.RequireAdmin)

				// Reads: authenticated, no CSRF token required.
				r.Get("/session", d.Admin.CurrentSession)
				r.Get("/overview", d.Admin.Overview)
				r.Get("/topics", d.Topics.ListAdmin)
				r.Get("/registrations", d.Registrations.List)
				r.Get("/registrations/export", d.Registrations.Export)
				r.Get("/settings", d.Settings.Get)
				r.Get("/audit-logs", d.Audit.List)

				// Writes: authenticated AND CSRF-verified.
				r.Group(func(r chi.Router) {
					r.Use(d.Auth.RequireCSRF)

					r.Post("/logout", d.Admin.Logout)

					r.Post("/topics", d.Topics.Create)
					r.Patch("/topics/{id}", d.Topics.Update)
					r.Post("/topics/{id}/activate", d.Topics.Activate)
					r.Post("/topics/{id}/deactivate", d.Topics.Deactivate)

					r.Delete("/registrations/{id}", d.Registrations.Delete)

					r.Patch("/settings", d.Settings.Update)
				})
			})
		})
	})

	return r
}
