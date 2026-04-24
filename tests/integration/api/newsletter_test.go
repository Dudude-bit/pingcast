//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec: newsletter is a global signup (not per-tenant). Double opt-in
// mirrors status-page subscribers: POST subscribe → pending row +
// tokens; GET confirm → flips confirmed_at; GET unsubscribe → deletes.

func fetchNewsletterTokens(t *testing.T, h *harness.Harness, email string) (confirm, unsub string, confirmedAt any) {
	t.Helper()
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT confirm_token, unsubscribe_token, confirmed_at
		 FROM blog_subscribers WHERE email = $1`,
		email,
	).Scan(&confirm, &unsub, &confirmedAt)
	if err != nil {
		t.Fatalf("lookup newsletter row for %s: %v", email, err)
	}
	return
}

func newsletterRowCount(t *testing.T, h *harness.Harness, email string) int {
	t.Helper()
	var n int
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM blog_subscribers WHERE email = $1`, email,
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestNewsletter_Subscribe_Creates202Pending(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().POST(t, "/api/newsletter/subscribe",
		map[string]any{"email": "newsletter@test.local"})
	harness.AssertStatus(t, resp, 202)

	confirm, unsub, confirmedAt := fetchNewsletterTokens(t, h, "newsletter@test.local")
	if len(confirm) < 16 || len(unsub) < 16 {
		t.Errorf("tokens too short: confirm=%q unsub=%q", confirm, unsub)
	}
	if confirmedAt != nil {
		t.Errorf("confirmed_at should be null on fresh signup, got %v", confirmedAt)
	}
}

func TestNewsletter_Subscribe_PreservesSource(t *testing.T) {
	h := harness.New(t)
	source := "footer"
	resp := h.App.NewSession().POST(t, "/api/newsletter/subscribe",
		map[string]any{"email": "src@test.local", "source": source})
	harness.AssertStatus(t, resp, 202)

	var storedSource *string
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT source FROM blog_subscribers WHERE email = $1`, "src@test.local",
	).Scan(&storedSource); err != nil {
		t.Fatalf("lookup source: %v", err)
	}
	if storedSource == nil || *storedSource != "footer" {
		t.Errorf("source: got %v want footer", storedSource)
	}
}

func TestNewsletter_SubscribeDuplicate_Silent202(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	body := map[string]any{"email": "dup@test.local"}

	first := s.POST(t, "/api/newsletter/subscribe", body)
	harness.AssertStatus(t, first, 202)
	second := s.POST(t, "/api/newsletter/subscribe", body)
	harness.AssertStatus(t, second, 202)

	if n := newsletterRowCount(t, h, "dup@test.local"); n != 1 {
		t.Errorf("rows: got %d want 1 after duplicate", n)
	}
}

func TestNewsletter_Confirm_FlipsConfirmedAt(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()

	s.POST(t, "/api/newsletter/subscribe",
		map[string]any{"email": "confirm@test.local"})

	confirm, _, _ := fetchNewsletterTokens(t, h, "confirm@test.local")
	resp := s.GET(t, "/api/newsletter/confirm?token="+confirm)
	harness.AssertStatus(t, resp, 200)

	var confirmedAt any
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT confirmed_at FROM blog_subscribers WHERE email = $1`,
		"confirm@test.local").Scan(&confirmedAt); err != nil {
		t.Fatalf("post-confirm lookup: %v", err)
	}
	if confirmedAt == nil {
		t.Error("confirmed_at should be set")
	}
}

func TestNewsletter_Confirm_InvalidToken_Returns404(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/api/newsletter/confirm?token=bogus-token")
	harness.AssertStatus(t, resp, 404)
}

func TestNewsletter_Unsubscribe_DeletesRow(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()

	s.POST(t, "/api/newsletter/subscribe",
		map[string]any{"email": "bye@test.local"})
	_, unsub, _ := fetchNewsletterTokens(t, h, "bye@test.local")

	resp := s.GET(t, "/api/newsletter/unsubscribe?token="+unsub)
	harness.AssertStatus(t, resp, 200)
	if n := newsletterRowCount(t, h, "bye@test.local"); n != 0 {
		t.Errorf("row still present after unsubscribe: %d", n)
	}
}
