//go:build integration

package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §4 + §7 (Webhooks): HMAC for LemonSqueezy + a liveness check
// on the Telegram inbound webhook path.

const lsSecret = "test-ls-secret" // mirrored in harness/app.go

func signLS(body []byte) string {
	mac := hmac.New(sha256.New, []byte(lsSecret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestLemonSqueezy_BadSignature_Returns401(t *testing.T) {
	h := harness.New(t)
	body := []byte(`{"meta":{"event_name":"subscription_created"},"data":{}}`)

	s := h.App.NewSession()
	s.SetHeader("X-Signature", "not-a-valid-hex-signature")
	resp := s.DoRaw(t, "POST", "/webhook/lemonsqueezy", "application/json", body)
	if resp.Status != 401 {
		t.Fatalf("want 401, got %d body=%s", resp.Status, resp.Body)
	}
}

func TestLemonSqueezy_ValidSignature_Returns200(t *testing.T) {
	h := harness.New(t)
	body := []byte(`{"meta":{"event_name":"subscription_created"},"data":{"id":"e1","attributes":{"customer_id":1,"status":"active","user_email":"unknown@test.local"}}}`)

	s := h.App.NewSession()
	s.SetHeader("X-Signature", signLS(body))
	resp := s.DoRaw(t, "POST", "/webhook/lemonsqueezy", "application/json", body)
	if resp.Status != 200 {
		t.Fatalf("want 200, got %d body=%s", resp.Status, resp.Body)
	}
}

func TestLemonSqueezy_BadJSON_Returns400(t *testing.T) {
	h := harness.New(t)
	body := []byte("{broken")

	s := h.App.NewSession()
	s.SetHeader("X-Signature", signLS(body))
	resp := s.DoRaw(t, "POST", "/webhook/lemonsqueezy", "application/json", body)
	if resp.Status != 400 {
		t.Fatalf("want 400, got %d body=%s", resp.Status, resp.Body)
	}
}

func TestTelegramWebhook_ValidBody_Returns200(t *testing.T) {
	// Current implementation accepts any :token value; spec §7 will
	// eventually gate this on a per-channel secret. Test locks in the
	// happy-path route shape for now.
	h := harness.New(t)
	body := []byte(`{"message":{"chat":{"id":42},"text":"hello"}}`)

	s := h.App.NewSession()
	resp := s.DoRaw(t, "POST", "/webhook/telegram/any-token-here", "application/json", body)
	if resp.Status != 200 {
		t.Fatalf("want 200, got %d body=%s", resp.Status, resp.Body)
	}
}
