package topics

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groupify/groupify/backend/internal/audit"
	"github.com/groupify/groupify/backend/internal/database"
	"github.com/groupify/groupify/backend/internal/domain"
)

// Service holds topic business rules. Handlers call it with primitives; it owns
// transactions, validation and audit events.
type Service struct {
	pool  *pgxpool.Pool
	store *Store
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, store: NewStore(pool)}
}

// ListPublic returns the student-facing topic list.
func (s *Service) ListPublic(ctx context.Context) ([]PublicTopic, error) {
	rows, err := s.store.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PublicTopic, 0, len(rows))
	for _, t := range rows {
		out = append(out, t.ToPublic())
	}
	return out, nil
}

// ListAdmin returns every topic with management details.
func (s *Service) ListAdmin(ctx context.Context) ([]AdminTopic, error) {
	rows, err := s.store.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AdminTopic, 0, len(rows))
	for _, t := range rows {
		out = append(out, t.ToAdmin())
	}
	return out, nil
}

// Get returns a single topic for the admin UI.
func (s *Service) Get(ctx context.Context, id int64) (*AdminTopic, error) {
	t, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	a := t.ToAdmin()
	return &a, nil
}

// Create validates and inserts a topic.
func (s *Service) Create(ctx context.Context, adminID int64, req CreateRequest, requestID string) (*AdminTopic, error) {
	name, desc, verr := domain.ValidateTopicInput(req.Name, req.Description, req.Capacity)
	if verr != nil {
		return nil, verr
	}

	var created *Topic
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := s.store.Insert(ctx, tx, name, desc, req.Capacity)
		if err != nil {
			return err
		}
		created = t

		return audit.WriteTx(ctx, tx, audit.Event{
			ActorAdminID: &adminID,
			Action:       audit.ActionTopicCreated,
			EntityType:   audit.EntityTopic,
			EntityID:     fmt.Sprint(t.ID),
			After:        snapshot(t),
			RequestID:    requestID,
		})
	})
	if err != nil {
		return nil, err
	}

	a := created.ToAdmin()
	return &a, nil
}

// Update applies a partial change to a topic. Capacity may never drop below the
// number of students already registered (PRD §7); the CHECK constraint backs
// this up at the storage layer.
func (s *Service) Update(ctx context.Context, adminID, id int64, req UpdateRequest, requestID string) (*AdminTopic, error) {
	var updated *Topic

	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		current, err := s.store.GetForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}

		merged := *current
		if req.Name != nil {
			merged.Name = *req.Name
		}
		if req.Description != nil {
			merged.Description = *req.Description
		}
		if req.Capacity != nil {
			merged.Capacity = *req.Capacity
		}

		name, desc, verr := domain.ValidateTopicInput(merged.Name, merged.Description, merged.Capacity)
		if verr != nil {
			return verr
		}
		merged.Name, merged.Description = name, desc

		if merged.Capacity < current.CurrentMembers {
			return domain.ErrCapacityBelowMembers
		}

		// Nothing to do: avoid noisy audit rows for a no-op PATCH.
		if merged.Name == current.Name &&
			merged.Description == current.Description &&
			merged.Capacity == current.Capacity {
			updated = current
			return nil
		}

		t, err := s.store.Update(ctx, tx, &merged)
		if err != nil {
			return err
		}
		updated = t

		if err := audit.WriteTx(ctx, tx, audit.Event{
			ActorAdminID: &adminID,
			Action:       audit.ActionTopicUpdated,
			EntityType:   audit.EntityTopic,
			EntityID:     fmt.Sprint(t.ID),
			Before:       snapshot(current),
			After:        snapshot(t),
			RequestID:    requestID,
		}); err != nil {
			return err
		}

		if t.Capacity != current.Capacity {
			if err := audit.WriteTx(ctx, tx, audit.Event{
				ActorAdminID: &adminID,
				Action:       audit.ActionTopicCapacityChanged,
				EntityType:   audit.EntityTopic,
				EntityID:     fmt.Sprint(t.ID),
				Before:       map[string]any{"capacity": current.Capacity},
				After:        map[string]any{"capacity": t.Capacity},
				RequestID:    requestID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	a := updated.ToAdmin()
	return &a, nil
}

// SetActive activates or deactivates a topic. Existing registrations survive
// deactivation; the topic simply stops accepting new ones.
func (s *Service) SetActive(ctx context.Context, adminID, id int64, active bool, requestID string) (*AdminTopic, error) {
	var updated *Topic

	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		current, err := s.store.GetForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if current.IsActive == active {
			updated = current
			return nil
		}

		t, err := s.store.SetActive(ctx, tx, id, active)
		if err != nil {
			return err
		}
		updated = t

		action := audit.ActionTopicDeactivated
		if active {
			action = audit.ActionTopicActivated
		}
		return audit.WriteTx(ctx, tx, audit.Event{
			ActorAdminID: &adminID,
			Action:       action,
			EntityType:   audit.EntityTopic,
			EntityID:     fmt.Sprint(t.ID),
			Before:       map[string]any{"is_active": current.IsActive},
			After:        map[string]any{"is_active": t.IsActive},
			RequestID:    requestID,
		})
	})
	if err != nil {
		return nil, err
	}

	a := updated.ToAdmin()
	return &a, nil
}

func snapshot(t *Topic) map[string]any {
	return map[string]any{
		"name":             t.Name,
		"description":      t.Description,
		"capacity":         t.Capacity,
		"registered_count": t.CurrentMembers,
		"is_active":        t.IsActive,
	}
}
