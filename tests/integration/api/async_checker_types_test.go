//go:build integration

package api

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncTCP_ClosedPort_Fails(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "tcp-closed",
		"type":             "tcp",
		"check_config":     map[string]any{"host": "127.0.0.1", "port": port},
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
			`SELECT COUNT(*) FROM check_results WHERE monitor_id=$1 AND status <> 'up'`, m.ID).Scan(&count)
		return count > 0
	}, "tcp check failed")
}

func TestAsyncDNS_NonresolvingHost_Fails(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "dns-bad",
		"type":             "dns",
		"check_config":     map[string]any{"hostname": "does-not-resolve.invalid"},
		"interval_seconds": 300,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 30*time.Second, func() bool {
		var count int
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM check_results WHERE monitor_id=$1 AND status <> 'up'`, m.ID).Scan(&count)
		return count > 0
	}, "dns check failed")
}
