// Package settings owns the global registration open/close switch.
//
// The close operation is the other half of the concurrency contract described
// in registrations: registration transactions hold a FOR SHARE lock on the
// singleton settings row, and a close takes FOR UPDATE on the same row. That
// pair guarantees no registration can commit after a closure has taken effect,
// while a registration that committed before closure stays valid.
package settings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groupify/groupify/backend/internal/audit"
	"github.com/groupify/groupify/backend/internal/database"
)

// State is the registration switch as exposed to clients.
type State struct {
	RegistrationOpen bool      `json:"registration_open"`
	UpdatedAt        time.Time `json:"updated_at"`
	UpdatedByAdminID *int64    `json:"updated_by_admin_id,omitempty"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Get reads the current state without locking.
func (s *Service) Get(ctx context.Context) (*State, error) {
	var st State
	err := s.pool.QueryRow(ctx, `
		SELECT registration_open, updated_at, updated_by_admin_id
		FROM app_settings WHERE id = 1`).
		Scan(&st.RegistrationOpen, &st.UpdatedAt, &st.UpdatedByAdminID)
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	return &st, nil
}

// SetOpen opens or closes registration. Existing registrations are unaffected.
func (s *Service) SetOpen(ctx context.Context, adminID int64, open bool, requestID string) (*State, error) {
	var st State

	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var current bool
		err := tx.QueryRow(ctx, `SELECT registration_open FROM app_settings WHERE id = 1 FOR UPDATE`).Scan(&current)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("lock settings: %w", err)
		}

		if err == nil && current == open {
			// No-op: return the current state without a misleading audit row.
			return tx.QueryRow(ctx, `
				SELECT registration_open, updated_at, updated_by_admin_id
				FROM app_settings WHERE id = 1`).
				Scan(&st.RegistrationOpen, &st.UpdatedAt, &st.UpdatedByAdminID)
		}

		if err := tx.QueryRow(ctx, `
			UPDATE app_settings
			SET registration_open = $1, updated_at = now(), updated_by_admin_id = $2
			WHERE id = 1
			RETURNING registration_open, updated_at, updated_by_admin_id`,
			open, adminID).Scan(&st.RegistrationOpen, &st.UpdatedAt, &st.UpdatedByAdminID); err != nil {
			return fmt.Errorf("update settings: %w", err)
		}

		action := audit.ActionRegistrationClosed
		if open {
			action = audit.ActionRegistrationOpened
		}
		return audit.WriteTx(ctx, tx, audit.Event{
			ActorAdminID: &adminID,
			Action:       action,
			EntityType:   audit.EntitySettings,
			EntityID:     "1",
			Before:       map[string]any{"registration_open": current},
			After:        map[string]any{"registration_open": open},
			RequestID:    requestID,
		})
	})
	if err != nil {
		return nil, err
	}
	return &st, nil
}
