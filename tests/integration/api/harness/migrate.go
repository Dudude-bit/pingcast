//go:build integration

package harness

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kirillinakin/pingcast/internal/database"
)

// RunMigrations applies all embedded migrations using the production
// migration code path.
func RunMigrations(ctx context.Context, pgURL string) error {
	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
