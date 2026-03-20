package checker_test

import (
	"encoding/json"
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
