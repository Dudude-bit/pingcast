package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	goredis "github.com/redis/go-redis/v9"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	"github.com/kirillinakin/pingcast/internal/app"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type SchedulerDeps struct {
	Pool   *pgxpool.Pool
	Redis  *goredis.Client
	JS     jetstream.JetStream
	Cipher port.Cipher

	RetentionDays int

	// CertProvisionerConfig must mirror what the API got — without it
	// the scheduler-driven validation + renewal loops would call the
	// noop provisioner, marking domains "active" without an actual
	// cert and leaving renewals as no-ops.
	CertProvisionerConfig

	// SSLScanEnabled enables the daily Pro-tier SSL-expiry probe. On by
	// default; tests flip it off to avoid real TCP dials. The ticker
	// cadence is fixed at 24h — misses only matter for certs already
	// within 24h of expiry, which surface on the next scan.
	SSLScanEnabled bool

	// SkipLeaderElection is TEST-ONLY. When true, the leader loop skips
	// the distributed-lock dance and behaves as if this instance is
	// always leader — so the harness can call DispatchAll synchronously.
	// Cannot be set via env; only by the integration harness.
	SkipLeaderElection bool
}

type Scheduler struct {
	Leader *checker.LeaderScheduler

	monitorSub       *natsadapter.MonitorSubscriber
	cleanupFunc      func(context.Context)
	sslScanFunc      func(context.Context)
	customDomainFunc func(context.Context)
	certRenewalFunc  func(context.Context)
	cleanupCancel    context.CancelFunc
	cleanupWg        sync.WaitGroup
}

