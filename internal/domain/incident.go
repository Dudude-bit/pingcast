package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// IncidentState is the public lifecycle a manual incident moves through
// on a status page. Auto-detected incidents ride the same enum but only
// transition investigating → resolved.
type IncidentState string

const (
	IncidentStateInvestigating IncidentState = "investigating"
	IncidentStateIdentified    IncidentState = "identified"
	IncidentStateMonitoring    IncidentState = "monitoring"
	IncidentStateResolved      IncidentState = "resolved"
)

func (s IncidentState) Valid() bool {
	switch s {
	case IncidentStateInvestigating, IncidentStateIdentified,
		IncidentStateMonitoring, IncidentStateResolved:
		return true
	}
	return false
}

// CanTransitionTo enforces the public-facing lifecycle. Forward moves
// are always allowed; the only rewind we permit is monitoring → identified
// (an engineer realising they jumped the gun before declaring resolved).
// Everything else is rejected.
func (s IncidentState) CanTransitionTo(next IncidentState) error {
	if !next.Valid() {
		return fmt.Errorf("invalid incident state: %q", next)
	}
	if s == next {
		return nil
	}
	order := map[IncidentState]int{
		IncidentStateInvestigating: 0,
		IncidentStateIdentified:    1,
		IncidentStateMonitoring:    2,
		IncidentStateResolved:      3,
	}
	if order[next] > order[s] {
		return nil
	}
	if s == IncidentStateMonitoring && next == IncidentStateIdentified {
		return nil
	}
	return fmt.Errorf("cannot move incident from %s to %s", s, next)
}

type Incident struct {
	ID         int64
	MonitorID  uuid.UUID
	StartedAt  time.Time
	ResolvedAt *time.Time
	Cause      string
	State      IncidentState
	IsManual   bool
	Title      *string
}

func (i Incident) IsResolved() bool {
	return i.ResolvedAt != nil
}
