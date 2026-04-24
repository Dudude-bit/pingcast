package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.IncidentRepo = (*IncidentRepo)(nil)

type IncidentRepo struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

func NewIncidentRepo(pool *pgxpool.Pool, q *gen.Queries) *IncidentRepo {
	return &IncidentRepo{pool: pool, q: q}
}

// queries returns sqlc Queries scoped to the active transaction (if any).
// Without this, Create inside txm.Do couldn't see the just-inserted
// monitor row and would fail FK validation.
func (r *IncidentRepo) queries(ctx context.Context) *gen.Queries {
	return QueriesFromCtx(ctx, r.q, r.pool)
}

func (r *IncidentRepo) Create(ctx context.Context, in port.CreateIncidentInput) (*domain.Incident, error) {
	state := in.State
	if state == "" {
		state = domain.IncidentStateInvestigating
	}
	row, err := r.queries(ctx).CreateIncident(ctx, gen.CreateIncidentParams{
		MonitorID: in.MonitorID,
		Cause:     in.Cause,
		State:     gen.IncidentState(state),
		IsManual:  in.IsManual,
		Title:     in.Title,
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
	return r.queries(ctx).ResolveIncident(ctx, gen.ResolveIncidentParams{
		ID:         id,
		ResolvedAt: timeToPgtypeTimestamptz(resolvedAt),
	})
}

func (r *IncidentRepo) UpdateState(ctx context.Context, id int64, state domain.IncidentState) error {
	return r.queries(ctx).UpdateIncidentState(ctx, gen.UpdateIncidentStateParams{
		ID:    id,
		State: gen.IncidentState(state),
	})
}

func (r *IncidentRepo) GetByID(ctx context.Context, id int64) (*domain.Incident, error) {
	row, err := r.queries(ctx).GetIncidentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	out := incidentFromRow(row)
	return &out, nil
}

func (r *IncidentRepo) GetOpen(ctx context.Context, monitorID uuid.UUID) (*domain.Incident, error) {
	row, err := r.queries(ctx).GetOpenIncidentByMonitorID(ctx, monitorID)
	if err != nil {
		return nil, err
	}
	out := incidentFromRow(row)
	return &out, nil
}

func (r *IncidentRepo) IsInCooldown(ctx context.Context, monitorID uuid.UUID) (bool, error) {
	return r.queries(ctx).IsInCooldown(ctx, monitorID)
}

func (r *IncidentRepo) ListForExport(ctx context.Context, userID uuid.UUID) ([]port.IncidentExportRow, error) {
	rows, err := r.queries(ctx).ListIncidentsByUserIDForExport(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]port.IncidentExportRow, len(rows))
	for i, row := range rows {
		out[i] = port.IncidentExportRow{
			ID:          row.ID,
			MonitorID:   row.MonitorID,
			MonitorName: row.MonitorName,
			StartedAt:   row.StartedAt,
			ResolvedAt:  pgtypeTimestamptzToPtr(row.ResolvedAt),
			Cause:       row.Cause,
			State:       domain.IncidentState(row.State),
			IsManual:    row.IsManual,
			Title:       row.Title,
		}
	}
	return out, nil
}

func (r *IncidentRepo) ListByMonitorID(ctx context.Context, monitorID uuid.UUID, limit int) ([]domain.Incident, error) {
	rows, err := r.queries(ctx).ListIncidentsByMonitorID(ctx, gen.ListIncidentsByMonitorIDParams{
		MonitorID: monitorID,
		//nolint:gosec // G115: list limit comes from constant call sites (5, 10), always small positive
		Limit: int32(limit),
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
