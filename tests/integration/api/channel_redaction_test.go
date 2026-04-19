//go:build integration

package api

import (
	"strings"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §8.9: channel secrets must be masked on every read path. Value
// is "***" + last-4 chars of the original — enough for the owner to
// recognise it, opaque for anyone who intercepts the response.

func TestChannelRedaction_Telegram_BotTokenMasked(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	const secret = "12345:SUPERSECRETTOKEN"
	cr := s.POST(t, "/api/channels", map[string]any{
		"name":   "tg",
		"type":   "telegram",
		"config": map[string]any{"bot_token": secret, "chat_id": 99},
	})
	harness.AssertStatus(t, cr, 201)
	var ch struct {
		ID     string         `json:"id"`
		Config map[string]any `json:"config"`
	}
	cr.JSON(t, &ch)

	if tok, _ := ch.Config["bot_token"].(string); tok == secret {
		t.Fatalf("bot_token leaked unredacted: %q", tok)
	}

	// Re-read via GET by id — still redacted.
	got := s.GET(t, "/api/channels/"+ch.ID)
	if strings.Contains(string(got.Body), secret) {
		t.Fatalf("bot_token leaked on GET response:\n%s", got.Body)
	}

	// Redacted value preserves last-4 for recognisability.
	var after struct {
		Config map[string]any `json:"config"`
	}
	got.JSON(t, &after)
	tok, _ := after.Config["bot_token"].(string)
	if !strings.HasPrefix(tok, "***") || !strings.HasSuffix(tok, secret[len(secret)-4:]) {
		t.Errorf("expected *** + last-4 mask, got %q", tok)
	}
}

func TestChannelRedaction_Webhook_URLMasked(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	// Must pass the webhook URL validator (no localhost), so use a
	// publicly-shaped URL. The secret lives in the query-string suffix.
	const url = "https://example.com/hook?token=SECRET_ZZZZ"
	cr := s.POST(t, "/api/channels", map[string]any{
		"name":   "wh",
		"type":   "webhook",
		"config": map[string]any{"url": url},
	})
	harness.AssertStatus(t, cr, 201)
	var ch struct {
		Config map[string]any `json:"config"`
	}
	cr.JSON(t, &ch)

	got, _ := ch.Config["url"].(string)
	if got == url {
		t.Fatalf("url leaked unredacted: %q", got)
	}
	if !strings.HasPrefix(got, "***") {
		t.Errorf("expected *** prefix, got %q", got)
	}
}
