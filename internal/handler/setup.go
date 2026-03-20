package handler

import (
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/auth"
	"github.com/kirillinakin/pingcast/internal/web"
)

func SetupApp(
	authService *auth.Service,
	pageHandler *PageHandler,
	server *Server,
	webhookHandler *WebhookHandler,
) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(recover.New())

	// Static files
	staticFS, _ := fs.Sub(web.FS, "static")
	app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(staticFS),
	}))

	// Health
	app.Get("/health", server.HealthCheck)

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
	// RegisterHandlers wires all routes from the OpenAPI spec.
	apigen.RegisterHandlersWithOptions(app, server, apigen.FiberServerOptions{
		Middlewares: []apigen.MiddlewareFunc{
			authMiddlewareSelector(authService),
		},
	})

	// Authenticated HTML pages
	authed := app.Group("", authService.PageMiddleware())
	authed.Get("/dashboard", pageHandler.Dashboard)
	authed.Get("/monitors/new", pageHandler.MonitorNewForm)
	authed.Post("/monitors", pageHandler.MonitorCreate)
	authed.Get("/monitors/:id/edit", pageHandler.MonitorEditForm)
	authed.Get("/monitors/:id", pageHandler.MonitorDetail)

	return app
}

// authMiddlewareSelector returns a middleware that applies auth only to protected routes.
func authMiddlewareSelector(authService *auth.Service) apigen.MiddlewareFunc {
	publicPaths := map[string]bool{
		"Register":    true,
		"Login":       true,
		"HealthCheck": true,
		"GetStatusPage": true,
	}

	return func(c *fiber.Ctx) error {
		// The operation ID is set by oapi-codegen wrapper via context
		// For simplicity, check if the route needs auth based on path
		path := c.Path()

		// Public API endpoints
		if path == "/api/auth/register" || path == "/api/auth/login" ||
			path == "/health" || len(path) > 12 && path[:12] == "/api/status/" {
			return c.Next()
		}

		// All other /api/ routes need auth
		_ = publicPaths // suppress unused warning
		return authService.Middleware()(c)
	}
}
