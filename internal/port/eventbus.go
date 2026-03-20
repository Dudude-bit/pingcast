package port

import (
	"context"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

type MonitorEventPublisher interface {
	PublishMonitorChanged(ctx context.Context, action domain.MonitorAction, monitorID uuid.UUID, monitor *domain.Monitor) error
}

type AlertEventPublisher interface {
	PublishAlert(ctx context.Context, event *domain.AlertEvent) error
}

type MonitorEventSubscriber interface {
	Subscribe(ctx context.Context, handler func(ctx context.Context, action domain.MonitorAction, monitorID uuid.UUID, monitor *domain.Monitor) error) error
	Stop()
}

type AlertEventSubscriber interface {
	Subscribe(ctx context.Context, handler func(ctx context.Context, event *domain.AlertEvent) error) error
	Stop()
}
