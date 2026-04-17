package httpadapter

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/port"
)

func SetupApp(
	authService *app.AuthService,
	pageHandler *PageHandler,
	server *Server,
	webhookHandler *WebhookHandler,
	apiKeyRepo port.APIKeyRepo,
	healthChecker *HealthChecker,
) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Domain errors → structured JSON response
			var domErr *domain.DomainError
			if errors.As(err, &domErr) {
				code := fiber.StatusInternalServerError
				switch {
				case errors.Is(domErr, domain.ErrNotFound):
					code = fiber.StatusNotFound
				case errors.Is(domErr, domain.ErrForbidden):
					code = fiber.StatusForbidden
				case errors.Is(domErr, domain.ErrValidation):
					code = fiber.StatusUnprocessableEntity
				case errors.Is(domErr, domain.ErrConflict):
					code = fiber.StatusConflict
				}
				return c.Status(code).JSON(fiber.Map{
					"error": fiber.Map{
						"code":    domErr.Code,
						"message": domErr.Message,
					},
				})
			}

			// Fiber errors
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			slog.Error("unhandled error", "error", err, "code", code)
			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "INTERNAL_ERROR",
					"message": "internal error",
				},
			})
		},
	})

	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(recover.New())
	app.Use(observability.NewFiberTracing())

	// CSRF protection for HTML form submissions.
	// JSON API endpoints are exempt (they use Authorization header, not cookies).
	app.Use(csrf.New(csrf.Config{
		KeyLookup:      "form:_csrf",
		CookieName:     "csrf_",
		CookieSameSite: "Lax",
		CookieHTTPOnly: true,
		ContextKey:     "csrf",
		Next: func(c *fiber.Ctx) bool {
			// Skip CSRF for JSON API, webhooks, and health endpoints
			path := c.Path()
			if len(path) >= 4 && path[:4] == "/api" {
				return true
			}
			if len(path) >= 8 && path[:8] == "/webhook" {
				return true
			}
			if path == "/health" {
				return true
			}
			return false
		},
	}))

	// Health / readiness
	app.Get("/health", server.HealthCheck)
	app.Get("/healthz", healthChecker.Healthz)
	app.Get("/readyz", healthChecker.Readyz)

	// Logout form-POST (still Go-side because it must clear the session cookie).
	app.Post("/logout", pageHandler.Logout)

	// Webhooks (no auth)
	app.Post("/webhook/lemonsqueezy", webhookHandler.HandleLemonSqueezy)
	app.Post("/webhook/telegram/:token", webhookHandler.HandleTelegramWebhook)

	// JSON API — uses oapi-codegen generated RegisterHandlers
	// Auth endpoints are public, monitor endpoints need auth middleware.
	apigen.RegisterHandlersWithOptions(app, server, apigen.FiberServerOptions{
		Middlewares: []apigen.MiddlewareFunc{
			authMiddlewareSelector(authService, apiKeyRepo),
		},
	})

	// /api/monitor-types is registered by oapi-codegen — no manual registration needed
	// /dashboard, /monitors/* HTML pages are served by the Next.js frontend (C2).

	// Authenticated HTML pages — all migrated to Next.js (C3).
	// Only the public status page and logout POST remain on Go.

	return app
}

// authMiddlewareSelector returns a middleware that applies auth only to /api/ routes.
// Non-API routes (HTML pages, webhooks, health, static) are passed through —
// they have their own auth via PageMiddleware.
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
