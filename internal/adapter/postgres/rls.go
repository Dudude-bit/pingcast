package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithUserScope sets the app.current_user_id session variable for RLS policies.
// Must be called at the start of each transaction that accesses user-scoped tables.
func WithUserScope(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_user_id = '%s'", userID.String()))
	return err
}

// BeginWithUserScope starts a transaction and sets the RLS user scope.
func BeginWithUserScope(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) (pgx.Tx, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	if err := WithUserScope(ctx, tx, userID); err != nil {
		tx.Rollback(ctx)
		return nil, fmt.Errorf("set user scope: %w", err)
	}
	return tx, nil
}
