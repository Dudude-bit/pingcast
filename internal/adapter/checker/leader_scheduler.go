package checker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// LeaderScheduler is a scheduler that only runs on the leader instance.
// Uses port.DistributedMutex for leader election with fencing via Extend().
type LeaderScheduler struct {
	mutex      port.DistributedMutex
	publisher  port.CheckPublisher
	instanceID string
	monitors   map[uuid.UUID]*scheduledMonitor
	mu         sync.Mutex
	isLeader   atomic.Bool
	ctx        context.Context
	cancel     context.CancelFunc
}

type scheduledMonitor struct {
	monitor  *domain.Monitor
	lastTick time.Time
}

func NewLeaderScheduler(mutex port.DistributedMutex, publisher port.CheckPublisher, instanceID string) *LeaderScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &LeaderScheduler{
		mutex:      mutex,
		publisher:  publisher,
		instanceID: instanceID,
		monitors:   make(map[uuid.UUID]*scheduledMonitor),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (s *LeaderScheduler) Add(m *domain.Monitor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.monitors[m.ID] = &scheduledMonitor{monitor: m}
}

func (s *LeaderScheduler) Remove(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.monitors, id)
}

// Run starts the scheduler loop. Blocks until context is cancelled.
func (s *LeaderScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if !s.isLeader.Load() {
				s.tryAcquireLeadership()
				continue
			}

			// Fencing: extend lock BEFORE dispatching.
			// If extend fails, step down immediately — do NOT dispatch.
			ok, err := s.mutex.Extend()
			if err != nil || !ok {
				slog.Warn("lost leader lock, stepping down", "instance", s.instanceID, "error", err)
				s.isLeader.Store(false)
				continue
			}

			s.dispatchDueChecks(ctx)
		}
	}
}

func (s *LeaderScheduler) tryAcquireLeadership() {
	// Lock() may block with retries. Redsync default: ~8s total.
	// This is acceptable — non-leader ticks are idle anyway.
	if err := s.mutex.Lock(); err != nil {
		return
	}
	slog.Info("became scheduler leader", "instance", s.instanceID)
	s.isLeader.Store(true)
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

// DispatchAll publishes a check task for every non-paused monitor
// regardless of lastTick. Tests use this to force a deterministic
// fan-out without running the ticker/leader loop. Production code
// paths use Run() → dispatchDueChecks() unchanged.
func (s *LeaderScheduler) DispatchAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sm := range s.monitors {
		if sm.monitor.IsPaused {
			continue
		}
		if err := s.publisher.Publish(ctx, sm.monitor.ID); err != nil {
			return err
		}
	}
	return nil
}

// MonitorCount returns the number of monitors currently registered.
// Tests use it to verify Add/Remove bookkeeping without reaching into
// the unexported map.
func (s *LeaderScheduler) MonitorCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.monitors)
}

// Stop stops the scheduler. Releases the lock only if currently leader.
func (s *LeaderScheduler) Stop() {
	s.cancel()
	if s.isLeader.Load() {
		if _, err := s.mutex.Unlock(); err != nil {
			slog.Warn("failed to release leader lock on shutdown", "instance", s.instanceID, "error", err)
		}
		s.isLeader.Store(false)
	}
}
