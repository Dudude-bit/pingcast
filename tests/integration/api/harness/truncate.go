//go:build integration

package harness

import (
	"context"
	"strings"
	"testing"
)

// truncateTables is the set of user-data tables reset between tests.
// CASCADE handles FK dependencies and partition children
// (e.g. check_results_default).
var truncateTables = []string{
	"check_results",
	"incidents",
	"monitor_channels",
	"api_keys",
	"sessions",
	"notification_channels",
	"failed_alerts",
	"monitor_uptime_hourly",
	"monitors",
	"users",
}

// Reset restores the App to a clean slate: truncates Postgres tables
// and flushes Redis. Called from harness.New on every test entry.
func (a *App) Reset(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	stmt := "TRUNCATE " + strings.Join(truncateTables, ", ") + " RESTART IDENTITY CASCADE"
	if _, err := a.Pool.Exec(ctx, stmt); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	if err := a.Redis.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushdb: %v", err)
	}
}
