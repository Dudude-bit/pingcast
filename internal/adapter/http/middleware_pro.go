package httpadapter

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// RequirePro is a Fiber middleware that 402s any request from a user
// not on the Pro plan. Must run AFTER AuthMiddleware, which is what
// stores *domain.User under userCtxKey; if no user is present we fall
// back to 401 so the client knows to re-auth rather than retry with
// a payment.
func RequirePro() fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals(userCtxKey).(*domain.User)
		if !ok || user == nil {
			return httperr.WriteUnauthorized(c)
		}
		if domain.RequiresPro(user.Plan) {
			return c.Status(fiber.StatusPaymentRequired).JSON(httperr.Envelope{
				Error: httperr.Inner{
					Code:    "PRO_REQUIRED",
					Message: "this feature requires a Pro subscription",
				},
			})
		}
		return c.Next()
	}
}
