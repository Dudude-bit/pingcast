package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// Metrics holds all application-level metric instruments backed by OpenTelemetry.
// It implements port.Metrics.
type Metrics struct {
	checksTotal        otelmetric.Int64Counter
	checkDuration      otelmetric.Float64Histogram
	alertsSentTotal    otelmetric.Int64Counter
	alertsFailedTotal  otelmetric.Int64Counter
	alertsDeadLettered otelmetric.Int64Counter
	monitorsActive     otelmetric.Int64UpDownCounter
	incidentsOpen      otelmetric.Int64UpDownCounter
}

// NewMetrics creates all OTel metric instruments using the global meter provider.
func NewMetrics() *Metrics {
	meter := otel.Meter("pingcast")

	checksTotal, _ := meter.Int64Counter("pingcast.checks.total",
		otelmetric.WithDescription("Total number of health checks executed"),
		otelmetric.WithUnit("{check}"),
	)

	checkDuration, _ := meter.Float64Histogram("pingcast.checks.duration",
		otelmetric.WithDescription("Health check latency"),
		otelmetric.WithUnit("s"),
	)

	alertsSentTotal, _ := meter.Int64Counter("pingcast.alerts.sent.total",
		otelmetric.WithDescription("Total alert delivery attempts per channel"),
		otelmetric.WithUnit("{alert}"),
	)

	alertsFailedTotal, _ := meter.Int64Counter("pingcast.alerts.all_failed.total",
		otelmetric.WithDescription("Total alert events where all channels failed"),
		otelmetric.WithUnit("{event}"),
	)

	alertsDeadLettered, _ := meter.Int64Counter("pingcast.alerts.dead_lettered.total",
		otelmetric.WithDescription("Total alerts moved to dead-letter queue"),
		otelmetric.WithUnit("{alert}"),
	)

	monitorsActive, _ := meter.Int64UpDownCounter("pingcast.monitors.active",
		otelmetric.WithDescription("Number of currently active monitors"),
		otelmetric.WithUnit("{monitor}"),
	)

	incidentsOpen, _ := meter.Int64UpDownCounter("pingcast.incidents.open",
		otelmetric.WithDescription("Number of currently open incidents"),
		otelmetric.WithUnit("{incident}"),
	)

	return &Metrics{
		checksTotal:        checksTotal,
		checkDuration:      checkDuration,
		alertsSentTotal:    alertsSentTotal,
		alertsFailedTotal:  alertsFailedTotal,
		alertsDeadLettered: alertsDeadLettered,
		monitorsActive:     monitorsActive,
		incidentsOpen:      incidentsOpen,
	}
}

func (m *Metrics) RecordCheck(ctx context.Context, monitorType, status string, duration time.Duration) {
	attrs := otelmetric.WithAttributes(
		attribute.String("type", monitorType),
		attribute.String("status", status),
	)
	m.checksTotal.Add(ctx, 1, attrs)
	m.checkDuration.Record(ctx, duration.Seconds(), otelmetric.WithAttributes(
		attribute.String("type", monitorType),
	))
}

func (m *Metrics) RecordAlertSent(ctx context.Context, channelType string, success bool, reason string) {
	status := "success"
	if !success {
		status = "failure"
	}
	attrs := []attribute.KeyValue{
		attribute.String("channel_type", channelType),
		attribute.String("status", status),
	}
	if reason != "" {
		attrs = append(attrs, attribute.String("reason", reason))
	}
	m.alertsSentTotal.Add(ctx, 1, otelmetric.WithAttributes(attrs...))
}

func (m *Metrics) RecordAlertAllFailed(ctx context.Context) {
	m.alertsFailedTotal.Add(ctx, 1)
}

func (m *Metrics) RecordAlertDeadLettered(ctx context.Context) {
	m.alertsDeadLettered.Add(ctx, 1)
}

func (m *Metrics) MonitorCreated(ctx context.Context) {
	m.monitorsActive.Add(ctx, 1)
}

func (m *Metrics) MonitorDeleted(ctx context.Context) {
	m.monitorsActive.Add(ctx, -1)
}

func (m *Metrics) IncidentOpened(ctx context.Context) {
	m.incidentsOpen.Add(ctx, 1)
}

func (m *Metrics) IncidentResolved(ctx context.Context) {
	m.incidentsOpen.Add(ctx, -1)
}
