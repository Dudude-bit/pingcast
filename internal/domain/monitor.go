package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MonitorStatus string

const (
	StatusUp      MonitorStatus = "up"
	StatusDown    MonitorStatus = "down"
	StatusUnknown MonitorStatus = "unknown"
)

type MonitorType string

const (
	MonitorHTTP MonitorType = "http"
	MonitorTCP  MonitorType = "tcp"
	MonitorDNS  MonitorType = "dns"
)

func (t MonitorType) Valid() bool {
	switch t {
	case MonitorHTTP, MonitorTCP, MonitorDNS:
		return true
	}
	return false
}

type Monitor struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	Name               string
	Type               MonitorType
	CheckConfig        json.RawMessage
	IntervalSeconds    int
	AlertAfterFailures int
	IsPaused           bool
	IsPublic           bool
	CurrentStatus      MonitorStatus
	CreatedAt          time.Time
}

// ParseCheckConfig unmarshals CheckConfig into a map. Returns error instead of silently returning nil.
func (m *Monitor) ParseCheckConfig() (map[string]any, error) {
	if len(m.CheckConfig) == 0 {
		return nil, nil
	}
	var config map[string]any
	if err := json.Unmarshal(m.CheckConfig, &config); err != nil {
		return nil, fmt.Errorf("parse check config for monitor %s: %w", m.ID, err)
	}
	return config, nil
}

type CheckResult struct {
	ID             int64
	MonitorID      uuid.UUID
	Status         MonitorStatus
	StatusCode     *int
	ResponseTimeMs int
	ErrorMessage   *string
	CheckedAt      time.Time
}
