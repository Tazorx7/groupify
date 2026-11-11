// Package admin handles administrator authentication and dashboard statistics.
package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groupify/groupify/backend/internal/audit"
	"github.com/groupify/groupify/backend/internal/auth"
	"github.com/groupify/groupify/backend/internal/domain"
)

type Service struct {
	pool     *pgxpool.Pool
	sessions *auth.Store
	ttl      time.Duration
}

func NewService(pool *pgxpool.Pool, sessions *auth.Store, ttl time.Duration) *Service {
	return &Service{pool: pool, sessions: sessions, ttl: ttl}
}

// Account is the authenticated admin identity (never includes the hash).
type Account struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

// LoginResult carries the session token for the cookie plus what the SPA needs
// to bootstrap itself.
type LoginResult struct {
	Account   Account
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

// Authenticate verifies credentials and issues a session.
//
// Unknown emails and wrong passwords take the same code path and the same
// amount of CPU (a dummy Argon2id verification), so response timing does not
// reveal which admin accounts exist.
func (s *Service) Authenticate(ctx context.Context, email, password, requestID string) (*LoginResult, error) {
	normalised := domain.NormalizeEmail(email)
	if normalised == "" || password == "" {
		return nil, domain.ErrInvalidCredentials
	}

	var (
		id   int64
		hash string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, password_hash FROM admins WHERE email = $1`, normalised).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		auth.VerifyDummy(password)
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("load admin: %w", err)
	}

	ok, err := auth.VerifyPassword(hash, password)
	if err != nil {
		// A corrupt hash is an operational problem, not a user error.
		slog.Error("admin password hash is unreadable", "admin_id", id, "error", err)
		return nil, domain.ErrInvalidCredentials
	}
	if !ok {
		return nil, domain.ErrInvalidCredentials
	}

	token, sess, err := s.sessions.Create(ctx, id, s.ttl)
	if err != nil {
		return nil, err
	}

	if err := audit.Write(ctx, s.pool, audit.Event{
		ActorAdminID: &id,
		Action:       audit.ActionAdminLogin,
		EntityType:   audit.EntityAdmin,
		EntityID:     fmt.Sprint(id),
		After:        map[string]any{"email": normalised},
		RequestID:    requestID,
	}); err != nil {
		// A failed audit write must not block a legitimate login, but it must
		// be visible to operators.
		slog.Warn("audit write failed for admin login", "admin_id", id, "error", err)
	}

	return &LoginResult{
		Account:   Account{ID: id, Email: normalised},
		Token:     token,
		CSRFToken: sess.CSRF,
		ExpiresAt: sess.ExpiresAt,
	}, nil
}

// Logout revokes the session and records the event.
func (s *Service) Logout(ctx context.Context, token string, adminID int64, requestID string) error {
	if err := s.sessions.Delete(ctx, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	_ = audit.Write(ctx, s.pool, audit.Event{
		ActorAdminID: &adminID,
		Action:       audit.ActionAdminLogout,
		EntityType:   audit.EntityAdmin,
		EntityID:     fmt.Sprint(adminID),
		RequestID:    requestID,
	})
	return nil
}

// Overview is the dashboard summary required by PRD §7.
type Overview struct {
	TotalRegistrations   int  `json:"total_registrations"`
	TotalTopics          int  `json:"total_topics"`
	ActiveTopics         int  `json:"active_topics"`
	FullTopics           int  `json:"full_topics"`
	TotalAvailableSpaces int  `json:"total_available_spaces"`
	RegistrationOpen     bool `json:"registration_open"`
}

// GetOverview computes all counters in one round trip.
func (s *Service) GetOverview(ctx context.Context) (*Overview, error) {
	var o Overview
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM registrations),
			(SELECT count(*) FROM topics),
			(SELECT count(*) FROM topics WHERE is_active),
			(SELECT count(*) FROM topics WHERE is_active AND current_members >= capacity),
			(SELECT coalesce(sum(capacity - current_members), 0) FROM topics WHERE is_active),
			(SELECT registration_open FROM app_settings WHERE id = 1)`).
		Scan(&o.TotalRegistrations, &o.TotalTopics, &o.ActiveTopics, &o.FullTopics,
			&o.TotalAvailableSpaces, &o.RegistrationOpen)
	if err != nil {
		return nil, fmt.Errorf("read overview: %w", err)
	}
	return &o, nil
}
