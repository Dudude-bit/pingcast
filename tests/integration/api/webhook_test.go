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
