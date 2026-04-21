package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.MaintenanceWindowRepo = (*MaintenanceWindowRepo)(nil)

type MaintenanceWindowRepo struct {
	q *gen.Queries
}

func NewMaintenanceWindowRepo(q *gen.Queries) *MaintenanceWindowRepo {
	return &MaintenanceWindowRepo{q: q}
}

func (r *MaintenanceWindowRepo) Create(ctx context.Context, in port.CreateMaintenanceWindowInput) (*domain.MaintenanceWindow, error) {
	row, err := r.q.CreateMaintenanceWindow(ctx, gen.CreateMaintenanceWindowParams{
		MonitorID: in.MonitorID,
		StartsAt:  in.StartsAt,
		EndsAt:    in.EndsAt,
		Reason:    in.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &domain.MaintenanceWindow{
		ID:        row.ID,
		MonitorID: row.MonitorID,
		StartsAt:  row.StartsAt,
		EndsAt:    row.EndsAt,
		Reason:    row.Reason,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *MaintenanceWindowRepo) ListByMonitorID(ctx context.Context, monitorID uuid.UUID) ([]domain.MaintenanceWindow, error) {
	rows, err := r.q.ListMaintenanceWindowsByMonitorID(ctx, monitorID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.MaintenanceWindow, len(rows))
	for i, row := range rows {
		out[i] = domain.MaintenanceWindow{
			ID:        row.ID,
			MonitorID: row.MonitorID,
			StartsAt:  row.StartsAt,
			EndsAt:    row.EndsAt,
			Reason:    row.Reason,
			CreatedAt: row.CreatedAt,
		}
	}
	return out, nil
}

func (r *MaintenanceWindowRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.MaintenanceWindow, error) {
	rows, err := r.q.ListMaintenanceWindowsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.MaintenanceWindow, len(rows))
	for i, row := range rows {
		out[i] = domain.MaintenanceWindow{
			ID:        row.ID,
			MonitorID: row.MonitorID,
			StartsAt:  row.StartsAt,
			EndsAt:    row.EndsAt,
			Reason:    row.Reason,
			CreatedAt: row.CreatedAt,
		}
	}
	return out, nil
}

func (r *MaintenanceWindowRepo) Delete(ctx context.Context, id int64, userID uuid.UUID) error {
	return r.q.DeleteMaintenanceWindow(ctx, gen.DeleteMaintenanceWindowParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *MaintenanceWindowRepo) HasActive(ctx context.Context, monitorID uuid.UUID) (bool, error) {
	return r.q.HasActiveMaintenanceWindow(ctx, monitorID)
}
