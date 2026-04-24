//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §8 (custom domains): Pro-only. POST /api/custom-domains creates
// a pending row with a validation token; lookup-domain is public and
// returns the slug once the row is flipped to active + cache refreshed.

func TestCustomDomains_RequestAsPro_CreatesPendingRow(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	resp := s.POST(t, "/api/custom-domains",
		map[string]any{"hostname": "status.acme.test"})
	harness.AssertStatus(t, resp, 201)

	var body struct {
		ID              int64  `json:"id"`
		Hostname        string `json:"hostname"`
		ValidationToken string `json:"validation_token"`
		Status          string `json:"status"`
	}
	resp.JSON(t, &body)
	if body.Hostname != "status.acme.test" {
		t.Errorf("hostname: got %q", body.Hostname)
	}
	if len(body.ValidationToken) < 16 {
		t.Errorf("validation_token too short: %q", body.ValidationToken)
	}
	if body.Status != "pending" {
		t.Errorf("status: got %q want pending", body.Status)
	}
}

func TestCustomDomains_FreeUser_Returns402(t *testing.T) {
	h := harness.New(t)
	s, _ := h.RegisterAndLoginUser(t)

	resp := s.POST(t, "/api/custom-domains",
		map[string]any{"hostname": "status.acme.test"})
	harness.AssertError(t, resp, 402, "PRO_REQUIRED")
}

func TestCustomDomains_ReservedHostname_Returns422(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	resp := s.POST(t, "/api/custom-domains",
		map[string]any{"hostname": "pingcast.io"})
	harness.AssertError(t, resp, 422, "INVALID_HOSTNAME")
}

func TestCustomDomains_List_ReturnsOwnRowsOnly(t *testing.T) {
	h := harness.New(t)
	s1, u1 := h.RegisterAndLoginUser(t)
	s2, u2 := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, u1.ID)
	h.PromoteToPro(t, u2.ID)

	s1.POST(t, "/api/custom-domains", map[string]any{"hostname": "status.one.test"})
	s2.POST(t, "/api/custom-domains", map[string]any{"hostname": "status.two.test"})

	resp := s1.GET(t, "/api/custom-domains")
	harness.AssertStatus(t, resp, 200)
	var rows []struct {
		Hostname string `json:"hostname"`
	}
	resp.JSON(t, &rows)
	if len(rows) != 1 {
		t.Fatalf("rows: got %d want 1", len(rows))
	}
	if rows[0].Hostname != "status.one.test" {
		t.Errorf("hostname: got %q", rows[0].Hostname)
	}
}

func TestCustomDomains_Delete_RemovesRow(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	resp := s.POST(t, "/api/custom-domains",
		map[string]any{"hostname": "status.zap.test"})
	harness.AssertStatus(t, resp, 201)
	var body struct {
		ID int64 `json:"id"`
	}
	resp.JSON(t, &body)

	del := s.Do(t, "DELETE", "/api/custom-domains/"+itoa(body.ID), nil)
	harness.AssertStatus(t, del, 204)

	var n int
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM custom_domains WHERE id = $1`, body.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("row still present: %d", n)
	}
}

// LookupCustomDomain: seed an active row directly, refresh cache, then
// the public lookup endpoint returns the slug. Skipping the full
// validation probe because that hits live HTTPS on the customer's host.
func TestLookupCustomDomain_ActiveHostname_ReturnsSlug(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	// Insert an active domain directly; the test is about the lookup
	// cache, not the state machine.
	_, err := h.App.Pool.Exec(context.Background(),
		`INSERT INTO custom_domains (user_id, hostname, validation_token, status, cert_issued_at)
		 VALUES ($1, $2, $3, 'active', NOW())`,
		user.ID, "status.live.test", "fixed-token-123")
	if err != nil {
		t.Fatalf("seed active domain: %v", err)
	}
	h.App.CustomDomainSvc.PreloadHostnameCache(context.Background())

	anon := h.App.NewSession()
	resp := anon.GET(t, "/api/public/lookup-domain?hostname=status.live.test")
	harness.AssertStatus(t, resp, 200)

	var body struct {
		Slug string `json:"slug"`
	}
	resp.JSON(t, &body)
	if body.Slug != user.Slug {
		t.Errorf("slug: got %q want %q", body.Slug, user.Slug)
	}
	if cc := resp.Headers.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("cache-control: got %q", cc)
	}
}

func TestLookupCustomDomain_UnknownHostname_Returns404(t *testing.T) {
	h := harness.New(t)

	resp := h.App.NewSession().GET(t,
		"/api/public/lookup-domain?hostname=nothing-here.test")
	harness.AssertStatus(t, resp, 404)
}

// itoa is the tiny helper the stdlib-free side of this test file needs;
// strconv.Itoa would be fine too but keeping the file import-free for
// small integer formatting.
func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
