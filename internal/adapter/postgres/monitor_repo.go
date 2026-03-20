package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.MonitorRepo = (*MonitorRepo)(nil)

type MonitorRepo struct {
	q *gen.Queries
}

func NewMonitorRepo(q *gen.Queries) *MonitorRepo {
	return &MonitorRepo{q: q}
}

func (r *MonitorRepo) Create(ctx context.Context, m *domain.Monitor) (*domain.Monitor, error) {
	row, err := r.q.CreateMonitor(ctx, monitorToCreateParams(m))
	if err != nil {
		return nil, err
	}
	out := monitorFromRow(row)
	return &out, nil
}

func (r *MonitorRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Monitor, error) {
	row, err := r.q.GetMonitorByID(ctx, id)
	if err != nil {
		return nil, err
	}
	out := monitorFromRow(row)
	return &out, nil
}

func (r *MonitorRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Monitor, error) {
	rows, err := r.q.ListMonitorsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromRow(row)
	}
	return out, nil
}

func (r *MonitorRepo) ListPublicBySlug(ctx context.Context, slug string) ([]domain.Monitor, error) {
	rows, err := r.q.ListPublicMonitorsByUserSlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromRow(row)
	}
	return out, nil
}

func (r *MonitorRepo) ListActive(ctx context.Context) ([]domain.Monitor, error) {
	rows, err := r.q.ListActiveMonitors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromRow(row)
	}
	return out, nil
}

func (r *MonitorRepo) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	n, err := r.q.CountMonitorsByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *MonitorRepo) Update(ctx context.Context, m *domain.Monitor) error {
	return r.q.UpdateMonitor(ctx, monitorToUpdateParams(m))
}

func (r *MonitorRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MonitorStatus) error {
	return r.q.UpdateMonitorStatus(ctx, gen.UpdateMonitorStatusParams{
		ID:            id,
		CurrentStatus: string(status),
	})
}

func (r *MonitorRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.q.DeleteMonitor(ctx, gen.DeleteMonitorParams{
		ID:     id,
		UserID: userID,
	})
}
