package topics

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groupify/groupify/backend/internal/database"
	"github.com/groupify/groupify/backend/internal/domain"
)

// Store holds every SQL statement touching the topics table.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const topicColumns = `id, name, description, capacity, current_members, is_active, created_at, updated_at`

func scanTopic(row pgx.Row) (*Topic, error) {
	var t Topic
	err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Capacity, &t.CurrentMembers, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListActive returns active topics with their occupancy. Exactly one query, and
// it never joins registrations: the public view cannot leak members even by
// accident.
func (s *Store) ListActive(ctx context.Context) ([]Topic, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+topicColumns+`
		FROM topics
		WHERE is_active
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list active topics: %w", err)
	}
	defer rows.Close()

	var out []Topic
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// ListAll returns every topic for the admin dashboard.
func (s *Store) ListAll(ctx context.Context) ([]Topic, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+topicColumns+`
		FROM topics
		ORDER BY is_active DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	out := make([]Topic, 0, 16)
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// Get loads a topic without locking.
func (s *Store) Get(ctx context.Context, id int64) (*Topic, error) {
	t, err := scanTopic(s.pool.QueryRow(ctx, `SELECT `+topicColumns+` FROM topics WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTopicNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get topic: %w", err)
	}
	return t, nil
}

// GetForUpdate locks the row for the duration of tx. Admin mutations use this
// so concurrent capacity edits are serialised against each other.
func (s *Store) GetForUpdate(ctx context.Context, tx pgx.Tx, id int64) (*Topic, error) {
	t, err := scanTopic(tx.QueryRow(ctx, `
		SELECT `+topicColumns+` FROM topics WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTopicNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock topic: %w", err)
	}
	return t, nil
}

// Insert creates a topic.
func (s *Store) Insert(ctx context.Context, tx pgx.Tx, name, description string, capacity int) (*Topic, error) {
	t, err := scanTopic(tx.QueryRow(ctx, `
		INSERT INTO topics (name, description, capacity)
		VALUES ($1, $2, $3)
		RETURNING `+topicColumns, name, description, capacity))
	if err != nil {
		if database.IsUniqueViolation(err, database.ConstraintTopicNameUnique) {
			return nil, domain.ErrTopicNameTaken
		}
		return nil, fmt.Errorf("insert topic: %w", err)
	}
	return t, nil
}

// Update writes the full mutable state of a locked topic.
func (s *Store) Update(ctx context.Context, tx pgx.Tx, t *Topic) (*Topic, error) {
	updated, err := scanTopic(tx.QueryRow(ctx, `
		UPDATE topics
		SET name = $2, description = $3, capacity = $4, updated_at = now()
		WHERE id = $1
		RETURNING `+topicColumns, t.ID, t.Name, t.Description, t.Capacity))
	if err != nil {
		if database.IsUniqueViolation(err, database.ConstraintTopicNameUnique) {
			return nil, domain.ErrTopicNameTaken
		}
		// The CHECK constraint is the last line of defence if an application
		// bug ever tries to shrink capacity below occupancy.
		if database.IsCheckViolation(err, database.ConstraintMembersCapacity) {
			return nil, domain.ErrCapacityBelowMembers
		}
		return nil, fmt.Errorf("update topic: %w", err)
	}
	return updated, nil
}

// SetActive activates or deactivates a locked topic.
func (s *Store) SetActive(ctx context.Context, tx pgx.Tx, id int64, active bool) (*Topic, error) {
	t, err := scanTopic(tx.QueryRow(ctx, `
		UPDATE topics SET is_active = $2, updated_at = now()
		WHERE id = $1
		RETURNING `+topicColumns, id, active))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTopicNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set topic active: %w", err)
	}
	return t, nil
}
