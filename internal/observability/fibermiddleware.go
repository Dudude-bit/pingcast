package observability

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// NewFiberTracing returns a Fiber middleware that creates an OTel span per request.
// Fiber uses fasthttp (not net/http), so the standard otelhttp middleware does not apply.
func NewFiberTracing() fiber.Handler {
	tracer := otel.Tracer("pingcast-http")
	return func(c *fiber.Ctx) error {
		ctx, span := tracer.Start(c.UserContext(), fmt.Sprintf("%s %s", c.Method(), c.Path()),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()
		c.SetUserContext(ctx)

		err := c.Next()

		span.SetAttributes(
			attribute.Int("http.status_code", c.Response().StatusCode()),
			attribute.String("http.method", c.Method()),
			attribute.String("http.route", c.Path()),
		)
		if err != nil {
			span.RecordError(err)
		}
		return err
	}
}
