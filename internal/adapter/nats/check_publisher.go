package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// CheckPublisher publishes check tasks to the CHECKS stream.
// Used by the scheduler to dispatch work to checker workers.
type CheckPublisher struct {
	js jetstream.JetStream
}

func NewCheckPublisher(js jetstream.JetStream) *CheckPublisher {
	return &CheckPublisher{js: js}
}

type checkTaskMessage struct {
	MonitorID uuid.UUID `json:"monitor_id"`
}

// Publish publishes a check task for a monitor.
func (p *CheckPublisher) Publish(ctx context.Context, monitorID uuid.UUID) error {
	data, err := json.Marshal(checkTaskMessage{MonitorID: monitorID})
	if err != nil {
		return fmt.Errorf("marshal check task: %w", err)
	}
	_, err = p.js.Publish(ctx, "checks.run", data)
	if err != nil {
		return fmt.Errorf("publish check task: %w", err)
	}
	return nil
}
