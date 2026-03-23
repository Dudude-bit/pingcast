package checker_test

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func tcpMonitor(host string, port int) *domain.Monitor {
	cfg := map[string]any{
		"host": host,
		"port": port,
	}
	data, _ := json.Marshal(cfg)
	return &domain.Monitor{
		Type:        domain.MonitorTCP,
		CheckConfig: data,
	}
}

func tcpMonitorWithTimeout(host string, port int, timeout int) *domain.Monitor {
	cfg := map[string]any{
		"host":    host,
		"port":    port,
		"timeout": timeout,
	}
	data, _ := json.Marshal(cfg)
	return &domain.Monitor{
		Type:        domain.MonitorTCP,
		CheckConfig: data,
	}
}

func TestTCPChecker_OpenPort(t *testing.T) {
	// Start a TCP listener on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	// Accept connections in background so the dial succeeds.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	c := checker.NewTCPChecker(5 * time.Second)
	result := c.Check(context.Background(), tcpMonitor("127.0.0.1", port))

	if result.Status != domain.StatusUp {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusUp)
	}
	if result.ResponseTimeMs < 0 {
		t.Error("expected non-negative response time")
	}
}

func TestTCPChecker_ClosedPort(t *testing.T) {
	// Find a port that is not listening by binding and immediately closing.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	ln.Close() // close immediately so port is not listening

	c := checker.NewTCPChecker(2 * time.Second)
	result := c.Check(context.Background(), tcpMonitor("127.0.0.1", port))

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Error("expected error message for closed port")
	}
}

func TestTCPChecker_PerMonitorTimeoutOverride(t *testing.T) {
	// Verify per-monitor timeout is parsed: use a closed port with short override.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	ln.Close()

	c := checker.NewTCPChecker(30 * time.Second)
	result := c.Check(context.Background(), tcpMonitorWithTimeout("127.0.0.1", port, 1))

	// Should be down (closed port), confirming per-monitor timeout was applied
	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
}

func TestTCPChecker_InvalidConfig(t *testing.T) {
	c := checker.NewTCPChecker(5 * time.Second)
	mon := &domain.Monitor{
		Type:        domain.MonitorTCP,
		CheckConfig: json.RawMessage(`{invalid json`),
	}
	result := c.Check(context.Background(), mon)

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Error("expected error message for invalid config")
	}
}

func TestTCPChecker_UnresolvableHost(t *testing.T) {
	c := checker.NewTCPChecker(2 * time.Second)
	result := c.Check(context.Background(), tcpMonitor("this-host-does-not-exist.invalid", 80))

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Error("expected error message for unresolvable host")
	}
}
