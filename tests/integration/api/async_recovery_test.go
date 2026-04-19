//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncRecovery_AfterFailures_ClosesIncident(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	// 3 failures, then 200 forever. Explicit because FailNext(3) alone
	// leaves the queue with a single 500 that would otherwise repeat.
	target := harness.NewFakeTarget(t).FailNext(3).RespondWith(200)

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "recovering",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	// First down → incident opens
	_ = stack.Scheduler.Leader.DispatchAll(context.Background())
	harness.WaitFor(t, 10*time.Second, func() bool {
		var open bool
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NULL)`, m.ID).Scan(&open)
		return open
	}, "incident opened")

	// Fire dispatches until the monitor's current_status flips to 'up',
	// then wait for the incident to resolve. Multiple dispatches may be
	// needed because the scheduler's background ticker runs alongside
	// DispatchAll — target.FailNext(3) may drain via either path.
	harness.WaitFor(t, 15*time.Second, func() bool {
		_ = stack.Scheduler.Leader.DispatchAll(context.Background())
		time.Sleep(300 * time.Millisecond)
		var status string
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT current_status FROM monitors WHERE id=$1`, m.ID).Scan(&status)
		return status == "up"
	}, "monitor transitioned to up")

	harness.WaitFor(t, 10*time.Second, func() bool {
		var closed bool
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NOT NULL)`, m.ID).Scan(&closed)
		return closed
	}, "incident resolved")
}
