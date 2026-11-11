// Command api runs the Groupify HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/groupify/groupify/backend/internal/admin"
	"github.com/groupify/groupify/backend/internal/audit"
	"github.com/groupify/groupify/backend/internal/auth"
	"github.com/groupify/groupify/backend/internal/config"
	"github.com/groupify/groupify/backend/internal/database"
	"github.com/groupify/groupify/backend/internal/httpapi"
	"github.com/groupify/groupify/backend/internal/middleware"
	"github.com/groupify/groupify/backend/internal/registrations"
	"github.com/groupify/groupify/backend/internal/settings"
	"github.com/groupify/groupify/backend/internal/static"
	"github.com/groupify/groupify/backend/internal/topics"
	"github.com/groupify/groupify/backend/migrations"
)

const (
	sessionCookieName = "groupify_admin"
	shutdownTimeout   = 10 * time.Second
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	setupLogger(cfg)

	if cfg.StaticDir != "" && !static.Exists(cfg.StaticDir) {
		slog.Error("STATIC_DIR does not contain index.html", "dir", cfg.StaticDir)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		applied, err := database.Migrate(ctx, pool, migrations.FS)
		if err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		if len(applied) > 0 {
			slog.Info("migrations applied", "count", len(applied), "versions", applied)
		}
	}

	sessionStore := auth.NewStore(pool)
	authMW := middleware.NewAuth(sessionStore, middleware.SessionCookie{
		Name:   sessionCookieName,
		TTL:    cfg.SessionTTL,
		Secure: cfg.CookieSecure,
		Domain: cfg.CookieDomain,
	})

	topicsSvc := topics.NewService(pool)
	settingsSvc := settings.NewService(pool)
	registrationsSvc := registrations.NewService(pool)
	adminSvc := admin.NewService(pool, sessionStore, cfg.SessionTTL)
	auditStore := audit.NewStore(pool)

	handler := httpapi.New(httpapi.Deps{
		Config:          cfg,
		Topics:          topics.NewHandler(topicsSvc),
		Registrations:   registrations.NewHandler(registrationsSvc),
		Settings:        settings.NewHandler(settingsSvc),
		Admin:           admin.NewHandler(adminSvc, authMW),
		Audit:           audit.NewHandler(auditStore),
		Auth:            authMW,
		LoginLimiter:    middleware.NewLimiter(cfg.RateLimitLoginPerMinute),
		RegisterLimiter: middleware.NewLimiter(cfg.RateLimitRegisterPerMinute),
		Ping:            pool.Ping,
		StaticDir:       cfg.StaticDir,
	})

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
		// Conservative timeouts: the API has no long-lived request types.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go cleanupSessions(ctx, sessionStore)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("stopped")
}

// setupLogger uses human-readable text in development and JSON elsewhere so
// production logs stay machine-parseable.
func setupLogger(cfg *config.Config) {
	level := slog.LevelInfo
	if cfg.Env == "dev" {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if cfg.Env == "dev" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h))
}

// cleanupSessions garbage-collects expired sessions hourly. It is best-effort:
// the Lookup query already treats expired rows as absent.
func cleanupSessions(ctx context.Context, store *auth.Store) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := store.DeleteExpired(ctx); err != nil {
				slog.Warn("session cleanup failed", "error", err)
			} else if n > 0 {
				slog.Info("expired sessions removed", "count", n)
			}
		}
	}
}
