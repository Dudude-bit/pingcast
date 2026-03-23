package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// CheckSubscriber consumes check tasks from the CHECKS stream using pull-based consumption.
type CheckSubscriber struct {
	js   jetstream.JetStream
	cons jetstream.Consumer
	stop context.CancelFunc
}

func NewCheckSubscriber(js jetstream.JetStream) *CheckSubscriber {
	return &CheckSubscriber{js: js}
}

// Subscribe starts pulling check tasks. The handler receives monitor IDs to check.
// Uses pull-based consumption for natural backpressure.
func (s *CheckSubscriber) Subscribe(ctx context.Context, handler func(ctx context.Context, monitorID uuid.UUID) error) error {
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, "CHECKS", jetstream.ConsumerConfig{
		Durable:    "checker-workers",
		AckPolicy:  jetstream.AckExplicitPolicy,
		MaxDeliver: 3,
		AckWait:    60 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("create consumer checker-workers: %w", err)
	}
	s.cons = consumer

	pullCtx, cancel := context.WithCancel(ctx)
	s.stop = cancel

	go s.pullLoop(pullCtx, handler)

	return nil
}

func (s *CheckSubscriber) pullLoop(ctx context.Context, handler func(ctx context.Context, monitorID uuid.UUID) error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgs, err := s.cons.Fetch(10, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("fetch check tasks failed", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for msg := range msgs.Messages() {
			var task checkTaskMessage
			if err := json.Unmarshal(msg.Data(), &task); err != nil {
				slog.Error("unmarshal check task — discarding malformed message", "error", err)
				_ = msg.Ack() // Ack (discard): malformed JSON will never succeed on retry
				continue
			}

			if err := handler(ctx, task.MonitorID); err != nil {
				slog.Error("handle check task", "monitor_id", task.MonitorID, "error", err)
				_ = msg.NakWithDelay(5 * time.Second)
				continue
			}

			_ = msg.Ack()
		}
	}
}

func (s *CheckSubscriber) Stop() {
	if s.stop != nil {
		s.stop()
	}
}
