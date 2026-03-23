package checker_test

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func TestScheduler_AddsAndFires(t *testing.T) {
	fired := make(chan uuid.UUID, 10)

	s := checker.NewScheduler(func(m *domain.Monitor) {
		fired <- m.ID
	})

	id := uuid.New()
	s.Add(&domain.Monitor{
		ID:              id,
		Type:            domain.MonitorHTTP,
		CheckConfig:     json.RawMessage(`{"url":"http://localhost"}`),
		IntervalSeconds: 1,
	})
	defer s.Stop()

	select {
	case got := <-fired:
		if got != id {
			t.Errorf("fired monitor id = %v, want %v", got, id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for scheduler to fire")
	}
}

func TestScheduler_Remove(t *testing.T) {
	fired := make(chan uuid.UUID, 10)

	s := checker.NewScheduler(func(m *domain.Monitor) {
		fired <- m.ID
	})

	id := uuid.New()
	s.Add(&domain.Monitor{
		ID:              id,
		Type:            domain.MonitorHTTP,
		CheckConfig:     json.RawMessage(`{"url":"http://localhost"}`),
		IntervalSeconds: 1,
	})
	s.Remove(id)

	select {
	case <-fired:
		t.Fatal("scheduler fired after remove")
	case <-time.After(2 * time.Second):
		// expected
	}

	s.Stop()
}

func TestScheduler_AddMultipleRemoveOne(t *testing.T) {
	fired := make(chan uuid.UUID, 20)

	s := checker.NewScheduler(func(m *domain.Monitor) {
		fired <- m.ID
	})
	defer s.Stop()

	id1 := uuid.New()
	id2 := uuid.New()

	s.Add(&domain.Monitor{
		ID:              id1,
		Type:            domain.MonitorHTTP,
		CheckConfig:     json.RawMessage(`{"url":"http://localhost"}`),
		IntervalSeconds: 1,
	})
	s.Add(&domain.Monitor{
		ID:              id2,
		Type:            domain.MonitorHTTP,
		CheckConfig:     json.RawMessage(`{"url":"http://localhost"}`),
		IntervalSeconds: 1,
	})

	// Remove monitor 1; monitor 2 should still fire.
	s.Remove(id1)

	// Wait for at least one fire event.
	select {
	case got := <-fired:
		if got == id1 {
			t.Errorf("removed monitor %v should not fire", id1)
		}
		if got != id2 {
			t.Errorf("fired monitor id = %v, want %v", got, id2)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for remaining monitor to fire")
	}
}

func TestScheduler_Count(t *testing.T) {
	s := checker.NewScheduler(func(m *domain.Monitor) {})
	defer s.Stop()

	if s.Count() != 0 {
		t.Errorf("count = %d, want 0", s.Count())
	}

	id1 := uuid.New()
	s.Add(&domain.Monitor{
		ID:              id1,
		Type:            domain.MonitorHTTP,
		CheckConfig:     json.RawMessage(`{"url":"http://localhost"}`),
		IntervalSeconds: 1,
	})
	if s.Count() != 1 {
		t.Errorf("count = %d, want 1", s.Count())
	}

	s.Remove(id1)
	if s.Count() != 0 {
		t.Errorf("count = %d, want 0 after remove", s.Count())
	}
}

// --- LeaderScheduler tests (paused monitors) ---

// stubMutex implements port.DistributedMutex for testing.
type stubMutex struct{}

func (m *stubMutex) Lock() error              { return nil }
func (m *stubMutex) Unlock() (bool, error)    { return true, nil }
func (m *stubMutex) Extend() (bool, error)    { return true, nil }

// stubPublisher implements checker.CheckPublisher for testing.
type stubPublisher struct {
	mu        sync.Mutex
	published []uuid.UUID
}

func (p *stubPublisher) Publish(_ context.Context, id uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, id)
	return nil
}

func (p *stubPublisher) get() []uuid.UUID {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]uuid.UUID, len(p.published))
	copy(out, p.published)
	return out
}

func TestLeaderScheduler_PausedMonitorsNotDispatched(t *testing.T) {
	pub := &stubPublisher{}
	ls := checker.NewLeaderScheduler(&stubMutex{}, pub)

	activeID := uuid.New()
	pausedID := uuid.New()

	ls.Add(&domain.Monitor{
		ID:              activeID,
		Type:            domain.MonitorHTTP,
		CheckConfig:     json.RawMessage(`{"url":"http://localhost"}`),
		IntervalSeconds: 1,
		IsPaused:        false,
	})
	ls.Add(&domain.Monitor{
		ID:              pausedID,
		Type:            domain.MonitorHTTP,
		CheckConfig:     json.RawMessage(`{"url":"http://localhost"}`),
		IntervalSeconds: 1,
		IsPaused:        true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go ls.Run(ctx)

	// Poll until active monitor is published (instead of fixed sleep).
	deadline := time.After(5 * time.Second)
	for {
		published := pub.get()
		hasActive := slices.Contains(published, activeID)
		if hasActive {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for active monitor to be dispatched")
		case <-time.After(100 * time.Millisecond):
		}
	}

	cancel()
	ls.Stop()

	// Verify paused monitor was never dispatched.
	published := pub.get()
	if slices.Contains(published, pausedID) {
		t.Errorf("paused monitor %v should not be dispatched", pausedID)
	}
}
