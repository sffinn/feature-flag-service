package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/feature-flag-service/feature-flag-service/internal/flag"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store implements flag.Store using Postgres.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateGlobal(ctx context.Context, f flag.GlobalFlag) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO global_flags (flagname, enabled) VALUES ($1, $2)`,
		f.FlagName, f.Enabled,
	)
	return mapWriteErr(err)
}

func (s *Store) CreateUser(ctx context.Context, f flag.UserFlag) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_flags (username, flagname, enabled) VALUES ($1, $2, $3)`,
		f.Username, f.FlagName, f.Enabled,
	)
	return mapWriteErr(err)
}

func (s *Store) UpdateGlobal(ctx context.Context, f flag.GlobalFlag) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE global_flags SET enabled = $1 WHERE flagname = $2`,
		f.Enabled, f.FlagName,
	)
	return mapWriteErr(err)
}

func (s *Store) UpdateUser(ctx context.Context, f flag.UserFlag) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE user_flags SET enabled = $1 WHERE username = $2 AND flagname = $3`,
		f.Enabled, f.Username, f.FlagName,
	)
	return mapWriteErr(err)
}

func (s *Store) GetGlobal(ctx context.Context, flagName string) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx,
		`SELECT enabled FROM global_flags WHERE flagname = $1`,
		flagName,
	).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, flag.ErrFlagNotFound
	}
	return enabled, err
}

func (s *Store) GetUser(ctx context.Context, username, flagName string) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx,
		`SELECT enabled FROM user_flags WHERE username = $1 AND flagname = $2`,
		username, flagName,
	).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, flag.ErrFlagNotFound
	}
	return enabled, err
}

func mapWriteErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return flag.ErrFlagExists
	}
	return err
}

// Migrate applies embedded SQL migrations using a simple version table.
func Migrate(ctx context.Context, pool *pgxpool.Pool, upSQL string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = 1)`).Scan(&exists); err != nil {
		return fmt.Errorf("check migration: %w", err)
	}
	if exists {
		return tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, upSQL); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES (1)`); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	return tx.Commit(ctx)
}
