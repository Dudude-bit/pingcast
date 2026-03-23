package checker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// CheckPublisher is the interface for publishing check tasks.
type CheckPublisher interface {
	Publish(ctx context.Context, monitorID uuid.UUID) error
}

// LeaderScheduler is a scheduler that only runs on the leader instance.
// Uses port.DistributedMutex for leader election.
type LeaderScheduler struct {
	mutex     port.DistributedMutex
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

func NewLeaderScheduler(mutex port.DistributedMutex, publisher CheckPublisher) *LeaderScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &LeaderScheduler{
		mutex:     mutex,
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

	isLeader := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if !isLeader {
				// Try to acquire leadership
				if err := s.mutex.Lock(); err != nil {
					continue // another instance holds the lock
				}
				slog.Info("became scheduler leader")
				isLeader = true
			} else {
				// Refresh lock (fencing) — extend TTL
				ok, err := s.mutex.Extend()
				if err != nil || !ok {
					slog.Warn("failed to refresh leader lock, stepping down", "error", err)
					isLeader = false
					continue
				}
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

// Stop stops the scheduler and releases the lock.
func (s *LeaderScheduler) Stop() {
	s.cancel()
	s.mutex.Unlock()
}
