package httpadapter

import (
	"github.com/gofiber/fiber/v2"

	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
)

// SubscribeToNewsletter accepts {email, source?} and fires the
// double-opt-in confirmation email. Response is always 202 on valid
// email shape (so the endpoint can't be used to enumerate addresses
// already subscribed).
func (s *Server) SubscribeToNewsletter(c *fiber.Ctx) error {
	var req apigen.SubscribeToNewsletterJSONRequestBody
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	locale := ""
	if req.Locale != nil {
		locale = *req.Locale
	}
	if err := s.blogSubscriptions.Subscribe(c.UserContext(), string(req.Email), req.Source, locale); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(httperr.Envelope{
			Error: httperr.Inner{Code: "INVALID_EMAIL", Message: err.Error()},
		})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "check your inbox to confirm",
	})
}

// ConfirmNewsletterSubscription is the target of the confirmation link.
// Returns a minimal HTML page — user clicked from an email client, not
// from the site's JS.
func (s *Server) ConfirmNewsletterSubscription(c *fiber.Ctx, params apigen.ConfirmNewsletterSubscriptionParams) error {
	if _, err := s.blogSubscriptions.Confirm(c.UserContext(), params.Token); err != nil {
		return sendHTML(c, fiber.StatusNotFound, htmlMessage(
			"Link expired",
			"This confirmation link is invalid or already used. Re-subscribe from the site to try again.",
		))
	}
	return sendHTML(c, fiber.StatusOK, htmlMessage(
		"Subscribed",
		"You're confirmed. Next PingCast newsletter email will land in your inbox — 1-2 per month, unsubscribe in one click anytime.",
	))
}

// UnsubscribeFromNewsletter removes the row. Zero friction, no
// "are-you-sure" (CAN-SPAM wants one-click opt-out).
func (s *Server) UnsubscribeFromNewsletter(c *fiber.Ctx, params apigen.UnsubscribeFromNewsletterParams) error {
	if _, err := s.blogSubscriptions.Unsubscribe(c.UserContext(), params.Token); err != nil {
		return sendHTML(c, fiber.StatusNotFound, htmlMessage(
			"Already unsubscribed",
			"This unsubscribe link is invalid or already used. You're not on this list.",
		))
	}
	return sendHTML(c, fiber.StatusOK, htmlMessage(
		"Unsubscribed",
		"Done. You won't get any more PingCast newsletter emails.",
	))
}
