package httpadapter

import (
	"github.com/gofiber/fiber/v2"

	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
)

// SubscribeToStatusPage accepts {email} and fires the double-opt-in
// confirmation email. Responses are intentionally vague (always 202 on
// validation success) so an attacker can't use the endpoint to
// enumerate which emails are already subscribed to which slug.
func (s *Server) SubscribeToStatusPage(c *fiber.Ctx, slug string) error {
	var req apigen.SubscribeToStatusPageJSONRequestBody
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	if err := s.subscriptions.Subscribe(c.UserContext(), slug, string(req.Email)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(httperr.Envelope{
			Error: httperr.Inner{Code: "INVALID_EMAIL", Message: err.Error()},
		})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "check your inbox to confirm",
	})
}

// ConfirmStatusSubscription is the target of the confirmation link in
// the opt-in email. Returns a minimal HTML page because the user
// clicked through from an email client, not from the status-page JS.
func (s *Server) ConfirmStatusSubscription(c *fiber.Ctx, slug string, params apigen.ConfirmStatusSubscriptionParams) error {
	_ = slug
	if _, err := s.subscriptions.Confirm(c.UserContext(), params.Token); err != nil {
		return sendHTML(c, fiber.StatusNotFound, htmlMessage(
			"Link expired",
			"This confirmation link is invalid or already used. Re-subscribe from the status page to try again.",
		))
	}
	return sendHTML(c, fiber.StatusOK, htmlMessage(
		"Subscribed",
		"You're confirmed. You'll get an email the next time an incident is posted on this status page.",
	))
}

// UnsubscribeFromStatusPage is the target of the unsubscribe link in
// every outbound email. Deletes the row — zero friction, no
// "are-you-sure" confirm (CAN-SPAM / 152-ФЗ want the opt-out to be one
// click).
func (s *Server) UnsubscribeFromStatusPage(c *fiber.Ctx, slug string, params apigen.UnsubscribeFromStatusPageParams) error {
	_ = slug
	if _, err := s.subscriptions.Unsubscribe(c.UserContext(), params.Token); err != nil {
		return sendHTML(c, fiber.StatusNotFound, htmlMessage(
			"Already unsubscribed",
			"This unsubscribe link is invalid or already used. You're not on this list.",
		))
	}
	return sendHTML(c, fiber.StatusOK, htmlMessage(
		"Unsubscribed",
		"Done. You won't get any more incident emails from this status page.",
	))
}

func sendHTML(c *fiber.Ctx, status int, body string) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).SendString(body)
}

// htmlMessage wraps a title/body pair in a zero-dependency HTML shell.
// Kept inline so there's no template dir to maintain — three tiny
// pages at most.
func htmlMessage(title, body string) string {
	return `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + title + ` · PingCast</title>
<style>
  body { font: 16px/1.5 system-ui, -apple-system, sans-serif; max-width: 480px;
         margin: 80px auto; padding: 0 20px; color: #0f172a; }
  h1 { font-size: 28px; margin: 0 0 16px; }
  p { color: #475569; }
</style></head>
<body><h1>` + title + `</h1><p>` + body + `</p></body></html>`
}
