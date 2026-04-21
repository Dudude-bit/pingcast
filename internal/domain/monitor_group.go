package domain

import (
	"time"

	"github.com/google/uuid"
)

// MonitorGroup is an optional grouping for a user's monitors on the
// public status page. Zero or one group per monitor; any number of
// groups per user.
type MonitorGroup struct {
	ID        int64
	UserID    uuid.UUID
	Name      string
	Ordering  int
	CreatedAt time.Time
}
