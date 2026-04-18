package httpadapter

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/port"
)

func SetupApp(
	authService *app.AuthService,
	server *Server,
	webhookHandler *WebhookHandler,
	apiKeyRepo port.APIKeyRepo,
	healthChecker *HealthChecker,
) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return httperr.Write(c, err)
		},
	})

	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(recover.New())
	app.Use(observability.NewFiberTracing())

	// CSRF middleware was removed after the Next.js migration (spec §8.6).
	// The frontend talks to the API via JSON + Authorization header
	// exclusively; there are no browser form-POST routes left to guard.

	// Health / readiness
	app.Get("/health", server.HealthCheck)
	app.Get("/healthz", healthChecker.Healthz)
	app.Get("/readyz", healthChecker.Readyz)

	// /logout page form-POST was removed post-Next.js migration (spec §8.6).
	// The only logout endpoint is POST /api/auth/logout.

	// Webhooks (no auth — HMAC-protected by handler; spec §7)
	app.Post("/webhook/lemonsqueezy", webhookHandler.HandleLemonSqueezy)
	app.Post("/webhook/telegram/:token", webhookHandler.HandleTelegramWebhook)

	// JSON API — uses oapi-codegen generated RegisterHandlers.
	// Auth middleware gates every /api/* path except register, login,
	// and the public status page.
	apigen.RegisterHandlersWithOptions(app, server, apigen.FiberServerOptions{
		Middlewares: []apigen.MiddlewareFunc{
			authMiddlewareSelector(authService, apiKeyRepo),
		},
	})

	return app
}

// authMiddlewareSelector returns a middleware that applies auth only to /api/ routes.
// Non-API routes (webhooks, health, static) are passed through — they
// have their own auth or are intentionally public.
func authMiddlewareSelector(authService *app.AuthService, apiKeyRepo port.APIKeyRepo) apigen.MiddlewareFunc {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Only apply auth to /api/* routes — everything else has its own middleware
		if len(path) < 5 || path[:5] != "/api/" {
			return c.Next()
		}

		// Public API endpoints (no auth needed)
		if path == "/api/auth/register" || path == "/api/auth/login" ||
			(len(path) > 12 && path[:12] == "/api/status/") {
			return c.Next()
		}

		// All other /api/ routes need auth (session cookie or API key)
		return AuthMiddleware(authService, apiKeyRepo)(c)
	}
}
