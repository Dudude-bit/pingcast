package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/xcontext"
)

var _ port.MonitorEventSubscriber = (*MonitorSubscriber)(nil)
var _ port.AlertEventSubscriber = (*AlertSubscriber)(nil)

// MonitorSubscriber subscribes to monitor change events from NATS JetStream.
type MonitorSubscriber struct {
	js   jetstream.JetStream
	cons jetstream.ConsumeContext
}

func NewMonitorSubscriber(js jetstream.JetStream) *MonitorSubscriber {
	return &MonitorSubscriber{js: js}
}

func (s *MonitorSubscriber) Subscribe(ctx context.Context, handler func(ctx context.Context, event port.MonitorChangedEvent) error) error {
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, "MONITORS", jetstream.ConsumerConfig{
		Durable:    "checker-worker",
		AckPolicy:  jetstream.AckExplicitPolicy,
		MaxDeliver: 5,
		BackOff:    []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second},
	})
	if err != nil {
		return fmt.Errorf("create consumer checker-worker: %w", err)
	}

	cons, err := consumer.Consume(func(msg jetstream.Msg) {
		var event port.MonitorChangedEvent
		if umErr := json.Unmarshal(msg.Data(), &event); umErr != nil {
			slog.Error("unmarshal monitor changed event — discarding malformed message", "error", umErr)
			_ = msg.Ack()
			return
		}

		msgCtx, cancel := xcontext.Detached(ctx, 5*time.Second, "nats.monitor.handle")
		defer cancel()

		if hErr := handler(msgCtx, event); hErr != nil {
			slog.Error("handle monitor changed event", "error", hErr)
			_ = msg.Nak()
			return
		}

		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("consume MONITORS: %w", err)
	}

	s.cons = cons
	return nil
}

func (s *MonitorSubscriber) Stop() {
	if s.cons != nil {
		s.cons.Stop()
	}
}

// AlertSubscriber subscribes to alert events from NATS JetStream.
type AlertSubscriber struct {
	js   jetstream.JetStream
	cons jetstream.ConsumeContext
}

func NewAlertSubscriber(js jetstream.JetStream) *AlertSubscriber {
	return &AlertSubscriber{js: js}
}

func (s *AlertSubscriber) Subscribe(ctx context.Context, handler func(ctx context.Context, event *domain.AlertEvent) error) error {
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, "ALERTS", jetstream.ConsumerConfig{
		Durable:    "notifier-alerts",
		AckPolicy:  jetstream.AckExplicitPolicy,
		MaxDeliver: 10,
		BackOff: []time.Duration{
			2 * time.Second, 5 * time.Second, 10 * time.Second,
			30 * time.Second, 60 * time.Second, 120 * time.Second,
			120 * time.Second, 120 * time.Second, 120 * time.Second, 120 * time.Second,
		},
	})
	if err != nil {
		return fmt.Errorf("create consumer notifier-alerts: %w", err)
	}

	cons, err := consumer.Consume(func(msg jetstream.Msg) {
		var event domain.AlertEvent
		if umErr := json.Unmarshal(msg.Data(), &event); umErr != nil {
			slog.Error("unmarshal alert event — discarding malformed message", "error", umErr)
			_ = msg.Ack()
			return
		}

		msgCtx, cancel := xcontext.Detached(ctx, 30*time.Second, "nats.alert.handle")
		defer cancel()

		if hErr := handler(msgCtx, &event); hErr != nil {
			slog.Error("handle alert event", "error", hErr)
			_ = msg.Nak()
			return
		}

		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("consume ALERTS: %w", err)
	}

	s.cons = cons
	return nil
}

func (s *AlertSubscriber) Stop() {
	if s.cons != nil {
		s.cons.Stop()
	}
}
