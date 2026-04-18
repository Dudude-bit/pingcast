//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §8.6: the top-level /logout page handler is legacy SSR cruft
// removed after the Next.js migration. The test asserts the route no
// longer exists so nobody accidentally reintroduces it.

func TestPageLogout_RouteRemoved(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/logout", nil)
	if resp.Status != 404 {
		t.Fatalf("want 404 for removed /logout, got %d body=%s", resp.Status, resp.Body)
	}
}
