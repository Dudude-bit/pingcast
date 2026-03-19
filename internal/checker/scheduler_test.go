package checker_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/checker"
)

func TestScheduler_AddsAndFires(t *testing.T) {
	fired := make(chan uuid.UUID, 10)

	s := checker.NewScheduler(func(m *checker.MonitorInfo) {
		fired <- m.ID
	})

	id := uuid.New()
	s.Add(&checker.MonitorInfo{
		ID:              id,
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

	s := checker.NewScheduler(func(m *checker.MonitorInfo) {
		fired <- m.ID
	})

	id := uuid.New()
	s.Add(&checker.MonitorInfo{
		ID:              id,
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