func NewScheduler(deps SchedulerDeps) (*Scheduler, error) {
	queries := sqlcgen.New(deps.Pool)
	monitorRepo := postgres.NewMonitorRepo(deps.Pool, queries, deps.Cipher)
	checkResultRepo := postgres.NewCheckResultRepo(queries)

	rs := redisadapter.NewRedsync(deps.Redis)
	checkPub := natsadapter.NewCheckPublisher(deps.JS)

	hostname, _ := os.Hostname()
	instanceID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	var mutex port.DistributedMutex
	if deps.SkipLeaderElection {
		mutex = noopMutex{}
	} else {
		mutex = rs.NewMutex("lock:scheduler:leader", redsync.WithExpiry(10*time.Second))
	}

	leader := checker.NewLeaderScheduler(mutex, checkPub, instanceID)

	// Load existing monitors up-front so tests observe a non-empty scheduler.
	ctx := context.Background()
	active, err := monitorRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("load active monitors: %w", err)
	}
	for i := range active {
		leader.Add(&active[i])
	}

	s := &Scheduler{Leader: leader}

	// Monitor change subscriber — same logic as the old cmd/scheduler/main.go
	s.monitorSub = natsadapter.NewMonitorSubscriber(deps.JS)
	if err := s.monitorSub.Subscribe(ctx, func(_ context.Context, ev port.MonitorChangedEvent) error {
		switch ev.Action {
		case domain.ActionCreate, domain.ActionUpdate, domain.ActionResume:
			leader.Add(&domain.Monitor{
				ID:                 ev.MonitorID,
				Name:               ev.Name,
				Type:               ev.Type,
				CheckConfig:        ev.CheckConfig,
				IntervalSeconds:    ev.IntervalSeconds,
				AlertAfterFailures: ev.AlertAfterFailures,
				IsPaused:           ev.IsPaused,
			})
		case domain.ActionDelete, domain.ActionPause:
			leader.Remove(ev.MonitorID)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("subscribe monitor changes: %w", err)
	}

	// Retention cleanup closure — launched by Start().
	cleanupMutex := rs.NewMutex("lock:cleanup:retention", redsync.WithExpiry(1*time.Hour))
	s.cleanupFunc = func(ctx context.Context) {
		defer s.cleanupWg.Done()
		runLockedLoop(ctx, cleanupMutex, 24*time.Hour, "cleanup", func(ctx context.Context) {
			// Plan-aware retention (sprint 2 §6): Free users keep 30
			// days; Pro users keep 365. RETENTION_DAYS is still the
			// Free baseline so ops can tune it without a code change.
			freeCutoff := time.Now().Add(-time.Duration(deps.RetentionDays) * 24 * time.Hour)
			proCutoff := time.Now().AddDate(-1, 0, 0)
			deleted, err := checkResultRepo.DeleteByPlan(ctx, freeCutoff, proCutoff)
			if err != nil {
				slog.Error("retention cleanup failed", "error", err)
			} else if deleted > 0 {
				slog.Info("retention cleanup",
					"deleted_rows", deleted,
					"free_cutoff", freeCutoff, "pro_cutoff", proCutoff)
			}
			rangeStart := time.Date(time.Now().Year(), time.Now().Month()+1, 1, 0, 0, 0, 0, time.UTC)
			rangeEnd := rangeStart.AddDate(0, 1, 0)
			safeName := pgx.Identifier{fmt.Sprintf("check_results_%d_%02d", rangeStart.Year(), rangeStart.Month())}.Sanitize()
			ddl := fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS %s PARTITION OF check_results FOR VALUES FROM ('%s') TO ('%s')",
				safeName, rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02"),
			)
			if _, err := deps.Pool.Exec(ctx, ddl); err != nil {
				slog.Error("partition creation failed", "error", err)
			}
		})
	}

	// Daily SSL-expiry scan closure — Pro-only. Sprint 2 S2T2: probes
	// the TLS cert of every non-paused HTTP monitor owned by a Pro
	// user and publishes ssl_expiring alerts at 14/7/1 days remaining.
	if deps.SSLScanEnabled {
		alertPub := natsadapter.NewAlertPublisher(deps.JS)
		sslMutex := rs.NewMutex("lock:cleanup:ssl-scan", redsync.WithExpiry(1*time.Hour))
		s.sslScanFunc = func(ctx context.Context) {
			defer s.cleanupWg.Done()
			runLockedLoop(ctx, sslMutex, 24*time.Hour, "ssl-scan", func(ctx context.Context) {
				runSSLScan(ctx, monitorRepo, alertPub)
			})
		}
	}

	// Custom-domain validation loop — advances pending/validated/failed
	// rows through their state machine every 60s. Leader-elected via a
	// separate redsync mutex so only one scheduler instance drives the
	// worker. ACME provisioning is stubbed (NoopCertProvisioner) today;
	// flip to a real adapter when prod Traefik/ACME is wired.
	customDomainRepo := postgres.NewCustomDomainRepo(queries)
	customDomainCertRepo := postgres.NewCustomDomainCertRepo(queries)
	certProvisioner, certErr := buildCertProvisioner(deps.CertProvisionerConfig, customDomainCertRepo, customDomainRepo)
	if certErr != nil {
		// Same fallback policy as API bootstrap: log loud, run with
		// noop. Otherwise a misconfigured ACME account would crash
		// the whole scheduler instead of letting validation continue.
		slog.Error("acme provisioner setup failed — falling back to noop",
			"provider", deps.CertProvider, "error", certErr)
		certProvisioner = app.NoopCertProvisioner{}
	}
	customDomainSvc := app.NewCustomDomainService(customDomainRepo, customDomainCertRepo, certProvisioner, "")
	cdMutex := rs.NewMutex("lock:custom-domain:validate", redsync.WithExpiry(2*time.Minute))
	s.customDomainFunc = func(ctx context.Context) {
		defer s.cleanupWg.Done()
		runLockedLoop(ctx, cdMutex, 60*time.Second, "custom-domain", func(ctx context.Context) {
			customDomainSvc.RunValidationOnce(ctx)
		})
	}

	// Cert-renewal loop — once a day, leader-elected. Lego certs live
	// 90 days; we renew at day-60 (i.e. when expires_at < now+30d) so a
	// transient ACME outage doesn't expire prod certs. The loop is
	// idempotent — re-issuing a still-valid cert is fine.
	renewalMutex := rs.NewMutex("lock:custom-domain:renew", redsync.WithExpiry(15*time.Minute))
	s.certRenewalFunc = func(ctx context.Context) {
		defer s.cleanupWg.Done()
		runLockedLoop(ctx, renewalMutex, 24*time.Hour, "cert-renewal", func(ctx context.Context) {
			customDomainSvc.RunRenewalsOnce(ctx)
		})
	}

	return s, nil
}

// runLockedLoop is the shared shape behind every scheduler tick:
// acquire a redsync mutex (skip-if-held), run the work, release. All
// four periodic jobs (retention cleanup, SSL scan, custom-domain
// validation, cert renewal) used to inline this dance, with subtle
// log-message variations and one-off bugs introducing drift. One
// helper keeps them honest.
//
// Caller is responsible for the surrounding sync.WaitGroup bookkeeping
// (Add before goroutine launch, Done via defer in the goroutine).
func runLockedLoop(
	ctx context.Context,
	mutex *redsync.Mutex,
	interval time.Duration,
	name string,
	work func(context.Context),
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := mutex.Lock(); err != nil {
				if errors.Is(err, redsync.ErrFailed) {
					continue
				}
				slog.Warn(name+" lock failed", "error", err)
				continue
			}
			work(ctx)
			if _, err := mutex.Unlock(); err != nil {
				slog.Warn(name+" unlock failed", "error", err)
			}
		}
	}
}

