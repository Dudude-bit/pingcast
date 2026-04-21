package domain

import (
	"time"

	"github.com/google/uuid"
)

// IncidentUpdate is a single status post attached to an incident. Each
// one narrates a state transition ("we've identified the DB saturation")
// and is surfaced on the public status page as a timeline entry.
type IncidentUpdate struct {
	ID             int64
	IncidentID     int64
	State          IncidentState
	Body           string
	PostedByUserID uuid.UUID
	PostedAt       time.Time
}
