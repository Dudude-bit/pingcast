package postgres

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.StatsRepo = (*StatsRepo)(nil)

type StatsRepo struct {
	q *gen.Queries
}

func NewStatsRepo(q *gen.Queries) *StatsRepo { return &StatsRepo{q: q} }

func (r *StatsRepo) GetPublic(ctx context.Context) (port.PublicStats, error) {
	row, err := r.q.GetPublicStats(ctx)
	if err != nil {
		return port.PublicStats{}, err
	}
	return port.PublicStats{
		MonitorsCount:     row.MonitorsCount,
		IncidentsResolved: row.IncidentsResolved,
		PublicStatusPages: row.PublicStatusPages,
	}, nil
}
