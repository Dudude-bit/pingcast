package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	goredis "github.com/redis/go-redis/v9"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
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

	// SkipLeaderElection is TEST-ONLY. When true, the leader loop skips
	// the distributed-lock dance and behaves as if this instance is
	// always leader — so the harness can call DispatchAll synchronously.
	// Cannot be set via env; only by the integration harness.
	SkipLeaderElection bool
}

type Scheduler struct {
	Leader *checker.LeaderScheduler

	monitorSub    *natsadapter.MonitorSubscriber
	cleanupFunc   func(context.Context)
	cleanupCancel context.CancelFunc
	cleanupWg     sync.WaitGroup
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
	s.cleanupFunc = func(ctx context.Context) {
		defer s.cleanupWg.Done()

		cleanupMutex := rs.NewMutex("lock:cleanup:retention", redsync.WithExpiry(1*time.Hour))
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := cleanupMutex.Lock(); err != nil {
					if errors.Is(err, redsync.ErrFailed) {
						continue
					}
					slog.Warn("cleanup lock failed", "error", err)
					continue
				}
				cutoff := time.Now().Add(-time.Duration(deps.RetentionDays) * 24 * time.Hour)
				deleted, err := checkResultRepo.DeleteOlderThan(ctx, cutoff)
				if err != nil {
					slog.Error("retention cleanup failed", "error", err)
				} else if deleted > 0 {
					slog.Info("retention cleanup", "deleted_rows", deleted)
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
				if _, err := cleanupMutex.Unlock(); err != nil {
					slog.Warn("cleanup unlock failed", "error", err)
				}
			}
		}
	}

	return s, nil
}

// Start launches the leader scheduler loop and the retention cleanup
// goroutine. Non-blocking.
func (s *Scheduler) Start(ctx context.Context) {
	cleanupCtx, cancel := context.WithCancel(ctx)
	s.cleanupCancel = cancel

	s.cleanupWg.Add(1)
	go s.cleanupFunc(cleanupCtx)

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
