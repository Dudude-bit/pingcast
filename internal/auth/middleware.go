package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

const UserContextKey = "user"

func (s *Service) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		user, err := s.ValidateSession(c.UserContext(), sessionID)
		if err != nil {
			c.ClearCookie("session_id")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid session"})
		}

		c.Locals(UserContextKey, user)
		return c.Next()
	}
}

func (s *Service) PageMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Redirect("/login")
		}

		user, err := s.ValidateSession(c.UserContext(), sessionID)
		if err != nil {
			c.ClearCookie("session_id")
			return c.Redirect("/login")
		}

		c.Locals(UserContextKey, user)
		return c.Next()
	}
}

func UserFromCtx(c *fiber.Ctx) *gen.GetUserByIDRow {
	user, _ := c.Locals(UserContextKey).(*gen.GetUserByIDRow)
	return user
}
