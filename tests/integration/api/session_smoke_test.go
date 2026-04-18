//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestSession_RegisterAndLogin_AuthenticatesSubsequentCalls(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "cookie@test.local", "password123")

	resp := s.GET(t, "/api/monitors")
	if resp.Status == 401 {
		t.Fatalf("session did not authenticate; body=%s", resp.Body)
	}
}
