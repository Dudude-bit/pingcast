package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/config"
	natsbus "github.com/kirillinakin/pingcast/internal/nats"
	"github.com/kirillinakin/pingcast/internal/notifier"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadNotifier()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// NATS
	nc, err := natsbus.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer nc.Drain()

	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("failed to create jetstream context", "error", err)
		os.Exit(1)
	}

	if err := natsbus.SetupStreams(ctx, js); err != nil {
		slog.Error("failed to setup nats streams", "error", err)
		os.Exit(1)
	}

	// Senders
	var tgSender *notifier.TelegramSender
	if cfg.TelegramToken != "" {
		tgSender = notifier.NewTelegramSender(cfg.TelegramToken)
	}

	var emailSender *notifier.EmailSender
	if cfg.SMTPHost != "" {
		emailSender = notifier.NewEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
	}

	// Subscribe to alerts
	cons, err := js.CreateOrUpdateConsumer(ctx, "ALERTS", jetstream.ConsumerConfig{
		Durable:    "notifier-alerts",
		AckPolicy:  jetstream.AckExplicitPolicy,
		MaxDeliver: 5,
		AckWait:    60 * time.Second,
		BackOff:    []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute},
	})
	if err != nil {
		slog.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}

	consCtx, err := cons.Consume(func(msg jetstream.Msg) {
		var event natsbus.AlertEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			slog.Error("invalid alert event", "error", err)
			msg.Term()
			return
		}

		if err := handleAlert(&event, tgSender, emailSender); err != nil {
			slog.Error("alert delivery failed, will retry", "event", event.Event, "monitor_id", event.MonitorID, "error", err)
			msg.Nak()
			return
		}

		msg.Ack()
		slog.Info("alert delivered", "event", event.Event, "monitor_id", event.MonitorID)
	})
	if err != nil {
		slog.Error("failed to start consumer", "error", err)
		os.Exit(1)
	}

	slog.Info("notifier started")
	<-ctx.Done()
	slog.Info("notifier shutting down")
	consCtx.Stop()
}

func handleAlert(event *natsbus.AlertEvent, tg *notifier.TelegramSender, email *notifier.EmailSender) error {
	// Telegram
	if event.TgChatID != nil && tg != nil {
		var err error
		switch event.Event {
		case "down":
			err = tg.SendDown(*event.TgChatID, event.MonitorName, event.MonitorURL, event.Cause)
		case "up":
			err = tg.SendUp(*event.TgChatID, event.MonitorName, event.MonitorURL)
		}
		if err != nil {
			return err
		}
	}

	// Email (Pro only)
	if event.Plan == "pro" && event.Email != "" && email != nil {
		var err error
		switch event.Event {
		case "down":
			err = email.SendDown(event.Email, event.MonitorName, event.MonitorURL, event.Cause)
		case "up":
			err = email.SendUp(event.Email, event.MonitorName, event.MonitorURL)
		}
		if err != nil {
			return err
		}
	}

	return nil
}
