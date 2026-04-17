package httpadapter

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/port"
)

// PageHandler hosts the last remaining non-JSON HTTP handler: the
// HTML Logout form POST. All other HTML pages moved to the Next.js
// frontend (C1-C4). A future cleanup can migrate Logout to the apigen
// Server type and delete this file entirely.
type PageHandler struct {
	auth *app.AuthService
}

func NewPageHandler(auth *app.AuthService, _ *app.MonitoringService, _ *app.AlertService, _ port.RateLimiter, _ port.APIKeyRepo) *PageHandler {
	return &PageHandler{auth: auth}
}

// Logout clears the session cookie and redirects to the landing page.
// Form-POST'd by the Next.js navbar via /api/auth/logout (Fiber routes
// the POST to this handler — the /api/auth/logout path is handled here
// for legacy reasons and because apigen's logout does not write cookies).
func (h *PageHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		if err := h.auth.Logout(c.UserContext(), sessionID); err != nil {
			slog.Warn("logout failed — session will expire via Redis TTL", "error", err)
		}
	}
	c.ClearCookie("session_id")
	return c.Redirect("/")
}
