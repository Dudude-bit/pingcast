package checker

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type DispatchFunc func(m *MonitorInfo)

type schedulerEntry struct {
	monitor *MonitorInfo
	ticker  *time.Ticker
	stop    chan struct{}
}

type Scheduler struct {
	mu       sync.Mutex
	entries  map[uuid.UUID]*schedulerEntry
	dispatch DispatchFunc
}

func NewScheduler(dispatch DispatchFunc) *Scheduler {
	return &Scheduler{
		entries:  make(map[uuid.UUID]*schedulerEntry),
		dispatch: dispatch,
	}
}

func (s *Scheduler) Add(m *MonitorInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.entries[m.ID]; ok {
		entry.ticker.Stop()
		close(entry.stop)
		delete(s.entries, m.ID)
	}

	interval := time.Duration(m.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	stopCh := make(chan struct{})

	entry := &schedulerEntry{
		monitor: m,
		ticker:  ticker,
		stop:    stopCh,
	}
	s.entries[m.ID] = entry

	go func() {
		for {
			select {
			case <-ticker.C:
				s.dispatch(m)
			case <-stopCh:
				return
			}
		}
	}()
}

func (s *Scheduler) Remove(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.entries[id]; ok {
		entry.ticker.Stop()
		close(entry.stop)
		delete(s.entries, id)
	}
}

func (s *Scheduler) Update(m *MonitorInfo) {
	s.Add(m)
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entry := range s.entries {
		entry.ticker.Stop()
		close(entry.stop)
		delete(s.entries, id)
	}
}

func (s *Scheduler) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
