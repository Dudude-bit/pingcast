package bootstrap

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	goredis "github.com/redis/go-redis/v9"

	"github.com/kirillinakin/pingcast/internal/adapter/channel"
	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	httpadapter "github.com/kirillinakin/pingcast/internal/adapter/http"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	smtpadapter "github.com/kirillinakin/pingcast/internal/adapter/smtp"
	"github.com/kirillinakin/pingcast/internal/adapter/telegram"
	"github.com/kirillinakin/pingcast/internal/adapter/webhook"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

// AppDeps bundles every infrastructure handle the API app needs. The
// caller (main or integration harness) owns the lifecycle of these
// handles — NewApp does not open or close them.
type AppDeps struct {
	Pool   *pgxpool.Pool
	Redis  *goredis.Client
	NATS   *nats.Conn
	JS     jetstream.JetStream
	Cipher port.Cipher

	LemonSqueezySecret string

	// Optional overrides. If nil, defaults are used.
	Metrics *observability.Metrics
}

// App bundles the wired Fiber app with service handles that integration
// tests sometimes need (e.g. to bypass HTTP and manipulate state).
type App struct {
	Fiber *fiber.App

	AuthSvc    *app.AuthService
	MonitorSvc *app.MonitoringService
	AlertSvc   *app.AlertService

	Health *httpadapter.HealthChecker

	// Repos exposed for inspection/assertion in tests. Production main
	// does not use these fields directly.
	UserRepo       port.UserRepo
	SessionRepo    port.SessionRepo
	MonitorRepo    port.MonitorRepo
	ChannelRepo    port.ChannelRepo
	APIKeyRepo     port.APIKeyRepo
	IncidentRepo   port.IncidentRepo
	CheckResultRepo port.CheckResultRepo
}

// NewApp wires the full API composition and returns the Fiber app plus
// service handles. Panics on internal invariant violations (shouldn't
// happen in practice); returns an error only for setup-time checks
// that are recoverable.
func NewApp(deps AppDeps) (*App, error) {
	queries := sqlcgen.New(deps.Pool)

	metrics := deps.Metrics
	if metrics == nil {
		metrics = observability.NewMetrics()
	}

	// Repos
	userRepo := postgres.NewUserRepo(queries)
	sessionRepo := redisadapter.NewSessionRepo(deps.Redis)
	monitorRepo := postgres.NewMonitorRepo(deps.Pool, queries, deps.Cipher)
	channelRepo := postgres.NewChannelRepo(deps.Pool, queries, deps.Cipher)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)
	txm := postgres.NewTxManager(deps.Pool)
	apiKeyRepo := postgres.NewAPIKeyRepo(queries)
	failedAlertRepo := postgres.NewFailedAlertRepo(queries)

	// NATS publishers
	monitorPub := natsadapter.NewMonitorPublisher(deps.JS)
	alertPub := natsadapter.NewAlertPublisher(deps.JS)

	// Checker registry
	checkerReg := checker.NewRegistry()
	checkerReg.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPChecker())
	checkerReg.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker(10*time.Second))
	checkerReg.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

	// Channel registry (validation-only; actual sends happen in notifier)
	channelReg := channel.NewRegistry()
	channelReg.Register(domain.ChannelTelegram, "Telegram", telegram.NewFactory(""))
	channelReg.Register(domain.ChannelEmail, "Email", smtpadapter.NewFactory("", 0, "", "", ""))
	channelReg.Register(domain.ChannelWebhook, "Webhook", webhook.NewFactory())

	// App services
	authSvc := app.NewAuthService(userRepo, sessionRepo)
	monitoringSvc := app.NewMonitoringService(
		monitorRepo, channelRepo, checkResultRepo, incidentRepo,
		userRepo, uptimeRepo, txm, alertPub, monitorPub, checkerReg, metrics,
	)
	alertSvc := app.NewAlertService(channelRepo, monitorRepo, channelReg, failedAlertRepo, metrics)

	// Rate limiter (shared bucket for auth endpoints, 5/15min keyed per-target)
	rateLimiter := redisadapter.NewRateLimiter(deps.Redis, "auth", 5, 15*time.Minute)

	// HTTP handlers
	server := httpadapter.NewServer(authSvc, monitoringSvc, alertSvc, rateLimiter, apiKeyRepo)
	pageHandler := httpadapter.NewPageHandler(authSvc, monitoringSvc, alertSvc, rateLimiter, apiKeyRepo)
	webhookHandler := httpadapter.NewWebhookHandler(authSvc, alertSvc, deps.LemonSqueezySecret)
	healthChecker := httpadapter.NewHealthChecker(deps.Pool, deps.Redis, deps.NATS)

	fiberApp := httpadapter.SetupApp(authSvc, pageHandler, server, webhookHandler, apiKeyRepo, healthChecker)

	return &App{
		Fiber:           fiberApp,
		AuthSvc:         authSvc,
		MonitorSvc:      monitoringSvc,
		AlertSvc:        alertSvc,
		Health:          healthChecker,
		UserRepo:        userRepo,
		SessionRepo:     sessionRepo,
		MonitorRepo:     monitorRepo,
		ChannelRepo:     channelRepo,
		APIKeyRepo:      apiKeyRepo,
		IncidentRepo:    incidentRepo,
		CheckResultRepo: checkResultRepo,
	}, nil
}
