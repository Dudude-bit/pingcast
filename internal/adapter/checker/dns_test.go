package checker_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func dnsMonitor(hostname string, expectedIP *string) *domain.Monitor {
	cfg := map[string]any{
		"hostname": hostname,
	}
	if expectedIP != nil {
		cfg["expected_ip"] = *expectedIP
	}
	data, _ := json.Marshal(cfg)
	return &domain.Monitor{
		Type:        domain.MonitorDNS,
		CheckConfig: data,
	}
}

func TestDNSChecker_ValidDomain(t *testing.T) {
	// Use localhost — always resolvable, no network dependency.
	c := checker.NewDNSChecker()
	result := c.Check(context.Background(), dnsMonitor("localhost", nil))

	if result.Status != domain.StatusUp {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusUp)
	}
	if result.ResponseTimeMs < 0 {
		t.Error("expected non-negative response time")
	}
}

func TestDNSChecker_InvalidDomain(t *testing.T) {
	c := checker.NewDNSChecker()
	// .invalid TLD is reserved by RFC 2606 — guaranteed to not resolve.
	result := c.Check(context.Background(), dnsMonitor("this-will-never-exist.invalid", nil))

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Error("expected error message for unresolvable domain")
	}
}

func TestDNSChecker_ExpectedIPMatch(t *testing.T) {
	expected := "127.0.0.1"
	c := checker.NewDNSChecker()
	result := c.Check(context.Background(), dnsMonitor("localhost", &expected))

	if result.Status != domain.StatusUp {
		t.Errorf("status = %q, want %q (localhost should resolve to 127.0.0.1)", result.Status, domain.StatusUp)
	}
}

func TestDNSChecker_ExpectedIPMismatch(t *testing.T) {
	wrongIP := "192.0.2.1" // TEST-NET-1, will not match localhost
	c := checker.NewDNSChecker()
	result := c.Check(context.Background(), dnsMonitor("localhost", &wrongIP))

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
	if result.ErrorMessage == nil {
		t.Error("expected error message for IP mismatch")
	}
	if result.ErrorMessage != nil && !strings.Contains(*result.ErrorMessage, "expected IP") {
		t.Errorf("error should mention expected IP, got: %s", *result.ErrorMessage)
	}
}

func TestDNSChecker_InvalidConfig(t *testing.T) {
	c := checker.NewDNSChecker()
	mon := &domain.Monitor{
		Type:        domain.MonitorDNS,
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

