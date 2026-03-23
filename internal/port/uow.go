package port

import "context"

// TxManager manages database transactions.
// App layer calls Do() without knowing about pgx, sql, or any DB driver.
type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
