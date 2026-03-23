package channel

import (
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing, reject requests
	StateHalfOpen              // Testing if service recovered
)

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu              sync.Mutex
	state           State
	failures        int
	successes       int
	maxFailures     int
	resetTimeout    time.Duration
	halfOpenMax     int
	lastFailureTime time.Time
}

// NewCircuitBreaker creates a circuit breaker.
// maxFailures: consecutive failures before opening circuit.
// resetTimeout: how long to wait before half-open.
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		halfOpenMax:  1,
	}
}

// Allow checks if the request should proceed.
// Returns error if circuit is open.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil
	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.successes = 0
			return nil
		}
		return fmt.Errorf("circuit breaker open")
	case StateHalfOpen:
		return nil
	}
	return nil
}

// RecordSuccess records a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	if cb.state == StateHalfOpen {
		cb.successes++
		if cb.successes >= cb.halfOpenMax {
			cb.state = StateClosed
		}
	}
}

// RecordFailure records a failed call.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.state == StateHalfOpen || cb.failures >= cb.maxFailures {
		cb.state = StateOpen
	}
}

// State returns the current state. For metrics/logging.
func (cb *CircuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
