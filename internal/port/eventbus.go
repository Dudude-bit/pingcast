package port

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// MonitorChangedEvent is the event envelope — decoupled from domain.Monitor.
// Only contains fields that subscribers actually need for scheduling.
type MonitorChangedEvent struct {
	Action             domain.MonitorAction `json:"action"`
	MonitorID          uuid.UUID            `json:"monitor_id"`
	Name               string               `json:"name,omitempty"`
	Type               domain.MonitorType   `json:"type,omitempty"`
	CheckConfig        json.RawMessage      `json:"check_config,omitempty"`
	IntervalSeconds    int                  `json:"interval_seconds,omitempty"`
	AlertAfterFailures int                  `json:"alert_after_failures,omitempty"`
	IsPaused           bool                 `json:"is_paused"`
}

type MonitorEventPublisher interface {
	PublishMonitorChanged(ctx context.Context, event MonitorChangedEvent) error
}

type AlertEventPublisher interface {
	PublishAlert(ctx context.Context, event *domain.AlertEvent) error
}

type MonitorEventSubscriber interface {
	Subscribe(ctx context.Context, handler func(ctx context.Context, event MonitorChangedEvent) error) error
	Stop()
}

type AlertEventSubscriber interface {
	Subscribe(ctx context.Context, handler func(ctx context.Context, event *domain.AlertEvent) error) error
	Stop()
}
