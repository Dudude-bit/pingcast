package port

import "context"

// UnitOfWork creates transactional scopes.
type UnitOfWork interface {
	Begin(ctx context.Context) (UnitOfWorkTx, error)
}

// UnitOfWorkTx provides repos scoped to a single transaction.
type UnitOfWorkTx interface {
	Monitors() MonitorRepo
	Channels() ChannelRepo
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
