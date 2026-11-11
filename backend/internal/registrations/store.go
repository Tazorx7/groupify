package registrations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groupify/groupify/backend/internal/database"
	"github.com/groupify/groupify/backend/internal/domain"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// LockRegistrationOpen reads the global switch and holds a FOR SHARE lock on it
// for the remainder of tx. Every registration takes this lock, so a concurrent
// admin close (FOR UPDATE on the same row) cannot interleave: either the close
// waits for this transaction and commits after it, or this transaction waits,
// re-reads the committed `false`, and is rejected.
func (s *Store) LockRegistrationOpen(ctx context.Context, tx pgx.Tx) (bool, error) {
	var open bool
	err := tx.QueryRow(ctx, `
		SELECT registration_open FROM app_settings WHERE id = 1 FOR SHARE`).Scan(&open)
	if err != nil {
		return false, fmt.Errorf("lock registration status: %w", err)
	}
	return open, nil
}

// IndexNumberExists is a non-locking pre-check that only exists to produce a
// helpful error message. The UNIQUE constraint below is the real arbiter.
func (s *Store) IndexNumberExists(ctx context.Context, tx pgx.Tx, indexNumber string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM registrations WHERE index_number = $1)`,
		indexNumber).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check index number: %w", err)
	}
	return exists, nil
}

// Slot is the result of successfully claiming capacity.
type Slot struct {
	CurrentMembers int
	Capacity       int
	TopicName      string
}

var errNoSlot = errors.New("no capacity slot available")

// ClaimSlot performs the atomic conditional increment. This single statement is
// the entire capacity guarantee:
//
//	UPDATE ... WHERE id = $1 AND is_active AND current_members < capacity
//
// PostgreSQL serialises concurrent updates to the same row; a transaction that
// loses the race re-evaluates the predicate against the committed row and gets
// zero rows back. The caller MUST roll back if it cannot insert the matching
// registration, otherwise the slot is leaked.
func (s *Store) ClaimSlot(ctx context.Context, tx pgx.Tx, topicID int64) (*Slot, error) {
	var slot Slot
	err := tx.QueryRow(ctx, `
		UPDATE topics
		SET current_members = current_members + 1, updated_at = now()
		WHERE id = $1
		  AND is_active
		  AND current_members < capacity
		RETURNING current_members, capacity, name`,
		topicID).Scan(&slot.CurrentMembers, &slot.Capacity, &slot.TopicName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNoSlot
	}
	if err != nil {
		return nil, fmt.Errorf("claim capacity slot: %w", err)
	}
	return &slot, nil
}

// DiagnoseTopic explains why ClaimSlot found no row: missing, inactive or full.
func (s *Store) DiagnoseTopic(ctx context.Context, tx pgx.Tx, topicID int64) error {
	var (
		isActive bool
		members  int
		capacity int
	)
	err := tx.QueryRow(ctx, `
		SELECT is_active, current_members, capacity FROM topics WHERE id = $1`, topicID).
		Scan(&isActive, &members, &capacity)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTopicNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnose topic: %w", err)
	}
	if !isActive {
		return domain.ErrTopicInactive
	}
	return domain.ErrTopicFull
}

// Insert writes the registration. A unique violation here aborts the
// transaction, which rolls back the capacity increment from ClaimSlot.
func (s *Store) Insert(ctx context.Context, tx pgx.Tx, fullName, indexNumber string, topicID int64) (*Registration, error) {
	var r Registration
	err := tx.QueryRow(ctx, `
		INSERT INTO registrations (full_name, index_number, topic_id)
		VALUES ($1, $2, $3)
		RETURNING id, full_name, index_number, topic_id, created_at`,
		fullName, indexNumber, topicID).
		Scan(&r.ID, &r.FullName, &r.IndexNumber, &r.TopicID, &r.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err, database.ConstraintIndexNumberUnique) {
			return nil, domain.ErrIndexNumberTaken
		}
		if database.IsForeignKeyViolation(err, "") {
			return nil, domain.ErrTopicNotFound
		}
		return nil, fmt.Errorf("insert registration: %w", err)
	}
	return &r, nil
}

// ClaimIdempotencyKey inserts the claim row. It reports false when the key is
// already in use; in that case the statement blocks until the competing
// transaction commits or rolls back, so the caller can safely read the finished
// record afterwards.
func (s *Store) ClaimIdempotencyKey(ctx context.Context, tx pgx.Tx, key, hash string) (bool, error) {
	var claimed string
	err := tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (key, request_hash)
		VALUES ($1, $2)
		ON CONFLICT (key) DO NOTHING
		RETURNING key`, key, hash).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim idempotency key: %w", err)
	}
	return claimed == key, nil
}

// CompleteIdempotencyKey stores the response so a retry can be served verbatim.
func (s *Store) CompleteIdempotencyKey(ctx context.Context, tx pgx.Tx, key string, status int, body []byte) error {
	_, err := tx.Exec(ctx, `
		UPDATE idempotency_keys SET response_status = $2, response_body = $3 WHERE key = $1`,
		key, status, body)
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	return nil
}

// IdempotencyRecord is a completed or in-flight idempotency claim.
type IdempotencyRecord struct {
	RequestHash  string
	Status       *int
	ResponseBody []byte
	Found        bool
}

