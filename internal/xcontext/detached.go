package xcontext

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("xcontext")

// Detached creates a new context that is NOT cancelled when the parent is cancelled.
// Useful for background operations (NATS message handlers, async writes) that must
// complete even if the request/app context is shutting down.
//
// A linked OTel span is created so detached operations are correlated in Grafana
// without reusing the (possibly finished) parent span.
func Detached(parent context.Context, timeout time.Duration, spanName string) (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if parentSpan := trace.SpanFromContext(parent); parentSpan.SpanContext().IsValid() {
		ctx, _ = tracer.Start(ctx, spanName,
			trace.WithLinks(trace.Link{SpanContext: parentSpan.SpanContext()}))
	}
	return context.WithTimeout(ctx, timeout)
}
