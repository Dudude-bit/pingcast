package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.MonitorChecker = (*DNSChecker)(nil)

type DNSCheckConfig struct {
	Hostname   string  `json:"hostname"`
	ExpectedIP *string `json:"expected_ip,omitempty"`
	DNSServer  *string `json:"dns_server,omitempty"`
}

type DNSChecker struct{}

func NewDNSChecker() *DNSChecker {
	return &DNSChecker{}
}

func (c *DNSChecker) Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult {
	start := time.Now()
	result := &domain.CheckResult{MonitorID: monitor.ID, CheckedAt: start}

	var cfg DNSCheckConfig
	if err := json.Unmarshal(monitor.CheckConfig, &cfg); err != nil {
		errMsg := fmt.Sprintf("invalid dns config: %s", err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		return result
	}

	resolver := net.DefaultResolver
	if cfg.DNSServer != nil && *cfg.DNSServer != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 10 * time.Second}
				return d.DialContext(ctx, "udp", *cfg.DNSServer+":53")
			},
		}
	}

	ips, err := resolver.LookupHost(ctx, cfg.Hostname)
	result.ResponseTimeMs = int(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := fmt.Sprintf("dns lookup failed: %s", err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		return result
	}

	if len(ips) == 0 {
		errMsg := "dns lookup returned no results"
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		return result
	}

	if cfg.ExpectedIP != nil && *cfg.ExpectedIP != "" {
		if !slices.Contains(ips, *cfg.ExpectedIP) {
			errMsg := fmt.Sprintf("expected IP %s not found in results: %v", *cfg.ExpectedIP, ips)
			result.Status = domain.StatusDown
			result.ErrorMessage = &errMsg
			return result
		}
	}

	result.Status = domain.StatusUp
	return result
}

func (c *DNSChecker) ValidateConfig(raw json.RawMessage) error {
	var cfg DNSCheckConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid dns config: %w", err)
	}
	if cfg.Hostname == "" {
		return fmt.Errorf("hostname required")
	}
	return nil
}

func (c *DNSChecker) ConfigSchema() port.ConfigSchema {
	return port.ConfigSchema{Fields: []port.ConfigField{
		{Name: "hostname", Label: "Hostname", Type: "text", Required: true, Placeholder: "example.com"},
		{Name: "expected_ip", Label: "Expected IP", Type: "text", Placeholder: "optional — verify specific IP"},
		{Name: "dns_server", Label: "DNS Server", Type: "text", Placeholder: "optional — e.g. 8.8.8.8"},
	}}
}

func (c *DNSChecker) Target(raw json.RawMessage) string {
	var cfg DNSCheckConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return ""
	}
	return "dns://" + cfg.Hostname
}

func (c *DNSChecker) Host(raw json.RawMessage) string {
	var cfg DNSCheckConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return ""
	}
	return cfg.Hostname
}
