package postgres

import (
	"context"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.TxManager = (*TxManager)(nil)

// TxManager wraps avito-tech/go-transaction-manager for hex-arch isolation.
type TxManager struct {
	mgr *manager.Manager
}

// NewTxManager creates a TxManager backed by pgxpool.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{
		mgr: manager.Must(trmpgx.NewDefaultFactory(pool)),
	}
}

// Do runs fn within a database transaction. If fn returns nil, commits. Otherwise rollback.
// The ctx passed to fn carries the active transaction.
func (t *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.mgr.Do(ctx, fn)
}

// CtxGetter is the default context getter for extracting the active tx from context.
// Used by repositories to get the transaction-scoped DBTX.
var CtxGetter = trmpgx.DefaultCtxGetter

// QueriesFromCtx returns sqlc Queries scoped to the active transaction (if any),
// or falls back to the provided pool-based queries.
func QueriesFromCtx(ctx context.Context, fallback *sqlcgen.Queries, pool *pgxpool.Pool) *sqlcgen.Queries {
	tx := CtxGetter.DefaultTrOrDB(ctx, pool)
	return sqlcgen.New(tx)
}
