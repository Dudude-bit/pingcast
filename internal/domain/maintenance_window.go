package domain

import (
	"time"

	"github.com/google/uuid"
)

// MaintenanceWindow marks a time range during which failures on a
// monitor should NOT produce alerts or open incidents. The status page
// renders "Scheduled maintenance" for any monitor inside an active
// window.
type MaintenanceWindow struct {
	ID        int64
	MonitorID uuid.UUID
	StartsAt  time.Time
	EndsAt    time.Time
	Reason    string
	CreatedAt time.Time
}

func (w MaintenanceWindow) ActiveAt(now time.Time) bool {
	return !now.Before(w.StartsAt) && now.Before(w.EndsAt)
}
