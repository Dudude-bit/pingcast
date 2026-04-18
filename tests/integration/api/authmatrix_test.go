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

// Spec §2 auth matrix: every protected endpoint returns 401
// UNAUTHORIZED when called without credentials. The list is the
// union of session-auth and bearer-auth routes from §2.
var protectedEndpoints = []struct {
	Method string
	Path   string
}{
	{"GET", "/api/monitors"},
	{"POST", "/api/monitors"},
	{"GET", "/api/monitor-types"},
	{"GET", "/api/monitors/00000000-0000-0000-0000-000000000000"},
	{"PUT", "/api/monitors/00000000-0000-0000-0000-000000000000"},
	{"DELETE", "/api/monitors/00000000-0000-0000-0000-000000000000"},
	{"POST", "/api/monitors/00000000-0000-0000-0000-000000000000/pause"},
	{"GET", "/api/channels"},
	{"POST", "/api/channels"},
	{"GET", "/api/channels/00000000-0000-0000-0000-000000000000"},
	{"PUT", "/api/channels/00000000-0000-0000-0000-000000000000"},
	{"DELETE", "/api/channels/00000000-0000-0000-0000-000000000000"},
	{"POST", "/api/monitors/00000000-0000-0000-0000-000000000000/channels"},
	{"DELETE", "/api/monitors/00000000-0000-0000-0000-000000000000/channels/00000000-0000-0000-0000-000000000000"},
	{"GET", "/api/api-keys"},
	{"POST", "/api/api-keys"},
	{"DELETE", "/api/api-keys/00000000-0000-0000-0000-000000000000"},
	{"POST", "/api/auth/logout"},
}

func TestAuthMatrix_NoAuth_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	for _, c := range protectedEndpoints {
		t.Run(c.Method+" "+c.Path, func(t *testing.T) {
			resp := s.Do(t, c.Method, c.Path, map[string]any{})
			if resp.Status != 401 {
				t.Errorf("status: want 401, got %d (body=%s)", resp.Status, resp.Body)
			}
		})
	}
}

func TestAuthMatrix_BadCookie_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	s.InjectCookie("session_id", "totally-fake-session-id")

	for _, c := range protectedEndpoints {
		t.Run(c.Method+" "+c.Path, func(t *testing.T) {
			resp := s.Do(t, c.Method, c.Path, map[string]any{})
			if resp.Status != 401 {
				t.Errorf("status: want 401, got %d", resp.Status)
			}
		})
	}
}
