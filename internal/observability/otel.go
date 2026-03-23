package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Setup initializes OpenTelemetry with OTLP exporters.
// Returns a shutdown function that should be called on application exit.
// If otelEndpoint is empty, OTel is disabled (noop).
//
// Metrics are exported via OTLP gRPC to the OTel Collector, which exposes
// a Prometheus scrape endpoint on port 8889 (see deploy/otel/collector-config.yaml).
// No /metrics endpoint is needed on the application itself.
func Setup(ctx context.Context, serviceName, otelEndpoint string) (func(context.Context) error, error) {
	if otelEndpoint == "" {
		slog.Info("otel disabled: OTEL_EXPORTER_OTLP_ENDPOINT not set")
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	// Trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otelEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter, trace.WithBatchTimeout(5*time.Second)),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metric exporter
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(otelEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(15*time.Second))),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	slog.Info("otel initialized", "service", serviceName, "endpoint", otelEndpoint)

	shutdown := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown trace provider: %w", err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown metric provider: %w", err)
		}
		return nil
	}

	return shutdown, nil
}
