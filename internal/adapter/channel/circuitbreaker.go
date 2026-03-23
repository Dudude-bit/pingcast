package channel

import (
	"time"

	"github.com/sony/gobreaker/v2"
)

// NewCircuitBreaker creates a gobreaker circuit breaker for a channel type.
// maxFailures: consecutive failures before opening.
// resetTimeout: how long circuit stays open before half-open.
func NewCircuitBreaker(name string, maxFailures uint32, resetTimeout time.Duration) *gobreaker.CircuitBreaker[any] {
	return gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,
		Interval:    0,
		Timeout:     resetTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= maxFailures
		},
	})
}
