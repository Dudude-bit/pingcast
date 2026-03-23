package httpadapter

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
)

// HealthChecker provides health and readiness endpoints.
type HealthChecker struct {
	db    *pgxpool.Pool
	redis *goredis.Client
	nats  *nats.Conn
}

func NewHealthChecker(db *pgxpool.Pool, redis *goredis.Client, nc *nats.Conn) *HealthChecker {
	return &HealthChecker{db: db, redis: redis, nats: nc}
}

// Healthz checks all dependencies. Returns 200 if all healthy, 503 if any unhealthy.
func (h *HealthChecker) Healthz(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
	defer cancel()

	checks := map[string]string{}
	healthy := true

	// Postgres
	if err := h.db.Ping(ctx); err != nil {
		checks["postgres"] = "unhealthy: " + err.Error()
		healthy = false
	} else {
		checks["postgres"] = "ok"
	}

	// Redis
	if err := h.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		healthy = false
	} else {
		checks["redis"] = "ok"
	}

	// NATS
	if !h.nats.IsConnected() {
		checks["nats"] = "unhealthy: disconnected"
		healthy = false
	} else {
		checks["nats"] = "ok"
	}

	status := fiber.StatusOK
	if !healthy {
		status = fiber.StatusServiceUnavailable
	}

	return c.Status(status).JSON(fiber.Map{
		"status": map[bool]string{true: "healthy", false: "unhealthy"}[healthy],
		"checks": checks,
	})
}

// Readyz returns 200 when the service is ready to accept traffic.
func (h *HealthChecker) Readyz(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ready"})
}
