package port

import (
	"context"
	"time"
)

// Metrics records application-level business metrics.
// Implementations must be safe for concurrent use.
// A nil Metrics (or a no-op implementation) is valid and silently discards all recordings.
type Metrics interface {
	// RecordCheck records a completed health check.
	// monitorType is "http", "tcp", or "dns"; status is "up" or "down"; duration is the check latency.
	RecordCheck(ctx context.Context, monitorType, status string, duration time.Duration)

	// RecordAlertSent records a single alert delivery attempt.
	// channelType is "telegram", "email", or "webhook"; success indicates whether delivery succeeded.
	// reason classifies the failure (empty string on success): "timeout", "auth_error", "rate_limited", "network_error", "unknown".
	RecordAlertSent(ctx context.Context, channelType string, success bool, reason string)

	// RecordAlertAllFailed records that all channels failed for a single alert event.
	RecordAlertAllFailed(ctx context.Context)

	// RecordAlertDeadLettered records that an alert was moved to the dead-letter queue.
	RecordAlertDeadLettered(ctx context.Context)

	// MonitorCreated increments the active-monitors gauge.
	MonitorCreated(ctx context.Context)

	// MonitorDeleted decrements the active-monitors gauge.
	MonitorDeleted(ctx context.Context)

	// IncidentOpened increments the open-incidents gauge.
	IncidentOpened(ctx context.Context)

	// IncidentResolved decrements the open-incidents gauge.
	IncidentResolved(ctx context.Context)
}
