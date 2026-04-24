//go:build integration

package api

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §6 (distribution surfaces): badge + feed are public; CSV export
// is Pro-gated. All three read from the same status-page aggregate so
// a single configured slug drives them all.

func TestStatusBadge_FreeTier_IncludesViaPingCast(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	resp := h.App.NewSession().GET(t, "/status/"+user.Slug+"/badge.svg")
	harness.AssertStatus(t, resp, 200)
	if ct := resp.Headers.Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("content-type: got %q want image/svg+xml", ct)
	}
	body := string(resp.Body)
	if !strings.Contains(body, "<svg") {
		t.Errorf("not an SVG: %s", body[:min(200, len(body))])
	}
	// Free tier must include the credit link.
	if !strings.Contains(body, "via PingCast") {
		t.Error("free-tier badge should embed 'via PingCast' credit")
	}
}

func TestStatusBadge_ProTier_NoCredit(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	resp := h.App.NewSession().GET(t, "/status/"+user.Slug+"/badge.svg")
	harness.AssertStatus(t, resp, 200)
	body := string(resp.Body)
	if strings.Contains(body, "via PingCast") {
		t.Error("Pro badge must not embed 'via PingCast' credit")
	}
}

func TestStatusBadge_UnknownSlug_Returns404(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/status/nonexistent-slug/badge.svg")
	harness.AssertStatus(t, resp, 404)
}

func TestStatusFeed_ReturnsRSS20_WithChannel(t *testing.T) {
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	resp := h.App.NewSession().GET(t, "/status/"+user.Slug+"/feed.xml")
	harness.AssertStatus(t, resp, 200)

	ct := resp.Headers.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/rss+xml") {
		t.Errorf("content-type: got %q want application/rss+xml*", ct)
	}
	body := string(resp.Body)
	if !strings.Contains(body, `<?xml`) {
		t.Error("missing XML declaration")
	}
	if !strings.Contains(body, `<rss version="2.0"`) {
		t.Error("missing <rss version='2.0'> root")
	}
	if !strings.Contains(body, "<channel>") || !strings.Contains(body, "</channel>") {
		t.Error("missing <channel> element")
	}
	if !strings.Contains(body, user.Slug) {
		t.Errorf("slug %q not referenced in feed", user.Slug)
	}
}

func TestStatusFeed_RespectsXForwardedProto(t *testing.T) {
	// Behind our reverse proxy the inner app sees HTTP; X-Forwarded-Proto
	// is how the real scheme arrives. The feed must emit https:// links
	// when the header says so.
	h := harness.New(t)
	_, user := h.RegisterAndLoginUser(t)

	s := h.App.NewSession()
	s.SetHeader("X-Forwarded-Proto", "https")
	resp := s.GET(t, "/status/"+user.Slug+"/feed.xml")
	harness.AssertStatus(t, resp, 200)

	body := string(resp.Body)
	if !strings.Contains(body, "https://") {
		t.Errorf("feed links should use https:// when X-Forwarded-Proto=https; body=%s",
			body[:min(500, len(body))])
	}
}

func TestExportCSV_ProUser_StreamsHeaders(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	resp := s.GET(t, "/api/incidents/export.csv")
	harness.AssertStatus(t, resp, 200)
	ct := resp.Headers.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("content-type: got %q", ct)
	}
	cd := resp.Headers.Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment; filename=") {
		t.Errorf("content-disposition: got %q", cd)
	}

	// Body is at least the header row.
	r := csv.NewReader(strings.NewReader(string(resp.Body)))
	header, err := r.Read()
	if err != nil {
		t.Fatalf("read csv header: %v", err)
	}
	wantCols := []string{"id", "monitor_id", "monitor_name", "title", "state",
		"is_manual", "started_at", "resolved_at", "cause"}
	if len(header) != len(wantCols) {
		t.Fatalf("header cols: got %d want %d (%v)", len(header), len(wantCols), header)
	}
	for i, want := range wantCols {
		if header[i] != want {
			t.Errorf("col %d: got %q want %q", i, header[i], want)
		}
	}
}

func TestExportCSV_FreeUser_Returns402(t *testing.T) {
	h := harness.New(t)
	s, _ := h.RegisterAndLoginUser(t)

	resp := s.GET(t, "/api/incidents/export.csv")
	harness.AssertError(t, resp, 402, "PRO_REQUIRED")
}

func TestExportCSV_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/api/incidents/export.csv")
	if resp.Status != 401 {
		t.Fatalf("status: got %d want 401, body=%s", resp.Status, resp.Body)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
