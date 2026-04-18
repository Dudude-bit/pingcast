package sysclock

import "time"

// Clock is the stdlib-backed production impl of port.Clock.
type Clock struct{}

func New() Clock { return Clock{} }

func (Clock) Now() time.Time { return time.Now() }
