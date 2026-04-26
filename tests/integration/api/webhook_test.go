//go:build integration

package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// TestLemonSqueezy_SubscriptionCreated_FounderVariant_UpgradesAndTags
// is the safety net for "customer pays $9, doesn't get Pro." When the
// webhook arrives with subscription_created + status=active + a
// variant_id matching the founder env, we expect:
//   - users.plan flips to 'pro'
//   - users.subscription_variant = 'founder' (so founder-cap counts it)
//   - users.lemonsqueezy_subscription_id is recorded
func TestLemonSqueezy_SubscriptionCreated_FounderVariant_UpgradesAndTags(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	body := []byte(fmt.Sprintf(
		`{"meta":{"event_name":"subscription_created"},"data":{"id":"sub-1","attributes":{"customer_id":42,"status":"active","user_email":%q,"variant_id":%s}}}`,
		user.Email, harness.TestFounderVariantID,
	))

	s := h.App.NewSession()
	s.SetHeader("X-Signature", signLS(body))
	resp := s.DoRaw(t, "POST", "/webhook/lemonsqueezy", "application/json", body)
	if resp.Status != 200 {
		t.Fatalf("want 200, got %d body=%s", resp.Status, resp.Body)
	}

	var plan, variant string
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT plan, COALESCE(subscription_variant,'') FROM users WHERE id = $1`,
		user.ID,
	).Scan(&plan, &variant)
	if err != nil {
		t.Fatalf("read user row: %v", err)
	}
	if plan != "pro" {
		t.Errorf("plan: want 'pro', got %q — paying customer didn't get Pro", plan)
	}
	if variant != "founder" {
		t.Errorf("variant: want 'founder', got %q — founder-cap will undercount", variant)
	}
}

// TestLemonSqueezy_SubscriptionCreated_RetailVariant_TagsRetail —
// non-founder variant_id must record as 'retail' so the founder cap
// isn't accidentally inflated by retail subscribers.
func TestLemonSqueezy_SubscriptionCreated_RetailVariant_TagsRetail(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	// Some other variant — guaranteed not to match the founder ID.
	body := []byte(fmt.Sprintf(
		`{"meta":{"event_name":"subscription_created"},"data":{"id":"sub-2","attributes":{"customer_id":43,"status":"active","user_email":%q,"variant_id":99999}}}`,
		user.Email,
	))

	s := h.App.NewSession()
	s.SetHeader("X-Signature", signLS(body))
	resp := s.DoRaw(t, "POST", "/webhook/lemonsqueezy", "application/json", body)
	if resp.Status != 200 {
		t.Fatalf("want 200, got %d body=%s", resp.Status, resp.Body)
	}

	var plan, variant string
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT plan, COALESCE(subscription_variant,'') FROM users WHERE id = $1`,
		user.ID,
	).Scan(&plan, &variant)
	if err != nil {
		t.Fatalf("read user row: %v", err)
	}
	if plan != "pro" {
		t.Errorf("plan: want 'pro', got %q", plan)
	}
	if variant != "retail" {
		t.Errorf("variant: want 'retail', got %q", variant)
	}
}

// TestLemonSqueezy_FounderCap_RetailOverflow — when the founder cap
// is full, a new founder-variant subscription must be tagged 'retail'
// instead. Without atomic check-and-set, two concurrent webhooks at
// used=cap-1 can both pass the check and both write 'founder',
// breaking the contractual scarcity promise.
func TestLemonSqueezy_FounderCap_RetailOverflow(t *testing.T) {
	h := harness.New(t)

	// Saturate the cap (TestFounderCap=2) by seeding two founders.
	for i := 0; i < harness.TestFounderCap; i++ {
		_, u := h.RegisterAndLoginUser(t)
		h.SetSubscriptionVariant(t, u.ID, "founder")
	}

	// Now a NEW user pays the founder variant. With the cap full, the
	// webhook handler must atomically downgrade the tag to 'retail'.
	_, newUser := h.RegisterAndLoginUser(t)
	body := []byte(fmt.Sprintf(
		`{"meta":{"event_name":"subscription_created"},"data":{"id":"sub-overflow","attributes":{"customer_id":777,"status":"active","user_email":%q,"variant_id":%s}}}`,
		newUser.Email, harness.TestFounderVariantID,
	))
	s := h.App.NewSession()
	s.SetHeader("X-Signature", signLS(body))
	resp := s.DoRaw(t, "POST", "/webhook/lemonsqueezy", "application/json", body)
	if resp.Status != 200 {
		t.Fatalf("want 200, got %d body=%s", resp.Status, resp.Body)
	}

	var variant string
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COALESCE(subscription_variant,'') FROM users WHERE id = $1`,
		newUser.ID,
	).Scan(&variant)
	if err != nil {
		t.Fatalf("read user: %v", err)
	}
	if variant != "retail" {
		t.Errorf("variant: got %q, want 'retail' (cap saturated, must downgrade)", variant)
	}

	// Sanity: founder count must not exceed the cap.
	var founderCount int
	err = h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM users WHERE plan='pro' AND subscription_variant='founder'`,
	).Scan(&founderCount)
	if err != nil {
		t.Fatalf("count founders: %v", err)
	}
	if founderCount > harness.TestFounderCap {
		t.Errorf("founder count %d exceeds cap %d", founderCount, harness.TestFounderCap)
	}
}

