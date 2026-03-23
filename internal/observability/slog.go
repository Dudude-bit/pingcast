package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// tracingHandler wraps an slog.Handler to inject trace_id and span_id
// from the OpenTelemetry span context into every log record.
type tracingHandler struct {
	inner slog.Handler
}

// NewTracingHandler wraps an slog.Handler to add trace_id and span_id from OTel context.
func NewTracingHandler(inner slog.Handler) slog.Handler {
	return &tracingHandler{inner: inner}
}

func (h *tracingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *tracingHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, record)
}

func (h *tracingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &tracingHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *tracingHandler) WithGroup(name string) slog.Handler {
	return &tracingHandler{inner: h.inner.WithGroup(name)}
}
