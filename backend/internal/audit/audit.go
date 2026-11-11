// Package audit records who changed what, when, and from what previous state.
// Writes that accompany a mutation MUST run inside the same transaction as the
// mutation, so an audit trail can never disagree with the data.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Action names. Stable strings; the admin UI displays them as-is.
const (
	ActionRegistrationCreated = "registration.created"
	ActionRegistrationDeleted = "registration.deleted"

	ActionRegistrationOpened = "registration.period.opened"
	ActionRegistrationClosed = "registration.period.closed"

	ActionTopicCreated         = "topic.created"
	ActionTopicUpdated         = "topic.updated"
	ActionTopicActivated       = "topic.activated"
	ActionTopicDeactivated     = "topic.deactivated"
	ActionTopicCapacityChanged = "topic.capacity_changed"

	ActionAdminLogin  = "admin.login"
	ActionAdminLogout = "admin.logout"
)

// Entity types.
const (
	EntityRegistration = "registration"
	EntityTopic        = "topic"
	EntitySettings     = "app_settings"
	EntityAdmin        = "admin"
)

// Event is a single audit entry.
type Event struct {
	// ActorAdminID is nil for system/student events.
	ActorAdminID *int64
	// ActorLabel is a human hint such as "system" or "student". Never contains
	// student identifiers.
	ActorLabel string
	Action     string
	EntityType string
	EntityID   string
	Before     any
	After      any
	RequestID  string
}

// WriteTx inserts an event inside an existing transaction.
func WriteTx(ctx context.Context, tx pgx.Tx, e Event) error {
	before, err := marshal(e.Before)
	if err != nil {
		return fmt.Errorf("marshal audit before_data: %w", err)
	}
	after, err := marshal(e.After)
	if err != nil {
		return fmt.Errorf("marshal audit after_data: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_admin_id, actor_label, action, entity_type, entity_id, before_data, after_data, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ActorAdminID, e.ActorLabel, e.Action, e.EntityType, e.EntityID, before, after, nullString(e.RequestID))
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// Write inserts an event in its own statement (used for events that have no
// accompanying mutation, such as a login).
func Write(ctx context.Context, pool *pgxpool.Pool, e Event) error {
	before, err := marshal(e.Before)
	if err != nil {
		return fmt.Errorf("marshal audit before_data: %w", err)
	}
	after, err := marshal(e.After)
	if err != nil {
		return fmt.Errorf("marshal audit after_data: %w", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_admin_id, actor_label, action, entity_type, entity_id, before_data, after_data, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ActorAdminID, e.ActorLabel, e.Action, e.EntityType, e.EntityID, before, after, nullString(e.RequestID))
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// Entry is a row returned by the audit log query.
type Entry struct {
	ID           int64           `json:"id"`
	ActorAdminID *int64          `json:"actor_admin_id"`
	ActorEmail   *string         `json:"actor_email"`
	ActorLabel   string          `json:"actor_label"`
	Action       string          `json:"action"`
	EntityType   string          `json:"entity_type"`
	EntityID     string          `json:"entity_id"`
	Before       json.RawMessage `json:"before_data"`
	After        json.RawMessage `json:"after_data"`
	CreatedAt    string          `json:"created_at"`
}

// Query filters the audit log.
type Query struct {
	Action     string
	EntityType string
	Limit      int
	Offset     int
}

// Store reads the audit log for the admin UI.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// List returns audit entries newest first.
func (s *Store) List(ctx context.Context, q Query) ([]Entry, int, error) {
	// Fixed predicate list keeps the query parameterised while allowing
	// optional filters.
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.actor_admin_id, ad.email, a.actor_label, a.action,
		       a.entity_type, a.entity_id, a.before_data, a.after_data, a.created_at
		FROM audit_logs a
		LEFT JOIN admins ad ON ad.id = a.actor_admin_id
		WHERE ($1 = '' OR a.action = $1)
		  AND ($2 = '' OR a.entity_type = $2)
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $3 OFFSET $4`,
		q.Action, q.EntityType, q.Limit, q.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	entries := make([]Entry, 0, q.Limit)
	for rows.Next() {
		var (
			e         Entry
			before    []byte
			after     []byte
			createdAt time.Time
		)
		if err := rows.Scan(&e.ID, &e.ActorAdminID, &e.ActorEmail, &e.ActorLabel, &e.Action,
			&e.EntityType, &e.EntityID, &before, &after, &createdAt); err != nil {
			return nil, 0, err
		}
		e.Before = rawJSON(before)
		e.After = rawJSON(after)
		e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_logs
		WHERE ($1 = '' OR action = $1) AND ($2 = '' OR entity_type = $2)`,
		q.Action, q.EntityType).Scan(&total); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func marshal(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func rawJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}