func TestTelegramWebhook_ValidToken_PingMessageReturns200(t *testing.T) {
	// Sanity: a bare hello message with the right token is accepted as
	// a no-op (no channel created, but the webhook returns 200 so
	// Telegram doesn't retry).
	h := harness.New(t)
	body := []byte(`{"message":{"chat":{"id":42},"text":"hello"}}`)

	s := h.App.NewSession()
	resp := s.DoRaw(t, "POST", "/webhook/telegram/"+harness.TestTelegramBotToken,
		"application/json", body)
	if resp.Status != 200 {
		t.Fatalf("want 200, got %d body=%s", resp.Status, resp.Body)
	}
}

// TestTelegramWebhook_WrongTokenInPath_DoesNotCreateChannel — IDOR
// regression guard. Without path-token validation, anyone hitting the
// public webhook URL with /start <victim-uuid> could create a Telegram
// notification channel for the victim's account, redirecting their
// downtime alerts to the attacker's chat. The handler must reject
// requests whose path token doesn't match the configured bot secret.
func TestTelegramWebhook_WrongTokenInPath_DoesNotCreateChannel(t *testing.T) {
	h := harness.New(t)
	_, victim := h.RegisterAndLoginUser(t)

	body := []byte(`{"message":{"chat":{"id":99999},"text":"/start ` + victim.ID.String() + `"}}`)
	s := h.App.NewSession()
	resp := s.DoRaw(t, "POST", "/webhook/telegram/wrong-token-attacker",
		"application/json", body)
	// Returning 200 silently is fine (don't leak token-presence info)
	// — what matters is that no row gets written.
	if resp.Status != 200 && resp.Status != 401 && resp.Status != 404 {
		t.Fatalf("unexpected status %d, body=%s", resp.Status, resp.Body)
	}

	var n int
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM notification_channels WHERE user_id = $1 AND type = 'telegram'`,
		victim.ID,
	).Scan(&n)
	if err != nil {
		t.Fatalf("count channels: %v", err)
	}
	if n != 0 {
		t.Errorf("attacker created %d channel(s) on victim's account — IDOR is open", n)
	}
}

// TestTelegramWebhook_CorrectTokenAndStartCommand_CreatesChannel —
// happy path: with the right path token and a /start <userID>
// payload, the handler creates a Telegram channel for that user.
// This is the legitimate flow that the IDOR fix must NOT break.
func TestTelegramWebhook_CorrectTokenAndStartCommand_CreatesChannel(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	body := []byte(`{"message":{"chat":{"id":12345},"text":"/start ` + user.ID.String() + `"}}`)
	s := h.App.NewSession()
	resp := s.DoRaw(t, "POST", "/webhook/telegram/"+harness.TestTelegramBotToken,
		"application/json", body)
	if resp.Status != 200 {
		t.Fatalf("want 200, got %d body=%s", resp.Status, resp.Body)
	}

	var n int
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM notification_channels WHERE user_id = $1 AND type = 'telegram'`,
		user.ID,
	).Scan(&n)
	if err != nil {
		t.Fatalf("count channels: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 telegram channel after legit /start, got %d", n)
	}
}
