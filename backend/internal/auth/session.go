package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session is an authenticated admin request context.
type Session struct {
	AdminID   int64
	Email     string
	CSRF      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store persists sessions. Only the SHA-256 of a token is stored: the raw token
// exists solely in the browser's HttpOnly cookie, so a database read cannot
// impersonate an admin.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const tokenBytes = 32

// Create issues a new session for adminID and returns the raw token (to be put
// in a cookie) plus the session it created.
func (s *Store) Create(ctx context.Context, adminID int64, ttl time.Duration) (string, *Session, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	csrf, err := newCSRFToken()
	if err != nil {
		return "", nil, err
	}

	expires := time.Now().Add(ttl)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO admin_sessions (token_hash, admin_id, csrf_token, expires_at)
		VALUES ($1, $2, $3, $4)`,
		HashToken(token), adminID, csrf, expires)
	if err != nil {
		return "", nil, fmt.Errorf("insert session: %w", err)
	}

	return token, &Session{AdminID: adminID, CSRF: csrf, ExpiresAt: expires}, nil
}

// Lookup resolves a raw token to a live session. Expired or unknown tokens
// return (nil, nil) rather than an error: "not signed in" is a normal state.
func (s *Store) Lookup(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, nil
	}
	var sess Session
	err := s.pool.QueryRow(ctx, `
		SELECT a.id, a.email, s.csrf_token, s.created_at, s.expires_at
		FROM admin_sessions s
		JOIN admins a ON a.id = s.admin_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`,
		HashToken(token)).Scan(&sess.AdminID, &sess.Email, &sess.CSRF, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup session: %w", err)
	}
	return &sess, nil
}

// Touch extends a session's lifetime (sliding expiration). The write is
// throttled to at most once a minute per session so read-heavy dashboards do
// not generate a pointless UPDATE per request.
func (s *Store) Touch(ctx context.Context, token string, ttl time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE admin_sessions
		SET last_seen_at = now(), expires_at = now() + $2::interval
		WHERE token_hash = $1 AND last_seen_at < now() - interval '1 minute'`,
		HashToken(token), ttl.String())
	return err
}

// Delete revokes a single session.
func (s *Store) Delete(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token_hash = $1`, HashToken(token))
	return err
}

// DeleteExpired garbage-collects stale sessions; called opportunistically.
func (s *Store) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// HashToken returns the storage form of a session token.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func newCSRFToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
