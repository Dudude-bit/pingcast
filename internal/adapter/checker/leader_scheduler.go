package checker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/domain"
	goredis "github.com/redis/go-redis/v9"
)

const (
	schedulerLockKey = "scheduler:leader"
	schedulerLockTTL = 30 * time.Second
)

// CheckPublisher is the interface for publishing check tasks.
type CheckPublisher interface {
	Publish(ctx context.Context, monitorID uuid.UUID) error
}

// LeaderScheduler is a scheduler that only runs on the leader instance.
// Uses Redis lock for leader election with fencing.
type LeaderScheduler struct {
	rdb       *goredis.Client
	publisher CheckPublisher
	monitors  map[uuid.UUID]*scheduledMonitor
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
}

type scheduledMonitor struct {
	monitor  *domain.Monitor
	lastTick time.Time
}

func NewLeaderScheduler(rdb *goredis.Client, publisher CheckPublisher) *LeaderScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &LeaderScheduler{
		rdb:       rdb,
		publisher: publisher,
		monitors:  make(map[uuid.UUID]*scheduledMonitor),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Add adds or updates a monitor in the schedule.
func (s *LeaderScheduler) Add(m *domain.Monitor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.monitors[m.ID] = &scheduledMonitor{monitor: m}
}

// Remove removes a monitor from the schedule.
func (s *LeaderScheduler) Remove(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.monitors, id)
}

// Run starts the scheduler loop. Blocks until context is cancelled.
func (s *LeaderScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	leaderTicker := time.NewTicker(10 * time.Second)
	defer leaderTicker.Stop()

	isLeader := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ctx.Done():
			return
		case <-leaderTicker.C:
			acquired, err := redisadapter.TryLock(ctx, s.rdb, schedulerLockKey, schedulerLockTTL)
			if err != nil {
				slog.Error("leader election failed", "error", err)
				isLeader = false
				continue
			}
			if acquired && !isLeader {
				slog.Info("became scheduler leader")
				isLeader = true
			} else if !acquired && isLeader {
				slog.Warn("lost scheduler leadership")
				isLeader = false
			}
		case <-ticker.C:
			if !isLeader {
				continue
			}

			// Refresh lock (fencing)
			acquired, err := redisadapter.TryLock(ctx, s.rdb, schedulerLockKey, schedulerLockTTL)
			if err != nil || !acquired {
				// Lock refresh failed — another instance may have taken over
				if isLeader {
					slog.Warn("failed to refresh leader lock, stepping down")
					isLeader = false
				}
				continue
			}

			s.dispatchDueChecks(ctx)
		}
	}
}

func (s *LeaderScheduler) dispatchDueChecks(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, sm := range s.monitors {
		if sm.monitor.IsPaused {
			continue
		}
		interval := time.Duration(sm.monitor.IntervalSeconds) * time.Second
		if now.Sub(sm.lastTick) < interval {
			continue
		}

		if err := s.publisher.Publish(ctx, sm.monitor.ID); err != nil {
			slog.Error("failed to publish check task",
				"monitor_id", sm.monitor.ID,
				"error", err,
			)
			continue
		}
		sm.lastTick = now
	}
}

// Stop stops the scheduler.
func (s *LeaderScheduler) Stop() {
	s.cancel()
}
