package registrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groupify/groupify/backend/internal/audit"
	"github.com/groupify/groupify/backend/internal/database"
	"github.com/groupify/groupify/backend/internal/domain"
)

const statusCreated = 201

// Service implements the registration rules. The correctness of the whole
// product lives in Register; read it top to bottom before changing anything.
type Service struct {
	pool  *pgxpool.Pool
	store *Store
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, store: NewStore(pool)}
}

// errIdempotencyReplay signals that another transaction owns the idempotency
// key; the caller re-reads the committed record outside the aborted transaction.
var errIdempotencyReplay = errors.New("idempotency record already exists")

// Register reserves one capacity slot and creates exactly one registration.
//
// Invariants upheld inside a single transaction:
//  1. registration is open (FOR SHARE on app_settings; conflicts with admin close)
//  2. index number is not already taken (pre-check for a clear message)
//  3. topic is active and has capacity (conditional UPDATE ... RETURNING)
//  4. the registration row is inserted
//
// Any error at any step rolls the transaction back, so a failed submission can
// never consume a slot. The database CHECK constraints are the final defence.
func (s *Service) Register(ctx context.Context, req CreateRequest, idempotencyKey, requestID string) (*Result, error) {
	fullName, indexNumber, verr := domain.ValidateStudentInput(req.FullName, req.IndexNumber)
	if verr != nil {
		return nil, verr
	}
	if req.TopicID <= 0 {
		return nil, domain.ValidationError("Please choose a topic.",
			map[string]string{"topic_id": "Please choose a topic."})
	}
	if idempotencyKey != "" && (len(idempotencyKey) < 8 || len(idempotencyKey) > 128) {
		return nil, domain.ValidationError("Idempotency key is not valid.",
			map[string]string{"Idempotency-Key": "Must be between 8 and 128 characters."})
	}

	hash := requestHash(fullName, indexNumber, req.TopicID)
	var result *Result

	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		// Claim the idempotency key first: a concurrent duplicate blocks here
		// until this transaction resolves, so it cannot double-register.
		if idempotencyKey != "" {
			claimed, err := s.store.ClaimIdempotencyKey(ctx, tx, idempotencyKey, hash)
			if err != nil {
				return err
			}
			if !claimed {
				return errIdempotencyReplay
			}
		}

		open, err := s.store.LockRegistrationOpen(ctx, tx)
		if err != nil {
			return err
		}
		if !open {
			return domain.ErrRegistrationClosed
		}

		taken, err := s.store.IndexNumberExists(ctx, tx, indexNumber)
		if err != nil {
			return err
		}
		if taken {
			return domain.ErrIndexNumberTaken
		}

		slot, err := s.store.ClaimSlot(ctx, tx, req.TopicID)
		if errors.Is(err, errNoSlot) {
			return s.store.DiagnoseTopic(ctx, tx, req.TopicID)
		}
		if err != nil {
			return err
		}

		reg, err := s.store.Insert(ctx, tx, fullName, indexNumber, req.TopicID)
		if err != nil {
			return err
		}
		reg.TopicName = slot.TopicName

		confirmation := reg.ToConfirmation()
		body, err := json.Marshal(confirmation)
		if err != nil {
			return fmt.Errorf("marshal confirmation: %w", err)
		}

		if idempotencyKey != "" {
			if err := s.store.CompleteIdempotencyKey(ctx, tx, idempotencyKey, statusCreated, body); err != nil {
				return err
			}
		}

		// Audit without personal data: the record id and topic are enough to
		// reconstruct what happened, and index numbers stay out of the log.
		if err := audit.WriteTx(ctx, tx, audit.Event{
			ActorLabel: "student",
			Action:     audit.ActionRegistrationCreated,
			EntityType: audit.EntityRegistration,
			EntityID:   fmt.Sprint(reg.ID),
			After: map[string]any{
				"registration_id": reg.ID,
				"topic_id":        reg.TopicID,
				"topic_name":      reg.TopicName,
			},
			RequestID: requestID,
		}); err != nil {
			return err
		}

		result = &Result{Confirmation: &confirmation, Status: statusCreated}
		return nil
	})

	if errors.Is(err, errIdempotencyReplay) {
		return s.replay(ctx, idempotencyKey, hash)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// replay returns the response stored by the original request.
func (s *Service) replay(ctx context.Context, key, hash string) (*Result, error) {
	rec, err := s.store.LoadIdempotencyKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if !rec.Found || rec.Status == nil || len(rec.ResponseBody) == 0 {
		return nil, domain.ErrIdempotencyInProgress
	}
	if rec.RequestHash != hash {
		return nil, domain.ErrIdempotencyReuse
	}

	var confirmation Confirmation
	if err := json.Unmarshal(rec.ResponseBody, &confirmation); err != nil {
		return nil, fmt.Errorf("decode idempotent response: %w", err)
	}
	return &Result{Confirmation: &confirmation, Status: *rec.Status, Replayed: true}, nil
}

// List returns a page of registrations for the admin UI.
func (s *Service) List(ctx context.Context, f Filter) (domain.ListResult[AdminRegistration], error) {
	f = f.Normalize()
	items, total, err := s.store.List(ctx, f)
	if err != nil {
		return domain.ListResult[AdminRegistration]{}, err
	}
	return domain.NewListResult(items, total, f.Page), nil
}

// ExportRows returns registrations for CSV export, hard-capped so a single
// request cannot exhaust memory.
func (s *Service) ExportRows(ctx context.Context, f Filter) ([]AdminRegistration, error) {
	f = f.Normalize()
	return s.store.ExportRows(ctx, f, 10_000)
}

// Delete removes a registration on an administrator's authority and frees the
// capacity slot in the same transaction. This is the only sanctioned way to
// undo a confirmed selection; it is always audited.
func (s *Service) Delete(ctx context.Context, adminID, id int64, requestID string) error {
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		topicID, err := s.store.TopicIDForRegistration(ctx, tx, id)
		if err != nil {
			return err
		}
		// Topic lock first: same order as registration creation.
		if err := s.store.LockTopic(ctx, tx, topicID); err != nil {
			return err
		}
		if err := s.store.Delete(ctx, tx, id); err != nil {
			return err
		}
		if err := s.store.ReleaseSlot(ctx, tx, topicID); err != nil {
			return err
		}

		return audit.WriteTx(ctx, tx, audit.Event{
			ActorAdminID: &adminID,
			Action:       audit.ActionRegistrationDeleted,
			EntityType:   audit.EntityRegistration,
			EntityID:     fmt.Sprint(id),
			Before: map[string]any{
				"registration_id": id,
				"topic_id":        topicID,
			},
			RequestID: requestID,
		})
	})
}
