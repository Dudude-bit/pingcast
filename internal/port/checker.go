package port

import (
	"context"
	"encoding/json"

	"github.com/kirillinakin/pingcast/internal/domain"
)

type MonitorChecker interface {
	Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult
	ValidateConfig(raw json.RawMessage) error
	ConfigSchema() ConfigSchema
	Target(raw json.RawMessage) string
	Host(raw json.RawMessage) string
}

type CheckerRegistry interface {
	Get(monitorType domain.MonitorType) (MonitorChecker, error)
	Types() []MonitorTypeInfo
	ValidateConfig(monitorType domain.MonitorType, raw json.RawMessage) error
	Target(monitorType domain.MonitorType, raw json.RawMessage) string
	Host(monitorType domain.MonitorType, raw json.RawMessage) string
}

type MonitorTypeInfo struct {
	Type   domain.MonitorType `json:"type"`
	Label  string             `json:"label"`
	Schema ConfigSchema       `json:"schema"`
}

type ConfigSchema struct {
	Fields []ConfigField `json:"fields"`
}

type ConfigField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []Option `json:"options,omitempty"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
