package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.MonitorGroupRepo = (*MonitorGroupRepo)(nil)

type MonitorGroupRepo struct {
	q *gen.Queries
}

func NewMonitorGroupRepo(q *gen.Queries) *MonitorGroupRepo {
	return &MonitorGroupRepo{q: q}
}

func (r *MonitorGroupRepo) Create(ctx context.Context, userID uuid.UUID, name string, ordering int) (*domain.MonitorGroup, error) {
	row, err := r.q.CreateMonitorGroup(ctx, gen.CreateMonitorGroupParams{
		UserID: userID,
		Name:   name,
		//nolint:gosec // G115: ordering bounded by caller (0..99 typically)
		Ordering: int32(ordering),
	})
	if err != nil {
		return nil, err
	}
	return &domain.MonitorGroup{
		ID:        row.ID,
		UserID:    row.UserID,
		Name:      row.Name,
		Ordering:  int(row.Ordering),
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *MonitorGroupRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.MonitorGroup, error) {
	rows, err := r.q.ListMonitorGroupsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.MonitorGroup, len(rows))
	for i, row := range rows {
		out[i] = domain.MonitorGroup{
			ID:        row.ID,
			UserID:    row.UserID,
			Name:      row.Name,
			Ordering:  int(row.Ordering),
			CreatedAt: row.CreatedAt,
		}
	}
	return out, nil
}

func (r *MonitorGroupRepo) Update(ctx context.Context, id int64, userID uuid.UUID, name string, ordering int) error {
	return r.q.UpdateMonitorGroup(ctx, gen.UpdateMonitorGroupParams{
		ID:     id,
		UserID: userID,
		Name:   name,
		//nolint:gosec // G115: ordering bounded by caller
		Ordering: int32(ordering),
	})
}

func (r *MonitorGroupRepo) Delete(ctx context.Context, id int64, userID uuid.UUID) error {
	return r.q.DeleteMonitorGroup(ctx, gen.DeleteMonitorGroupParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *MonitorGroupRepo) AssignMonitor(ctx context.Context, monitorID, userID uuid.UUID, groupID *int64) error {
	var gid pgtype.Int8
	if groupID != nil {
		gid = pgtype.Int8{Int64: *groupID, Valid: true}
	}
	return r.q.AssignMonitorToGroup(ctx, gen.AssignMonitorToGroupParams{
		ID:      monitorID,
		UserID:  userID,
		GroupID: gid,
	})
}
