package bootstrap

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/adapter/channel"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	smtpadapter "github.com/kirillinakin/pingcast/internal/adapter/smtp"
	"github.com/kirillinakin/pingcast/internal/adapter/telegram"
	"github.com/kirillinakin/pingcast/internal/adapter/webhook"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type NotifierDeps struct {
	Pool   *pgxpool.Pool
	NATS   *nats.Conn
	JS     jetstream.JetStream
	Cipher port.Cipher

	TelegramToken string
	SMTPHost      string
	SMTPPort      int
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string

	// SendOverrides is test-only. When non-nil, AlertService routes
	// delivery through these instead of the Telegram/SMTP/Webhook
	// factories. Production passes nil.
	SendOverrides map[domain.ChannelType]port.AlertSender
}

type Notifier struct {
	AlertSvc *app.AlertService

	alertSub    *natsadapter.AlertSubscriber
	dlqConsumer *natsadapter.DLQConsumer
}

func NewNotifier(deps NotifierDeps) (*Notifier, error) {
	queries := sqlcgen.New(deps.Pool)
	channelRepo := postgres.NewChannelRepo(deps.Pool, queries, deps.Cipher)
	monitorRepo := postgres.NewMonitorRepo(deps.Pool, queries, deps.Cipher)
	failedAlertRepo := postgres.NewFailedAlertRepo(queries)

	reg := channel.NewRegistry()
	if deps.TelegramToken != "" {
		reg.Register(domain.ChannelTelegram, "Telegram", telegram.NewFactory(deps.TelegramToken))
	}
	if deps.SMTPHost != "" {
		reg.Register(domain.ChannelEmail, "Email",
			smtpadapter.NewFactory(deps.SMTPHost, deps.SMTPPort, deps.SMTPUser, deps.SMTPPass, deps.SMTPFrom))
	}
	reg.Register(domain.ChannelWebhook, "Webhook", webhook.NewFactory())

	metrics := observability.NewMetrics()
	svc := app.NewAlertService(channelRepo, monitorRepo, reg, failedAlertRepo, metrics)
	if deps.SendOverrides != nil {
		svc = svc.WithSendOverrides(deps.SendOverrides)
	}

	n := &Notifier{AlertSvc: svc}

	n.alertSub = natsadapter.NewAlertSubscriber(deps.JS)
	if err := n.alertSub.Subscribe(context.Background(), func(ctx context.Context, event *domain.AlertEvent) error {
		return svc.Handle(ctx, event)
	}); err != nil {
		return nil, fmt.Errorf("alert subscribe: %w", err)
	}

	n.dlqConsumer = natsadapter.NewDLQConsumer(deps.NATS)
	if err := n.dlqConsumer.Subscribe(context.Background(),
		func(ctx context.Context, streamName, consumerName string, seq uint64, data []byte) error {
			msg := fmt.Sprintf("max deliveries exhausted: stream=%s consumer=%s seq=%d", streamName, consumerName, seq)
			return failedAlertRepo.Create(ctx, data, msg, nil)
		}); err != nil {
		return nil, fmt.Errorf("dlq subscribe: %w", err)
	}

	return n, nil
}

func (n *Notifier) Stop() {
	n.alertSub.Stop()
	n.dlqConsumer.Stop()
}
