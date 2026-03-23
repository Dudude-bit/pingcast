package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type ctxKey int

const (
	queryStartKey ctxKey = iota
	querySQLKey
)

// SlowQueryTracer implements pgx.QueryTracer and logs queries that exceed
// a configurable threshold. When devMode is true, every query is logged.
type SlowQueryTracer struct {
	threshold time.Duration
	devMode   bool
}

// NewSlowQueryTracer returns a tracer that warns on queries slower than threshold.
// If devMode is true, all queries are logged at debug level regardless of duration.
func NewSlowQueryTracer(threshold time.Duration, devMode bool) *SlowQueryTracer {
	return &SlowQueryTracer{
		threshold: threshold,
		devMode:   devMode,
	}
}

// TraceQueryStart is called at the beginning of Query, QueryRow, and Exec calls.
func (t *SlowQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = context.WithValue(ctx, queryStartKey, time.Now())
	ctx = context.WithValue(ctx, querySQLKey, data.SQL)
	return ctx
}

// TraceQueryEnd is called at the conclusion of Query, QueryRow, and Exec calls.
func (t *SlowQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	startTime, ok := ctx.Value(queryStartKey).(time.Time)
	if !ok {
		return
	}
	sql, _ := ctx.Value(querySQLKey).(string)

	duration := time.Since(startTime)

	if duration >= t.threshold {
		slog.WarnContext(ctx, "slow query",
			"query", sql,
			"duration_ms", duration.Milliseconds(),
			"err", data.Err,
		)
		return
	}

	if t.devMode {
		slog.DebugContext(ctx, "query",
			"query", sql,
			"duration_ms", duration.Milliseconds(),
			"err", data.Err,
		)
	}
}
