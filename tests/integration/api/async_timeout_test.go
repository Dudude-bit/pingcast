//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncTimeout_SlowTarget_ClassifiedAsFailure(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	// Target sleeps 3s on every request; monitor's timeout_seconds=1
	target := harness.NewFakeTarget(t).Slow(3 * time.Second)

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "slow",
		"type":             "http",
		"check_config":     map[string]any{"url": target.URL, "method": "GET", "timeout": 1},
		"interval_seconds": 300,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 15*time.Second, func() bool {
		var count int
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM check_results WHERE monitor_id=$1 AND status <> 'up'`,
			m.ID).Scan(&count)
		return count > 0
	}, "timeout classified as failure")
}
