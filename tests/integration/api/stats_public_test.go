//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec: /api/stats/public is an unauthenticated, 5-min-cached counter
// feed for the landing-page trust bar.
func TestPublicStats_Unauthenticated_Returns200WithCacheHeader(t *testing.T) {
	h := harness.New(t)

	resp := h.App.NewSession().GET(t, "/api/stats/public")
	harness.AssertStatus(t, resp, 200)

	if cc := resp.Headers.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Fatalf("Cache-Control: want=%q got=%q", "public, max-age=300", cc)
	}

	var body struct {
		MonitorsCount     int64 `json:"monitors_count"`
		IncidentsResolved int64 `json:"incidents_resolved"`
		PublicStatusPages int64 `json:"public_status_pages"`
	}
	resp.JSON(t, &body)

	// Empty DB → all zero. Assertions only check sanity: non-negative.
	if body.MonitorsCount < 0 || body.IncidentsResolved < 0 || body.PublicStatusPages < 0 {
		t.Fatalf("nonsense counts: %+v", body)
	}
}
