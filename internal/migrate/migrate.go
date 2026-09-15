package migrate

import (
	"context"
	"embed"
	"fmt"

	"github.com/feature-flag-service/feature-flag-service/internal/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// Up applies pending schema migrations.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	upSQL, err := sqlFS.ReadFile("sql/001_create_flags.up.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	return postgres.Migrate(ctx, pool, string(upSQL))
}
