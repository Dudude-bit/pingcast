package channel

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/sony/gobreaker/v2"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// RetryingSender wraps an AlertSender with retry (cenkalti/backoff) and circuit breaker (sony/gobreaker).
type RetryingSender struct {
	inner port.AlertSender
	cb    *gobreaker.CircuitBreaker[any]
}

// NewRetryingSender wraps a sender with exponential backoff retry and circuit breaker.
func NewRetryingSender(inner port.AlertSender, cb *gobreaker.CircuitBreaker[any]) *RetryingSender {
	return &RetryingSender{inner: inner, cb: cb}
}

func (s *RetryingSender) Send(ctx context.Context, event *domain.AlertEvent) error {
	_, err := s.cb.Execute(func() (any, error) {
		b := backoff.NewExponentialBackOff()
		b.InitialInterval = 1 * time.Second
		b.MaxInterval = 4 * time.Second

		_, err := backoff.Retry(ctx, backoff.Operation[struct{}](func() (struct{}, error) {
			return struct{}{}, s.inner.Send(ctx, event)
		}), backoff.WithBackOff(b), backoff.WithMaxTries(3))

		return nil, err
	})
	return err
}
