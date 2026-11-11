// Package testsupport provides database-backed test helpers.
//
// It is imported only by tests. When TEST_DATABASE_URL is unset, every helper
// skips the test instead of failing, so `go test ./...` works on a machine with
// no database while CI runs the full suite.
package testsupport

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groupify/groupify/backend/internal/auth"
	"github.com/groupify/groupify/backend/internal/database"
	"github.com/groupify/groupify/backend/migrations"
)

var (
	migrateOnce sync.Once
	migrateErr  error
)

// DB returns a pool against TEST_DATABASE_URL with the schema migrated and all
// tables emptied. Each test therefore starts from a known state.
func DB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping database-backed test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	migrateOnce.Do(func() {
		_, migrateErr = database.Migrate(ctx, pool, migrations.FS)
	})
	if migrateErr != nil {
		pool.Close()
		t.Fatalf("migrate test database: %v", migrateErr)
	}

	if err := database.ResetForTests(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("reset test database: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}

// SeedTopic inserts a topic and returns its id.
func SeedTopic(t *testing.T, pool *pgxpool.Pool, name string, capacity int, active bool) int64 {
	t.Helper()

	var id int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO topics (name, description, capacity, is_active)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, name, "seeded topic", capacity, active).Scan(&id)
	if err != nil {
		t.Fatalf("seed topic: %v", err)
	}
	return id
}

// SetRegistrationOpen flips the global switch.
func SetRegistrationOpen(t *testing.T, pool *pgxpool.Pool, open bool) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`UPDATE app_settings SET registration_open = $1, updated_at = now() WHERE id = 1`, open)
	if err != nil {
		t.Fatalf("set registration open: %v", err)
	}
}

// SeedAdmin creates an admin with the given password and returns its id.
func SeedAdmin(t *testing.T, pool *pgxpool.Pool, email, password string) int64 {
	t.Helper()

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	var id int64
	err = pool.QueryRow(context.Background(), `
		INSERT INTO admins (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, hash).Scan(&id)
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	return id
}

// CountRegistrations returns the number of rows in registrations.
func CountRegistrations(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM registrations`).Scan(&n); err != nil {
		t.Fatalf("count registrations: %v", err)
	}
	return n
}

// TopicMembers returns topics.current_members for a topic.
func TopicMembers(t *testing.T, pool *pgxpool.Pool, topicID int64) int {
	t.Helper()

	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT current_members FROM topics WHERE id = $1`, topicID).Scan(&n); err != nil {
		t.Fatalf("read topic members: %v", err)
	}
	return n
}

// AuditActions returns the distinct audit action names recorded so far.
func AuditActions(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()

	rows, err := pool.Query(context.Background(), `SELECT DISTINCT action FROM audit_logs ORDER BY action`)
	if err != nil {
		t.Fatalf("read audit actions: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatalf("scan audit action: %v", err)
		}
		out = append(out, a)
	}
	return out
}

// DSN is the configured test DSN (empty when unset).
func DSN() string { return os.Getenv("TEST_DATABASE_URL") }

// RequireDSN fails loudly with a helpful message when the DSN is missing.
func RequireDSN(t *testing.T) string {
	t.Helper()
	dsn := DSN()
	if dsn == "" {
		t.Skip(fmt.Sprintf("TEST_DATABASE_URL is not set; run `make test-integration`"))
	}
	return dsn
}
