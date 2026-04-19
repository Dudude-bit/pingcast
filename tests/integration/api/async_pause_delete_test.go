//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncScheduler_SkipsPausedMonitor(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "to-pause",
		"type":             "http",
		"check_config":     map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds": 300,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	p := s.POST(t, "/api/monitors/"+m.ID+"/pause", nil)
	harness.AssertStatus(t, p, 200)

	// Wait for the monitor.changed(pause) event to propagate. The
	// scheduler removes paused monitors from its map.
	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() == 0
	}, "scheduler dropped paused monitor")

	before := target.Hits()
	_ = stack.Scheduler.Leader.DispatchAll(context.Background())
	time.Sleep(500 * time.Millisecond)
	if target.Hits() != before {
		t.Fatalf("paused monitor got hit: before=%d after=%d", before, target.Hits())
	}
}

func TestAsyncScheduler_SkipsDeletedMonitor(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "to-delete",
		"type":             "http",
		"check_config":     map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds": 300,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	d := s.DELETE(t, "/api/monitors/"+m.ID)
	harness.AssertStatus(t, d, 204)

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() == 0
	}, "scheduler dropped deleted monitor")

	before := target.Hits()
	_ = stack.Scheduler.Leader.DispatchAll(context.Background())
	time.Sleep(500 * time.Millisecond)
	if target.Hits() != before {
		t.Fatalf("deleted monitor got hit: before=%d after=%d", before, target.Hits())
	}
}
