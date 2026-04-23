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
	"github.com/kirillinakin/pingcast/internal/adapter/sysclock"
	"github.com/kirillinakin/pingcast/internal/adapter/sysrand"
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

	LemonSqueezySecret        string
	LemonSqueezyFounderVariantID string
	LemonSqueezyRetailVariantID  string
	FounderCap                int

	// SMTP_* mirrors the notifier. Empty SMTPHost → logging noop.
	SMTPHost, SMTPUser, SMTPPass, SMTPFrom string
	SMTPPort                               int

	// BaseURL is the public site origin used for subscription confirm +
	// unsubscribe links in outbound emails. Defaults to
	// http://localhost:8080 via APIConfig.
	BaseURL string

	// Deterministic injection points. If nil, sysclock/sysrand defaults
	// are wired; tests override with Fake impls.
	Clock  port.Clock
	Random port.Random

	// RateLimits overrides per-scope bucket sizes and/or the window for
	// all scopes (used by integration tests to run burst scenarios in
	// seconds). Nil → production defaults from port.RateLimitDefaults.
	RateLimits *port.RateLimitConfig

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

	// Cipher is the encryption port handed to all repos. Exposed so
	// the integration harness can reuse the same cipher when composing
	// scheduler/worker/notifier against the same Postgres.
	Cipher port.Cipher

	// Repos exposed for inspection/assertion in tests. Production main
	// does not use these fields directly.
	UserRepo       port.UserRepo
	SessionRepo    port.SessionRepo
	MonitorRepo    port.MonitorRepo
	ChannelRepo    port.ChannelRepo
	APIKeyRepo     port.APIKeyRepo
	IncidentRepo       port.IncidentRepo
	IncidentUpdateRepo port.IncidentUpdateRepo
	CheckResultRepo    port.CheckResultRepo
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

	clock := deps.Clock
	if clock == nil {
		clock = sysclock.New()
	}
	rng := deps.Random
	if rng == nil {
		rng = sysrand.New()
	}

	// Repos
	userRepo := postgres.NewUserRepo(queries)
	sessionRepo := redisadapter.NewSessionRepo(deps.Redis)
	monitorRepo := postgres.NewMonitorRepo(deps.Pool, queries, deps.Cipher)
	channelRepo := postgres.NewChannelRepo(deps.Pool, queries, deps.Cipher)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	incidentUpdateRepo := postgres.NewIncidentUpdateRepo(queries)
	maintenanceRepo := postgres.NewMaintenanceWindowRepo(queries)
	monitorGroupRepo := postgres.NewMonitorGroupRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)
	txm := postgres.NewTxManager(deps.Pool)
	apiKeyRepo := postgres.NewAPIKeyRepo(queries)
	failedAlertRepo := postgres.NewFailedAlertRepo(queries)
	statsRepo := postgres.NewStatsRepo(queries)

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
	authSvc := app.NewAuthService(userRepo, sessionRepo, clock, rng)
	monitoringSvc := app.NewMonitoringService(
		monitorRepo, channelRepo, checkResultRepo, incidentRepo, incidentUpdateRepo, maintenanceRepo, monitorGroupRepo,
		userRepo, uptimeRepo, txm, alertPub, monitorPub, checkerReg, metrics,
		clock,
	)
	alertSvc := app.NewAlertService(channelRepo, monitorRepo, channelReg, failedAlertRepo, metrics)

	founderCap := deps.FounderCap
	if founderCap <= 0 {
		founderCap = 100 // matches .env.example default and spec §5
	}
	billingSvc := app.NewBillingService(userRepo, founderCap)
	atlassianImporter := app.NewAtlassianImporter(monitorRepo, incidentRepo, incidentUpdateRepo, txm, clock)
	statusSubRepo := postgres.NewStatusSubscriberRepo(queries)
	mailer := smtpadapter.NewMailer(deps.SMTPHost, deps.SMTPPort, deps.SMTPUser, deps.SMTPPass, deps.SMTPFrom)
	subscriptionsSvc := app.NewSubscriptionService(statusSubRepo, mailer, deps.BaseURL)

	customDomainRepo := postgres.NewCustomDomainRepo(queries)
	customDomainsSvc := app.NewCustomDomainService(customDomainRepo, app.NoopCertProvisioner{}, deps.BaseURL)

	// Per-scope rate limiters (spec §5). Each bucket has its own prefix
	// so they don't share keys, and its own max/window per scope. Tests
	// override windows via deps.RateLimits.WindowOverride.
	rls := buildRateLimiters(deps.Redis, deps.RateLimits)

	// HTTP handlers
	server := httpadapter.NewServer(authSvc, monitoringSvc, alertSvc, billingSvc, atlassianImporter, subscriptionsSvc, customDomainsSvc, rls, apiKeyRepo, statsRepo)
	webhookHandler := httpadapter.NewWebhookHandler(
		authSvc, alertSvc, billingSvc, deps.LemonSqueezySecret,
		deps.LemonSqueezyFounderVariantID,
	)
	healthChecker := httpadapter.NewHealthChecker(deps.Pool, deps.Redis, deps.NATS)

	fiberApp := httpadapter.SetupApp(authSvc, server, webhookHandler, apiKeyRepo, healthChecker, rls)

	_ = rls // silence if go vet flags rls as unused in some paths

	return &App{
		Fiber:           fiberApp,
		AuthSvc:         authSvc,
		MonitorSvc:      monitoringSvc,
		AlertSvc:        alertSvc,
		Health:          healthChecker,
		Cipher:          deps.Cipher,
		UserRepo:        userRepo,
		SessionRepo:     sessionRepo,
		MonitorRepo:     monitorRepo,
		ChannelRepo:     channelRepo,
		APIKeyRepo:      apiKeyRepo,
		IncidentRepo:       incidentRepo,
		IncidentUpdateRepo: incidentUpdateRepo,
		CheckResultRepo:    checkResultRepo,
	}, nil
}
