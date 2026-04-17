package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.IncidentRepo = (*IncidentRepo)(nil)

type IncidentRepo struct {
	q *gen.Queries
}

func NewIncidentRepo(q *gen.Queries) *IncidentRepo {
	return &IncidentRepo{q: q}
}

func (r *IncidentRepo) Create(ctx context.Context, monitorID uuid.UUID, cause string) (*domain.Incident, error) {
	row, err := r.q.CreateIncident(ctx, gen.CreateIncidentParams{
		MonitorID: monitorID,
		Cause:     cause,
	})
	if err != nil {
		// Partial unique index (migration 016) — concurrent Create on a monitor
		// that already has an active incident hits idx_incidents_active_monitor.
		// Treat as cooldown-active (Issue 4.6), not an error.
		if isUniqueViolation(err) {
			return nil, domain.ErrIncidentExists
		}
		return nil, err
	}
	out := incidentFromRow(row)
	return &out, nil
}

func (r *IncidentRepo) Resolve(ctx context.Context, id int64, resolvedAt time.Time) error {
	return r.q.ResolveIncident(ctx, gen.ResolveIncidentParams{
		ID:         id,
		ResolvedAt: timeToPgtypeTimestamptz(resolvedAt),
	})
}

func (r *IncidentRepo) GetOpen(ctx context.Context, monitorID uuid.UUID) (*domain.Incident, error) {
	row, err := r.q.GetOpenIncidentByMonitorID(ctx, monitorID)
	if err != nil {
		return nil, err
	}
	out := incidentFromRow(row)
	return &out, nil
}

func (r *IncidentRepo) IsInCooldown(ctx context.Context, monitorID uuid.UUID) (bool, error) {
	return r.q.IsInCooldown(ctx, monitorID)
}

func (r *IncidentRepo) ListByMonitorID(ctx context.Context, monitorID uuid.UUID, limit int) ([]domain.Incident, error) {
	rows, err := r.q.ListIncidentsByMonitorID(ctx, gen.ListIncidentsByMonitorIDParams{
		MonitorID: monitorID,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Incident, len(rows))
	for i, row := range rows {
		out[i] = incidentFromRow(row)
	}
	return out, nil
}
