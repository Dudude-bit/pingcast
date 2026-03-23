package channel

import (
	"context"
	"log/slog"
	"time"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// RetryingSender wraps an AlertSender with retry logic and circuit breaker.
type RetryingSender struct {
	inner   port.AlertSender
	cb      *CircuitBreaker
	retries int
	backoff []time.Duration
}

// NewRetryingSender wraps a sender with 3 retries (1s, 2s, 4s) and a circuit breaker.
func NewRetryingSender(inner port.AlertSender, cb *CircuitBreaker) *RetryingSender {
	return &RetryingSender{
		inner:   inner,
		cb:      cb,
		retries: 3,
		backoff: []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second},
	}
}

func (s *RetryingSender) Send(ctx context.Context, event *domain.AlertEvent) error {
	if err := s.cb.Allow(); err != nil {
		return err
	}

	var lastErr error
	for attempt := range s.retries {
		lastErr = s.inner.Send(ctx, event)
		if lastErr == nil {
			s.cb.RecordSuccess()
			return nil
		}

		slog.Warn("alert send failed, retrying",
			"attempt", attempt+1,
			"max_retries", s.retries,
			"error", lastErr,
		)

		if attempt < s.retries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.backoff[attempt]):
			}
		}
	}

	s.cb.RecordFailure()
	return lastErr
}
