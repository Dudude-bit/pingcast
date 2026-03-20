package natsadapter

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func Connect(url string) (*nats.Conn, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectBufSize(8*1024*1024),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Error("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("nats reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}
	return nc, nil
}

func SetupStreams(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "MONITORS",
		Subjects:  []string{"monitors.changed"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("create MONITORS stream: %w", err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "ALERTS",
		Subjects:  []string{"alerts.>"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("create ALERTS stream: %w", err)
	}

	return nil
}
