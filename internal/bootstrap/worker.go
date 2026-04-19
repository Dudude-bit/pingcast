package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	"github.com/kirillinakin/pingcast/internal/adapter/sysclock"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type WorkerDeps struct {
	Pool   *pgxpool.Pool
	JS     jetstream.JetStream
	Cipher port.Cipher

	DefaultTimeoutSecs int

	// Optional Clock override. If nil, sysclock.New() is used.
	Clock port.Clock
}

type Worker struct {
	MonitoringSvc *app.MonitoringService

	checkSub *natsadapter.CheckSubscriber
}

func NewWorker(deps WorkerDeps) (*Worker, error) {
	queries := sqlcgen.New(deps.Pool)

	userRepo := postgres.NewUserRepo(queries)
	monitorRepo := postgres.NewMonitorRepo(deps.Pool, queries, deps.Cipher)
	channelRepo := postgres.NewChannelRepo(deps.Pool, queries, deps.Cipher)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)
	txm := postgres.NewTxManager(deps.Pool)

	alertPub := natsadapter.NewAlertPublisher(deps.JS)

	timeout := time.Duration(deps.DefaultTimeoutSecs) * time.Second
	registry := checker.NewRegistry()
	registry.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPCheckerWithTimeout(deps.DefaultTimeoutSecs))
	registry.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker(timeout))
	registry.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

	metrics := observability.NewMetrics()

	clock := deps.Clock
	if clock == nil {
		clock = sysclock.New()
	}

	svc := app.NewMonitoringService(
		monitorRepo, channelRepo, checkResultRepo, incidentRepo,
		userRepo, uptimeRepo, txm, alertPub, nil, registry, metrics, clock,
	)

	w := &Worker{
		MonitoringSvc: svc,
		checkSub:      natsadapter.NewCheckSubscriber(deps.JS),
	}

	handler := func(ctx context.Context, monitorID uuid.UUID) error {
		mon, err := monitorRepo.GetByID(ctx, monitorID)
		if err != nil {
			return fmt.Errorf("get monitor %s: %w", monitorID, err)
		}
		return svc.RunCheck(ctx, mon)
	}

	if err := w.checkSub.Subscribe(context.Background(), handler); err != nil {
		return nil, fmt.Errorf("subscribe checks: %w", err)
	}

	return w, nil
}

func (w *Worker) Stop() {
	w.checkSub.Stop()
}
