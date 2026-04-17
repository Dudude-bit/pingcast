package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.MonitorEventPublisher = (*MonitorPublisher)(nil)
var _ port.AlertEventPublisher = (*AlertPublisher)(nil)

// MonitorPublisher publishes monitor change events to NATS JetStream.
type MonitorPublisher struct {
	js jetstream.JetStream
}

func NewMonitorPublisher(js jetstream.JetStream) *MonitorPublisher {
	return &MonitorPublisher{js: js}
}

func (p *MonitorPublisher) PublishMonitorChanged(ctx context.Context, event port.MonitorChangedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal monitor changed event: %w", err)
	}

	_, err = p.js.Publish(ctx, "monitors.changed", data)
	if err != nil {
		return fmt.Errorf("publish monitor changed event: %w", err)
	}

	return nil
}

// AlertPublisher publishes alert events to NATS JetStream.
type AlertPublisher struct {
	js jetstream.JetStream
}

func NewAlertPublisher(js jetstream.JetStream) *AlertPublisher {
	return &AlertPublisher{js: js}
}

func (p *AlertPublisher) PublishAlert(ctx context.Context, event *domain.AlertEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal alert event: %w", err)
	}

	subject := fmt.Sprintf("alerts.%s", event.Event)

	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publish alert event: %w", err)
	}

	return nil
}
