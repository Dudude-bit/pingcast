package domain

import (
	"time"

	"github.com/google/uuid"
)

type Incident struct {
	ID         int64
	MonitorID  uuid.UUID
	StartedAt  time.Time
	ResolvedAt *time.Time
	Cause      string
}

func (i Incident) IsResolved() bool {
	return i.ResolvedAt != nil
}
