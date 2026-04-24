//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §9 (subscribers): POST subscribe → pending row + confirm token;
// GET confirm?token=… → confirmed_at set; GET unsubscribe?token=… →
// row deleted. Subscribe returns 202 on any valid email shape so the
// endpoint can't be used to enumerate who's already subscribed.

// fetchConfirmToken reads the confirm token for a pending row straight
// out of Postgres. Tests are authoritative over their own DB; the real
// flow uses the token sent by email.
func fetchSubscriptionTokens(t *testing.T, h *harness.Harness, slug, email string) (confirm, unsub string, confirmedAt any) {
	t.Helper()
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT confirm_token, unsubscribe_token, confirmed_at
		 FROM status_subscribers
		 WHERE slug = $1 AND email = $2`,
		slug, email,
	).Scan(&confirm, &unsub, &confirmedAt)
	if err != nil {
		t.Fatalf("lookup subscription for %s/%s: %v", slug, email, err)
	}
	return
}

func subscriptionRowCount(t *testing.T, h *harness.Harness, slug, email string) int {
	t.Helper()
	var n int
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM status_subscribers WHERE slug = $1 AND email = $2`,
		slug, email,
	).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestSubscribe_ValidEmail_Creates202PendingRow(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)
	// status-subscribers table keys off slug, which exists for any user.

	resp := h.App.NewSession().POST(t,
		"/api/status/"+user.Slug+"/subscribe",
		map[string]any{"email": "sub@test.local"},
	)
	harness.AssertStatus(t, resp, 202)

	confirm, unsub, confirmedAt := fetchSubscriptionTokens(t, h, user.Slug, "sub@test.local")
	if len(confirm) < 16 {
		t.Errorf("confirm token too short: %q", confirm)
	}
	if len(unsub) < 16 {
		t.Errorf("unsub token too short: %q", unsub)
	}
	if confirmedAt != nil {
		t.Errorf("expected pending row (confirmed_at IS NULL), got %v", confirmedAt)
	}
}

func TestSubscribe_InvalidEmail_Returns400(t *testing.T) {
	// The openapi binding validates email format before the handler
	// runs — a clearly malformed value like "not-an-email" fails to
	// parse at that layer and surfaces as MALFORMED_JSON. Either way,
	// no row is written.
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	resp := h.App.NewSession().POST(t,
		"/api/status/"+user.Slug+"/subscribe",
		map[string]any{"email": "not-an-email"},
	)
	if resp.Status != 400 {
		t.Fatalf("status: got %d want 400, body=%s", resp.Status, resp.Body)
	}

	if n := subscriptionRowCount(t, h, user.Slug, "not-an-email"); n != 0 {
		t.Errorf("rows: got %d want 0", n)
	}
}

func TestSubscribe_DuplicateEmail_Silent202NoSecondRow(t *testing.T) {
	// Spec: duplicate (slug,email) returns ok silently to avoid leaking
	// which addresses are already subscribed.
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	anon := h.App.NewSession()
	body := map[string]any{"email": "dup@test.local"}
	first := anon.POST(t, "/api/status/"+user.Slug+"/subscribe", body)
	harness.AssertStatus(t, first, 202)

	second := anon.POST(t, "/api/status/"+user.Slug+"/subscribe", body)
	harness.AssertStatus(t, second, 202)

	if n := subscriptionRowCount(t, h, user.Slug, "dup@test.local"); n != 1 {
		t.Errorf("rows after duplicate: got %d want 1", n)
	}
}

func TestConfirm_ValidToken_FlipsConfirmedAt(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)
	anon := h.App.NewSession()

	resp := anon.POST(t, "/api/status/"+user.Slug+"/subscribe",
		map[string]any{"email": "confirm@test.local"})
	harness.AssertStatus(t, resp, 202)

	confirm, _, _ := fetchSubscriptionTokens(t, h, user.Slug, "confirm@test.local")

	cr := anon.GET(t, "/api/status/"+user.Slug+"/confirm?token="+confirm)
	harness.AssertStatus(t, cr, 200)

	var confirmedAt any
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT confirmed_at FROM status_subscribers WHERE slug = $1 AND email = $2`,
		user.Slug, "confirm@test.local").Scan(&confirmedAt); err != nil {
		t.Fatalf("post-confirm lookup: %v", err)
	}
	if confirmedAt == nil {
		t.Error("confirmed_at should be set after /confirm")
	}
}

func TestConfirm_InvalidToken_Returns404(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	resp := h.App.NewSession().GET(t,
		"/api/status/"+user.Slug+"/confirm?token=obviously-bogus-token")
	harness.AssertStatus(t, resp, 404)
}

func TestUnsubscribe_ValidToken_DeletesRow(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)
	anon := h.App.NewSession()

	anon.POST(t, "/api/status/"+user.Slug+"/subscribe",
		map[string]any{"email": "bye@test.local"})
	_, unsub, _ := fetchSubscriptionTokens(t, h, user.Slug, "bye@test.local")

	resp := anon.GET(t, "/api/status/"+user.Slug+"/unsubscribe?token="+unsub)
	harness.AssertStatus(t, resp, 200)

	if n := subscriptionRowCount(t, h, user.Slug, "bye@test.local"); n != 0 {
		t.Errorf("row still present after unsubscribe: %d", n)
	}
}

func TestListMySubscribers_OnlyConfirmedShown(t *testing.T) {
	h := harness.New(t)
	owner, user := h.RegisterAndLoginUser(t)
	anon := h.App.NewSession()

	// Two subscribers: one confirmed, one still pending.
	anon.POST(t, "/api/status/"+user.Slug+"/subscribe",
		map[string]any{"email": "a@test.local"})
	anon.POST(t, "/api/status/"+user.Slug+"/subscribe",
		map[string]any{"email": "b@test.local"})

	confirmA, _, _ := fetchSubscriptionTokens(t, h, user.Slug, "a@test.local")
	anon.GET(t, "/api/status/"+user.Slug+"/confirm?token="+confirmA)

	resp := owner.GET(t, "/api/me/subscribers")
	harness.AssertStatus(t, resp, 200)

	var subs []struct {
		Email string `json:"email"`
	}
	resp.JSON(t, &subs)

	if len(subs) != 1 {
		t.Fatalf("listed: got %d want 1 confirmed", len(subs))
	}
	if subs[0].Email != "a@test.local" {
		t.Errorf("email: got %q", subs[0].Email)
	}
}
