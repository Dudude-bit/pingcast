package natsbus

import "github.com/google/uuid"

// MonitorChangedEvent is published by API when a monitor is created/updated/deleted/paused/resumed.
type MonitorChangedEvent struct {
	Action    string       `json:"action"` // create, update, delete, pause, resume
	MonitorID uuid.UUID    `json:"monitor_id"`
	Monitor   *MonitorData `json:"monitor,omitempty"`
}

// MonitorData carries full monitor info for create/update/resume actions.
type MonitorData struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	URL                string    `json:"url"`
	Method             string    `json:"method"`
	IntervalSeconds    int       `json:"interval_seconds"`
	ExpectedStatus     int       `json:"expected_status"`
	Keyword            *string   `json:"keyword,omitempty"`
	AlertAfterFailures int       `json:"alert_after_failures"`
	UserID             uuid.UUID `json:"user_id"`
}

// AlertEvent is published by Checker when a monitor goes down or recovers.
// Fat event — contains all data needed for notification delivery.
type AlertEvent struct {
	MonitorID   uuid.UUID `json:"monitor_id"`
	IncidentID  int64     `json:"incident_id"`
	MonitorName string    `json:"monitor_name"`
	MonitorURL  string    `json:"monitor_url"`
	Event       string    `json:"event"` // "down" or "up"
	Cause       string    `json:"cause,omitempty"`
	TgChatID    *int64    `json:"tg_chat_id,omitempty"`
	Email       string    `json:"email"`
	Plan        string    `json:"plan"`
}
