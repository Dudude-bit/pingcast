package postgres

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.IncidentUpdateRepo = (*IncidentUpdateRepo)(nil)

type IncidentUpdateRepo struct {
	q *gen.Queries
}

func NewIncidentUpdateRepo(q *gen.Queries) *IncidentUpdateRepo {
	return &IncidentUpdateRepo{q: q}
}

func (r *IncidentUpdateRepo) Create(ctx context.Context, in port.CreateIncidentUpdateInput) (*domain.IncidentUpdate, error) {
	row, err := r.q.CreateIncidentUpdate(ctx, gen.CreateIncidentUpdateParams{
		IncidentID:     in.IncidentID,
		State:          gen.IncidentState(in.State),
		Body:           in.Body,
		PostedByUserID: in.PostedByUserID,
	})
	if err != nil {
		return nil, err
	}
	out := incidentUpdateFromRow(row)
	return &out, nil
}

func (r *IncidentUpdateRepo) ListByIncidentID(ctx context.Context, incidentID int64) ([]domain.IncidentUpdate, error) {
	rows, err := r.q.ListIncidentUpdates(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.IncidentUpdate, len(rows))
	for i, row := range rows {
		out[i] = incidentUpdateFromRow(row)
	}
	return out, nil
}

func incidentUpdateFromRow(r gen.IncidentUpdate) domain.IncidentUpdate {
	return domain.IncidentUpdate{
		ID:             r.ID,
		IncidentID:     r.IncidentID,
		State:          domain.IncidentState(r.State),
		Body:           r.Body,
		PostedByUserID: r.PostedByUserID,
		PostedAt:       r.PostedAt,
	}
}