// runSSLScan probes every Pro-tier HTTP monitor's cert and publishes a
// ssl_expiring alert when days-remaining crosses a 14/7/1 threshold.
// Extracted so tests can drive it with fakes instead of waiting for the
// 24h ticker.
func runSSLScan(
	ctx context.Context,
	monitorRepo port.MonitorRepo,
	alertPub port.AlertEventPublisher,
) {
	monitors, err := monitorRepo.ListProHTTPForSSLScan(ctx)
	if err != nil {
		slog.Error("ssl scan: list monitors", "error", err)
		return
	}
	now := time.Now()
	alerted := 0
	for _, m := range monitors {
		url, ok := extractHTTPURL(m.CheckConfig)
		if !ok {
			continue
		}
		notAfter, err := checker.CheckSSLExpiry(ctx, url)
		if err != nil {
			slog.Warn("ssl probe failed", "monitor_id", m.ID, "error", err)
			continue
		}
		days := checker.DaysUntilExpiry(notAfter, now)
		// Thresholds: 14, 7, 1. Single-day windows so each daily run
		// fires at most one alert per monitor.
		if days != 14 && days != 7 && days != 1 {
			continue
		}
		cause := fmt.Sprintf("TLS certificate for %s expires in %d days (at %s)",
			url, days, notAfter.UTC().Format(time.RFC3339))
		if err := alertPub.PublishAlert(ctx, &domain.AlertEvent{
			MonitorID:     m.ID,
			UserID:        m.UserID,
			MonitorName:   m.Name,
			MonitorTarget: url,
			Event:         domain.AlertSSLExpiring,
			Cause:         cause,
		}); err != nil {
			slog.Error("ssl alert publish failed",
				"monitor_id", m.ID, "days", days, "error", err)
			continue
		}
		alerted++
	}
	slog.Info("ssl scan complete",
		"monitors_scanned", len(monitors), "alerts_published", alerted)
}

// extractHTTPURL pulls the `url` field from an http monitor's
// check_config JSON, which is the probe target. Returns ok=false if
// absent (malformed config or non-http monitor leaking through).
func extractHTTPURL(cfg []byte) (string, bool) {
	var m map[string]interface{}
	if err := json.Unmarshal(cfg, &m); err != nil {
		return "", false
	}
	url, _ := m["url"].(string)
	if url == "" {
		return "", false
	}
	// Only probe https URLs — http:// has no TLS cert.
	if !strings.HasPrefix(url, "https://") {
		return "", false
	}
	return url, true
}

// Start launches the leader scheduler loop and the retention cleanup
// goroutine. Non-blocking.
func (s *Scheduler) Start(ctx context.Context) {
	cleanupCtx, cancel := context.WithCancel(ctx)
	s.cleanupCancel = cancel

	s.cleanupWg.Add(1)
	go s.cleanupFunc(cleanupCtx)

	if s.sslScanFunc != nil {
		s.cleanupWg.Add(1)
		go s.sslScanFunc(cleanupCtx)
	}

	if s.customDomainFunc != nil {
		s.cleanupWg.Add(1)
		go s.customDomainFunc(cleanupCtx)
	}

	if s.certRenewalFunc != nil {
		s.cleanupWg.Add(1)
		go s.certRenewalFunc(cleanupCtx)
	}

	go s.Leader.Run(ctx)
}

// Stop signals all loops and waits for them to exit.
func (s *Scheduler) Stop(shutdownCtx context.Context) {
	if s.monitorSub != nil {
		s.monitorSub.Stop()
	}
	s.Leader.Stop()
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}
	done := make(chan struct{})
	go func() {
		s.cleanupWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-shutdownCtx.Done():
	}
}

// noopMutex implements port.DistributedMutex with no-op Lock/Extend/Unlock.
// Used only by the integration harness via SchedulerDeps.SkipLeaderElection.
type noopMutex struct{}

func (noopMutex) Lock() error           { return nil }
func (noopMutex) Extend() (bool, error) { return true, nil }
func (noopMutex) Unlock() (bool, error) { return true, nil }
