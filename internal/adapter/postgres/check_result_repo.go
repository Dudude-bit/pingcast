package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.CheckResultRepo = (*CheckResultRepo)(nil)

type CheckResultRepo struct {
	q *gen.Queries
}

func NewCheckResultRepo(q *gen.Queries) *CheckResultRepo {
	return &CheckResultRepo{q: q}
}

func (r *CheckResultRepo) Insert(ctx context.Context, cr *domain.CheckResult) error {
	_, err := r.q.InsertCheckResult(ctx, checkResultToInsertParams(cr))
	return err
}

func (r *CheckResultRepo) ConsecutiveFailures(ctx context.Context, monitorID uuid.UUID) (int, error) {
	n, err := r.q.ConsecutiveFailures(ctx, monitorID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *CheckResultRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.q.DeleteCheckResultsOlderThan(ctx, cutoff)
}
