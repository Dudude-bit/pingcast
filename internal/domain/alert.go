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
	AlertDown AlertEventType = "down"
	AlertUp   AlertEventType = "up"
)

type AlertEvent struct {
	MonitorID   uuid.UUID
	IncidentID  int64
	MonitorName string
	MonitorURL  string
	Event       AlertEventType
	Cause       string
	TgChatID    *int64
	Email       string
	Plan        Plan
}
