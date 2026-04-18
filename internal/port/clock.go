package port

import "time"

// Clock is the source of wall-clock time used by application services.
// Production impl wraps time.Now; test impl is deterministic.
type Clock interface {
	Now() time.Time
}
