package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type UptimeRepo struct {
	q *sqlcgen.Queries
}

func NewUptimeRepo(q *sqlcgen.Queries) *UptimeRepo {
	return &UptimeRepo{q: q}
}

// RecordCheck increments the uptime hourly aggregation for the given check.
func (r *UptimeRepo) RecordCheck(ctx context.Context, monitorID uuid.UUID, checkedAt time.Time, success bool) error {
	successInt := int32(0)
	if success {
		successInt = 1
	}
	return r.q.UpsertUptimeHourly(ctx, sqlcgen.UpsertUptimeHourlyParams{
		MonitorID: monitorID,
		Column2:   checkedAt,
		Column3:   successInt,
	})
}

// GetUptime returns the uptime percentage from hourly aggregates since the given time.
func (r *UptimeRepo) GetUptime(ctx context.Context, monitorID uuid.UUID, since time.Time) (float64, error) {
	result, err := r.q.GetUptimeFromHourly(ctx, sqlcgen.GetUptimeFromHourlyParams{
		MonitorID: monitorID,
		Hour:      since,
	})
	if err != nil {
		return 0, fmt.Errorf("get uptime from hourly: %w", err)
	}
	switch v := result.(type) {
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	default:
		return 0, nil
	}
}

// GetUptimeBatch returns uptime percentages for multiple monitors since the given time.
func (r *UptimeRepo) GetUptimeBatch(ctx context.Context, monitorIDs []uuid.UUID, since time.Time) (map[uuid.UUID]float64, error) {
	rows, err := r.q.GetUptimeBatchFromHourly(ctx, sqlcgen.GetUptimeBatchFromHourlyParams{
		Column1: monitorIDs,
		Hour:    since,
	})
	if err != nil {
		return nil, fmt.Errorf("get uptime batch: %w", err)
	}
	result := make(map[uuid.UUID]float64, len(rows))
	for _, row := range rows {
		switch v := row.UptimePercent.(type) {
		case float64:
			result[row.MonitorID] = v
		case int64:
			result[row.MonitorID] = float64(v)
		}
	}
	return result, nil
}
