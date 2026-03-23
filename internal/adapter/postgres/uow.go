package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kirillinakin/pingcast/internal/crypto"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.UnitOfWork = (*UnitOfWork)(nil)

// UnitOfWork creates transaction-scoped repos from a pgxpool.Pool.
type UnitOfWork struct {
	pool *pgxpool.Pool
	enc  *crypto.Encryptor
}

func NewUnitOfWork(pool *pgxpool.Pool, enc *crypto.Encryptor) *UnitOfWork {
	return &UnitOfWork{pool: pool, enc: enc}
}

func (u *UnitOfWork) Begin(ctx context.Context) (port.UnitOfWorkTx, error) {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	queries := sqlcgen.New(tx)
	return &unitOfWorkTx{
		tx:       tx,
		monitors: NewMonitorRepoWithEncryption(queries, u.enc),
		channels: NewChannelRepoWithEncryption(queries, u.enc),
	}, nil
}

var _ port.UnitOfWorkTx = (*unitOfWorkTx)(nil)

type unitOfWorkTx struct {
	tx       interface{ Commit(context.Context) error; Rollback(context.Context) error }
	monitors *MonitorRepo
	channels *ChannelRepo
}

func (t *unitOfWorkTx) Monitors() port.MonitorRepo { return t.monitors }
func (t *unitOfWorkTx) Channels() port.ChannelRepo { return t.channels }
func (t *unitOfWorkTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *unitOfWorkTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
