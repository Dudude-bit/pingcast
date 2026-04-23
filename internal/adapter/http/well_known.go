package httpadapter

import (
	"github.com/gofiber/fiber/v2"
)

// GetWellKnownProbe serves the domain-validation token requested by
// the custom-domain validation worker. Path:
// /.well-known/pingcast/<token>. Public, no auth — anyone can see a
// token if they know the URL, and the token only proves that a hostname
// currently resolves to us (the hostname the validator is probing).
//
// Implementation: return the token as the response body. The worker
// reads the body and compares to its stored value.
func (s *Server) GetWellKnownProbe(c *fiber.Ctx) error {
	token := c.Params("token")
	if token == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendString(token)
}
