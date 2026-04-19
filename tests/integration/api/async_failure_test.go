//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncFailure_ThreeConsecutive500s_OpensIncident(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(5)

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "api",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 3,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	// Scheduler's NATS subscriber needs a moment to receive the
	// monitor.changed event. Without this wait, DispatchAll fires
	// against an empty monitor map on the first iteration.
	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	// Three synchronous fan-outs. We wait for each check_result to
	// land in Postgres before firing the next — otherwise dispatches
	// race in NATS and the "consecutive failures" count may not
	// reach the threshold in order.
	for i := 0; i < 3; i++ {
		if err := stack.Scheduler.Leader.DispatchAll(context.Background()); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		want := i + 1
		harness.WaitFor(t, 5*time.Second, func() bool {
			var count int
			_ = h.App.Pool.QueryRow(context.Background(),
				`SELECT COUNT(*) FROM check_results WHERE monitor_id=$1`,
				m.ID).Scan(&count)
			return count >= want
		}, "check_result committed")
	}

	harness.WaitFor(t, 10*time.Second, func() bool {
		var open bool
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NULL)`,
			m.ID).Scan(&open)
		return open
	}, "incident opened")
}

func TestAsyncFailure_AlertAfterOne_FiresOnFirstFailure(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(1)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "single-fail",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	// Scheduler's NATS subscriber needs a moment to receive the
	// monitor.changed event. Without this wait, DispatchAll fires
	// against an empty monitor map on the first iteration.
	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	if err := stack.Scheduler.Leader.DispatchAll(context.Background()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	harness.WaitFor(t, 10*time.Second, func() bool {
		var open bool
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NULL)`,
			m.ID).Scan(&open)
		return open
	}, "incident opened on first failure")
}

func TestAsyncFailure_5xx_RecordsFailureStatus(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).RespondWith(502)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "5xx",
		"type":             "http",
		"check_config":     map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds": 300,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	// Scheduler's NATS subscriber needs a moment to receive the
	// monitor.changed event. Without this wait, DispatchAll fires
	// against an empty monitor map on the first iteration.
	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 10*time.Second, func() bool {
		var count int
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM check_results WHERE monitor_id=$1 AND status <> 'up'`,
			m.ID).Scan(&count)
		return count > 0
	}, "check_result recorded with failure status")
}
