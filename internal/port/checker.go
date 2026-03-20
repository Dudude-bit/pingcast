package port

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/domain"
)

// MonitorChecker performs health checks against a monitor's endpoint.
// Returns a CheckResult with Status up/down — a "down" result is not a Go error.
// Error is returned only for infrastructure failures (e.g., invalid monitor config).
// Implementations: HTTP checker, TCP checker, DNS checker, etc.
type MonitorChecker interface {
	Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult
}