// LoadIdempotencyKey reads a claim outside the registration transaction.
func (s *Store) LoadIdempotencyKey(ctx context.Context, key string) (*IdempotencyRecord, error) {
	var (
		rec    IdempotencyRecord
		status *int
		body   []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT request_hash, response_status, response_body
		FROM idempotency_keys WHERE key = $1`, key).
		Scan(&rec.RequestHash, &status, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return &IdempotencyRecord{Found: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load idempotency key: %w", err)
	}
	rec.Found = true
	rec.Status = status
	rec.ResponseBody = body
	return &rec, nil
}

// TopicIDForRegistration resolves a registration's topic without locking.
func (s *Store) TopicIDForRegistration(ctx context.Context, tx pgx.Tx, id int64) (int64, error) {
	var topicID int64
	err := tx.QueryRow(ctx, `SELECT topic_id FROM registrations WHERE id = $1`, id).Scan(&topicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrRegistrationNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("resolve registration topic: %w", err)
	}
	return topicID, nil
}

// LockTopic takes the topic row lock. Admin removals acquire this lock before
// touching the registration row, matching the lock order used by registration
// creation (topic first), which rules out a deadlock between the two paths.
func (s *Store) LockTopic(ctx context.Context, tx pgx.Tx, topicID int64) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM topics WHERE id = $1 FOR UPDATE`, topicID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTopicNotFound
	}
	if err != nil {
		return fmt.Errorf("lock topic: %w", err)
	}
	return nil
}

// Delete removes a registration row.
func (s *Store) Delete(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `DELETE FROM registrations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete registration: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRegistrationNotFound
	}
	return nil
}

// ReleaseSlot decrements the occupancy counter after an admin removes a
// registration. The guard makes a double-decrement impossible.
func (s *Store) ReleaseSlot(ctx context.Context, tx pgx.Tx, topicID int64) error {
	tag, err := tx.Exec(ctx, `
		UPDATE topics
		SET current_members = current_members - 1, updated_at = now()
		WHERE id = $1 AND current_members > 0`, topicID)
	if err != nil {
		return fmt.Errorf("release capacity slot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either the topic vanished (FK RESTRICT prevents that) or the counter
		// was already zero, which means the data is inconsistent. Fail loudly.
		return fmt.Errorf("release capacity slot: topic %d has no members to release", topicID)
	}
	return nil
}

// List returns a page of registrations plus the total matching count.
func (s *Store) List(ctx context.Context, f Filter) ([]AdminRegistration, int, error) {
	where, args := buildWhere(f)

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM registrations r JOIN topics t ON t.id = r.topic_id `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count registrations: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT r.id, r.full_name, r.index_number, r.topic_id, t.name, r.created_at
		FROM registrations r
		JOIN topics t ON t.id = r.topic_id
		%s
		ORDER BY %s %s, r.id %s
		LIMIT $%d OFFSET $%d`,
		where, orderColumn(f.SortBy), strings.ToUpper(f.SortDir), strings.ToUpper(f.SortDir),
		len(args)+1, len(args)+2)

	args = append(args, f.Page.Limit, f.Page.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list registrations: %w", err)
	}
	defer rows.Close()

	items := make([]AdminRegistration, 0, f.Page.Limit)
	for rows.Next() {
		var r Registration
		if err := rows.Scan(&r.ID, &r.FullName, &r.IndexNumber, &r.TopicID, &r.TopicName, &r.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, r.ToAdmin())
	}
	return items, total, rows.Err()
}

// ExportRows returns up to limit rows matching the filter, without pagination.
func (s *Store) ExportRows(ctx context.Context, f Filter, limit int) ([]AdminRegistration, error) {
	where, args := buildWhere(f)
	query := fmt.Sprintf(`
		SELECT r.id, r.full_name, r.index_number, r.topic_id, t.name, r.created_at
		FROM registrations r
		JOIN topics t ON t.id = r.topic_id
		%s
		ORDER BY %s %s, r.id %s
		LIMIT %d`,
		where, orderColumn(f.SortBy), strings.ToUpper(f.SortDir), strings.ToUpper(f.SortDir), limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("export registrations: %w", err)
	}
	defer rows.Close()

	items := make([]AdminRegistration, 0, limit)
	for rows.Next() {
		var r Registration
		if err := rows.Scan(&r.ID, &r.FullName, &r.IndexNumber, &r.TopicID, &r.TopicName, &r.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, r.ToAdmin())
	}
	return items, rows.Err()
}

// buildWhere produces a parameterised predicate. Values are always bound as
// parameters; only fixed SQL fragments are concatenated.
func buildWhere(f Filter) (string, []any) {
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if f.Search != "" {
		args = append(args, "%"+strings.ToLower(f.Search)+"%")
		clauses = append(clauses, fmt.Sprintf(
			`(lower(r.full_name) LIKE $%d OR r.index_number LIKE $%d)`,
			len(args), len(args)+1))
		args = append(args, "%"+domain.NormalizeIndexNumber(f.Search)+"%")
	}
	if f.TopicID > 0 {
		args = append(args, f.TopicID)
		clauses = append(clauses, fmt.Sprintf(`r.topic_id = $%d`, len(args)))
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// orderColumn maps a whitelisted sort key to a fixed SQL expression.
func orderColumn(sortBy string) string {
	switch sortBy {
	case "full_name":
		return "lower(r.full_name)"
	case "index_number":
		return "r.index_number"
	case "topic":
		return "lower(t.name)"
	default:
		return "r.created_at"
	}
}
