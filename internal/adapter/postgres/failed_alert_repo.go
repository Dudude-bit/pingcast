package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.FailedAlertRepo = (*FailedAlertRepo)(nil)

type FailedAlertRepo struct {
	q *gen.Queries
}

func NewFailedAlertRepo(q *gen.Queries) *FailedAlertRepo {
	return &FailedAlertRepo{q: q}
}

func (r *FailedAlertRepo) Create(ctx context.Context, event json.RawMessage, errMsg string, failedChannelIDs []uuid.UUID) error {
	if err := r.q.InsertFailedAlert(ctx, gen.InsertFailedAlertParams{
		Event:            event,
		Error:            errMsg,
		FailedChannelIds: failedChannelIDs,
	}); err != nil {
		return fmt.Errorf("insert failed alert: %w", err)
	}
	return nil
}
