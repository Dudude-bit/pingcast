package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.MonitorChecker = (*TCPChecker)(nil)

type TCPCheckConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type TCPChecker struct {
	timeout time.Duration
}

func NewTCPChecker(timeout time.Duration) *TCPChecker {
	return &TCPChecker{timeout: timeout}
}

func (c *TCPChecker) Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult {
	start := time.Now()
	result := &domain.CheckResult{MonitorID: monitor.ID, CheckedAt: start}

	var cfg TCPCheckConfig
	if err := json.Unmarshal(monitor.CheckConfig, &cfg); err != nil {
		errMsg := fmt.Sprintf("invalid tcp config: %s", err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		return result
	}

	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	conn, err := net.DialTimeout("tcp", addr, c.timeout)
	result.ResponseTimeMs = int(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := fmt.Sprintf("tcp connect failed: %s", err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		return result
	}
	conn.Close()

	result.Status = domain.StatusUp
	return result
}

func (c *TCPChecker) ValidateConfig(raw json.RawMessage) error {
	var cfg TCPCheckConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid tcp config: %w", err)
	}
	if cfg.Host == "" {
		return fmt.Errorf("host required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	return nil
}

func (c *TCPChecker) ConfigSchema() port.ConfigSchema {
	return port.ConfigSchema{Fields: []port.ConfigField{
		{Name: "host", Label: "Host", Type: "text", Required: true, Placeholder: "db.example.com"},
		{Name: "port", Label: "Port", Type: "number", Required: true, Placeholder: "5432"},
	}}
}

func (c *TCPChecker) Target(raw json.RawMessage) (string, error) {
	var cfg TCPCheckConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("invalid tcp config: %w", err)
	}
	return fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port), nil
}

func (c *TCPChecker) Host(raw json.RawMessage) (string, error) {
	var cfg TCPCheckConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("invalid tcp config: %w", err)
	}
	return cfg.Host, nil
}
