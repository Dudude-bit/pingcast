package natsadapter

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nats-io/nats.go"
)

// DLQConsumer listens for NATS JetStream max delivery advisory events
// and writes failed alerts to the database.
type DLQConsumer struct {
	nc      *nats.Conn
	sub     *nats.Subscription
	handler func(ctx context.Context, streamName, consumerName string, streamSeq uint64, data []byte) error
}

// MaxDeliveryAdvisory represents a NATS max delivery advisory message.
type MaxDeliveryAdvisory struct {
	Type      string `json:"type"`
	Stream    string `json:"stream"`
	Consumer  string `json:"consumer"`
	StreamSeq uint64 `json:"stream_seq"`
}

func NewDLQConsumer(nc *nats.Conn) *DLQConsumer {
	return &DLQConsumer{nc: nc}
}

// Subscribe starts listening for max delivery advisories on the ALERTS stream.
// The handler is called with the failed event data for persistence to failed_alerts table.
func (d *DLQConsumer) Subscribe(ctx context.Context, handler func(ctx context.Context, streamName, consumerName string, streamSeq uint64, data []byte) error) error {
	d.handler = handler

	sub, err := d.nc.Subscribe("$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.>", func(msg *nats.Msg) {
		var advisory MaxDeliveryAdvisory
		if err := json.Unmarshal(msg.Data, &advisory); err != nil {
			slog.Error("failed to unmarshal DLQ advisory", "error", err)
			return
		}

		// Only handle alerts stream advisories
		if advisory.Stream != "ALERTS" {
			return
		}

		slog.Error("alert delivery exhausted",
			"stream", advisory.Stream,
			"consumer", advisory.Consumer,
			"stream_seq", advisory.StreamSeq,
		)

		if d.handler != nil {
			if err := d.handler(ctx, advisory.Stream, advisory.Consumer, advisory.StreamSeq, msg.Data); err != nil {
				slog.Error("failed to handle DLQ advisory", "error", err)
			}
		}
	})
	if err != nil {
		return err
	}
	d.sub = sub
	return nil
}

func (d *DLQConsumer) Stop() {
	if d.sub != nil {
		d.sub.Unsubscribe()
	}
}
