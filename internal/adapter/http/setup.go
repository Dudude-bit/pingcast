package httpadapter

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/web"
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
			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "INTERNAL_ERROR",
					"message": err.Error(),
				},
			})
		},
	})

	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(recover.New())

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

	// Static files
	staticFS, _ := fs.Sub(web.FS, "static")
	app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(staticFS),
	}))

	// Health / readiness
	app.Get("/health", server.HealthCheck)
	app.Get("/healthz", healthChecker.Healthz)
	app.Get("/readyz", healthChecker.Readyz)

	// Public pages
	app.Get("/", pageHandler.Landing)
	app.Get("/login", pageHandler.LoginPage)
	app.Post("/login", pageHandler.LoginSubmit)
	app.Get("/register", pageHandler.RegisterPage)
	app.Post("/register", pageHandler.RegisterSubmit)
	app.Post("/logout", pageHandler.Logout)

	// Public status page (HTML)
	app.Get("/status/:slug", pageHandler.StatusPage)

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

	// Monitor type config (HTMX partial, authenticated)
	app.Get("/monitors/config-fields", PageMiddleware(authService), pageHandler.MonitorConfigFields)

	// API: list available monitor types
	app.Get("/api/monitor-types", server.ListMonitorTypes)

	// Authenticated HTML pages
	authed := app.Group("", PageMiddleware(authService))
	authed.Get("/dashboard", pageHandler.Dashboard)
	authed.Get("/monitors/new", pageHandler.MonitorNewForm)
	authed.Post("/monitors", pageHandler.MonitorCreate)
	authed.Post("/monitors/:id/pause", pageHandler.MonitorTogglePause)
	authed.Post("/monitors/:id", pageHandler.MonitorUpdate)
	authed.Get("/monitors/:id/edit", pageHandler.MonitorEditForm)
	authed.Get("/monitors/:id", pageHandler.MonitorDetail)
	authed.Post("/monitors/:id/delete", pageHandler.MonitorDelete)
	authed.Get("/channels", pageHandler.ChannelList)
	authed.Get("/channels/new", pageHandler.ChannelNewForm)
	authed.Post("/channels", pageHandler.ChannelCreate)
	authed.Get("/channels/:id/edit", pageHandler.ChannelEditForm)
	authed.Post("/channels/:id", pageHandler.ChannelUpdate)
	authed.Post("/channels/:id/delete", pageHandler.ChannelDelete)
	authed.Get("/channels/config-fields", pageHandler.ChannelConfigFields)
	authed.Get("/api-keys", pageHandler.APIKeyList)
	authed.Get("/api-keys/new", pageHandler.APIKeyCreate)
	authed.Post("/api-keys", pageHandler.APIKeyCreateSubmit)
	authed.Post("/api-keys/:id/revoke", pageHandler.APIKeyRevoke)

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
