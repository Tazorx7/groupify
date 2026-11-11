// Package database owns the connection pool, the transaction helper and the
// migration runner. Everything that talks to PostgreSQL goes through here so
// that transaction boundaries are explicit and uniform.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Open parses the DSN, builds a pool and verifies connectivity.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// Class-sized traffic: a handful of connections is plenty and keeps the
	// managed-Postgres connection budget honest.
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// InTx runs fn inside a transaction and commits only if fn returns nil.
// The rollback uses a context detached from cancellation so that a client
// disconnect still releases locks and slots.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// pgError unwraps a *pgconn.PgError from a pgx error.
func pgError(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr, true
	}
	return nil, false
}

// IsUniqueViolation reports whether err is a unique constraint violation,
// optionally limited to a named constraint.
func IsUniqueViolation(err error, constraint string) bool {
	pgErr, ok := pgError(err)
	if !ok || pgErr.Code != "23505" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// IsCheckViolation reports whether err is a CHECK constraint violation,
// optionally limited to a named constraint.
func IsCheckViolation(err error, constraint string) bool {
	pgErr, ok := pgError(err)
	if !ok || pgErr.Code != "23514" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// IsForeignKeyViolation reports whether err is a foreign key violation.
func IsForeignKeyViolation(err error, constraint string) bool {
	pgErr, ok := pgError(err)
	if !ok || pgErr.Code != "23503" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// Constraint names used by the domain logic.
const (
	ConstraintIndexNumberUnique = "registrations_index_number_unique"
	ConstraintTopicNameUnique   = "topics_name_unique_idx"
	ConstraintMembersCapacity   = "topics_members_within_capacity"
	ConstraintMembersNegative   = "topics_members_non_negative"
)

// ResetForTests truncates all application tables. Test-only helper; it is not
// reachable from the HTTP surface.
//
// app_settings must be listed explicitly: it has a foreign key to admins, so a
// CASCADE truncate of admins would silently remove the singleton row and every
// registration would then fail with "registration closed".
func ResetForTests(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		TRUNCATE registrations, topics, audit_logs, idempotency_keys, admin_sessions, admins, app_settings
		RESTART IDENTITY CASCADE`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO app_settings (id, registration_open) VALUES (1, false)
		ON CONFLICT (id) DO UPDATE
		SET registration_open = false, updated_by_admin_id = NULL, updated_at = now()`)
	return err
}
