package checker_test

import (
	"context"
	"encoding/json"
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
	c := checker.NewDNSChecker()
	result := c.Check(context.Background(), dnsMonitor("google.com", nil))

	if result.Status != domain.StatusUp {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusUp)
	}
	if result.ResponseTimeMs < 0 {
		t.Error("expected non-negative response time")
	}
}

func TestDNSChecker_InvalidDomain(t *testing.T) {
	c := checker.NewDNSChecker()
	result := c.Check(context.Background(), dnsMonitor("this-domain-does-not-exist-at-all.invalid", nil))

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Error("expected error message for unresolvable domain")
	}
}

func TestDNSChecker_ExpectedIPMatch(t *testing.T) {
	// localhost should resolve to 127.0.0.1
	expected := "127.0.0.1"
	c := checker.NewDNSChecker()
	result := c.Check(context.Background(), dnsMonitor("localhost", &expected))

	if result.Status != domain.StatusUp {
		t.Errorf("status = %q, want %q (expected IP should match)", result.Status, domain.StatusUp)
	}
}

func TestDNSChecker_ExpectedIPMismatch(t *testing.T) {
	wrongIP := "192.0.2.1" // TEST-NET, will not match google.com
	c := checker.NewDNSChecker()
	result := c.Check(context.Background(), dnsMonitor("google.com", &wrongIP))

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q (expected IP should not match)", result.Status, domain.StatusDown)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Error("expected error message for IP mismatch")
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
