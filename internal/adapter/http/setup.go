package httpadapter

import (
	"strings"

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
	rls *port.RateLimiters,
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

	// Webhooks (no auth — HMAC-protected by handler; spec §7)
	app.Post("/webhook/lemonsqueezy", webhookHandler.HandleLemonSqueezy)
	app.Post("/webhook/telegram/:token", webhookHandler.HandleTelegramWebhook)

	// SVG status badge (public, embeddable in READMEs / docs sites).
	// Registered outside the apigen router so we own the Content-Type.
	app.Get("/status/:slug/badge.svg", server.GetStatusBadge)

	// JSON API — uses oapi-codegen generated RegisterHandlers.
	// Auth middleware gates every /api/* path except register, login,
	// and the public status page. Rate-limiters run AFTER auth so
	// write/read buckets can key on user.ID. The Pro gate runs last so
	// the user is already resolved in Locals before we check their plan.
	apigen.RegisterHandlersWithOptions(app, server, apigen.FiberServerOptions{
		Middlewares: []apigen.MiddlewareFunc{
			authMiddlewareSelector(authService, apiKeyRepo),
			apiRateLimitSelector(rls),
			proGateSelector(),
		},
	})

	// Catch-all 404 so unmatched routes return the canonical error
	// envelope instead of Fiber's default "Cannot <METHOD> <path>"
	// plaintext. Registered last so it only fires when no earlier
	// handler matches.
	app.Use(func(c *fiber.Ctx) error {
		return httperr.WriteNotFound(c, "route")
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
			path == "/api/stats/public" ||
			path == "/api/billing/founder-status" ||
			(len(path) > 12 && path[:12] == "/api/status/") {
			return c.Next()
		}
		// GET /api/incidents/{id}/updates is public: it powers the
		// timeline on public status pages and is safe by construction
		// (only surfaces what's already on the public page).
		if c.Method() == fiber.MethodGet &&
			strings.HasPrefix(path, "/api/incidents/") &&
			strings.HasSuffix(path, "/updates") {
			return c.Next()
		}

		// All other /api/ routes need auth (session cookie or API key)
		return AuthMiddleware(authService, apiKeyRepo)(c)
	}
}

// proGateSelector enforces RequirePro on the routes exposed to Pro
// subscribers only. The gate reads *domain.User from Locals (populated
// by AuthMiddleware upstream) and 402s free users. Public routes and
// routes outside this match list pass through untouched.
func proGateSelector() apigen.MiddlewareFunc {
	gate := RequirePro()
	return func(c *fiber.Ctx) error {
		path := c.Path()
		method := c.Method()

		// POST /api/incidents — open manual incident
		if method == fiber.MethodPost && path == "/api/incidents" {
			return gate(c)
		}
		// PATCH /api/me/branding — Pro-only status-page customisation
		if method == fiber.MethodPatch && path == "/api/me/branding" {
			return gate(c)
		}
		// Maintenance window scheduling — Pro-only. GET is free-tier
		// readable so users can see their history even after a downgrade.
		if path == "/api/maintenance-windows" && method == fiber.MethodPost {
			return gate(c)
		}
		if strings.HasPrefix(path, "/api/maintenance-windows/") && method == fiber.MethodDelete {
			return gate(c)
		}
		// PATCH /api/incidents/{id}/state — change state + post update
		if method == fiber.MethodPatch &&
			strings.HasPrefix(path, "/api/incidents/") &&
			strings.HasSuffix(path, "/state") {
			return gate(c)
		}
		// POST /api/import/atlassian — Atlassian Statuspage importer
		if method == fiber.MethodPost && path == "/api/import/atlassian" {
			return gate(c)
		}
		return c.Next()
	}
}

// apiRateLimitSelector returns a middleware that applies the right
// scoped rate-limiter based on the request path/method.
//   - /api/status/{slug} (public)      → rls.Status, keyed by IP+slug
//   - Authenticated GET /api/...       → rls.Read,   keyed by user.ID
//   - Authenticated POST/PUT/DELETE    → rls.Write,  keyed by user.ID
//
// Register and Login keep their inline limiters (scopes need IP /
// email keys and a Reset on success, respectively).
func apiRateLimitSelector(rls *port.RateLimiters) apigen.MiddlewareFunc {
	return func(c *fiber.Ctx) error {
		path := c.Path()
		if !strings.HasPrefix(path, "/api/") {
			return c.Next()
		}
		if path == "/api/auth/register" || path == "/api/auth/login" {
			return c.Next() // handled inline
		}
		if strings.HasPrefix(path, "/api/status/") {
			return rateLimitMW(rls.Status, ipSlugKey, 1)(c)
		}
		if path == "/api/stats/public" {
			// Public cached endpoint — IP-keyed read bucket keeps a
			// botnet from hammering it while the in-process 5-min
			// memo absorbs normal load.
			return rateLimitMW(rls.Status, ipSlugKey, 1)(c)
		}

		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead {
			return rateLimitMW(rls.Read, userKey, 1)(c)
		}
		return rateLimitMW(rls.Write, userKey, 1)(c)
	}
}
