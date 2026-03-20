package httpadapter

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
)

const userCtxKey = "auth.user"

// AuthMiddleware returns a Fiber handler that validates the session cookie
// and stores *domain.User in Locals. Returns 401 JSON on failure.
func AuthMiddleware(auth *app.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		user, err := auth.ValidateSession(c.UserContext(), sessionID)
		if err != nil {
			c.ClearCookie("session_id")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid session"})
		}

		c.Locals(userCtxKey, user)
		return c.Next()
	}
}

// PageMiddleware returns a Fiber handler that validates the session cookie
// and stores *domain.User in Locals. Redirects to /login on failure.
func PageMiddleware(auth *app.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Redirect("/login")
		}

		user, err := auth.ValidateSession(c.UserContext(), sessionID)
		if err != nil {
			c.ClearCookie("session_id")
			return c.Redirect("/login")
		}

		c.Locals(userCtxKey, user)
		return c.Next()
	}
}

// UserFromCtx extracts the authenticated *domain.User from fiber.Locals.
func UserFromCtx(c *fiber.Ctx) *domain.User {
	user, _ := c.Locals(userCtxKey).(*domain.User)
	return user
}
