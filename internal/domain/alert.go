package domain

import "github.com/google/uuid"

type MonitorAction string

const (
	ActionCreate MonitorAction = "create"
	ActionUpdate MonitorAction = "update"
	ActionDelete MonitorAction = "delete"
	ActionPause  MonitorAction = "pause"
	ActionResume MonitorAction = "resume"
)

type AlertEventType string

const (
	AlertDown         AlertEventType = "down"
	AlertUp           AlertEventType = "up"
	AlertSSLExpiring  AlertEventType = "ssl_expiring"
)

type AlertEvent struct {
	MonitorID     uuid.UUID      `json:"monitor_id"`
	UserID        uuid.UUID      `json:"user_id"`
	IncidentID    int64          `json:"incident_id"`
	MonitorName   string         `json:"monitor_name"`
	MonitorTarget string         `json:"monitor_target"`
	Event         AlertEventType `json:"event"`
	Cause         string         `json:"cause,omitempty"`
}
