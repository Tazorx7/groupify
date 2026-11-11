// Command seed creates or updates the bootstrap administrator.
//
// There is no admin self-registration: an account exists only because an
// operator ran this command with credentials from the environment.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/groupify/groupify/backend/internal/auth"
	"github.com/groupify/groupify/backend/internal/config"
	"github.com/groupify/groupify/backend/internal/database"
	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	email := domain.NormalizeEmail(os.Getenv("ADMIN_EMAIL"))
	password := os.Getenv("ADMIN_PASSWORD")

	if email == "" {
		return errors.New("ADMIN_EMAIL is required")
	}
	if len(password) < auth.MinPasswordLength {
		return fmt.Errorf("ADMIN_PASSWORD must be at least %d characters", auth.MinPasswordLength)
	}
	if len(password) > auth.MaxPasswordLength {
		return fmt.Errorf("ADMIN_PASSWORD must be at most %d characters", auth.MaxPasswordLength)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Seeding a fresh install must not require a separate migration step.
	if cfg.AutoMigrate {
		if _, err := database.Migrate(ctx, pool, migrations.FS); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	var (
		id        int64
		createdAt time.Time
	)
	err = pool.QueryRow(ctx, `
		INSERT INTO admins (email, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (lower(email)) DO UPDATE
		SET password_hash = EXCLUDED.password_hash, updated_at = now()
		RETURNING id, created_at`, email, hash).Scan(&id, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("upsert admin returned no row")
		}
		return fmt.Errorf("upsert admin: %w", err)
	}

	fmt.Printf("admin ready: id=%d email=%s\n", id, email)
	return nil
}
