//go:build integration

package api

import (
	"context"
	"strings"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Monitor check_config may contain secrets embedded in URLs (e.g.
// health endpoints with a token query string). The production code
// passes a port.Cipher through MonitorRepo; this test verifies that
// the sentinel substring is NOT present in the raw DB column (i.e.
// the value is actually encrypted at rest) yet IS present when
// fetched back via the API (i.e. the read path decrypts correctly).
func TestEncryption_MonitorCheckConfig_RoundTrip(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	const sentinel = "SUPER_SECRET_VALUE_XR4K"
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "encrypted",
		"type":             "http",
		"check_config":     map[string]any{"url": "https://api.example.com/health?token=" + sentinel, "method": "GET"},
		"interval_seconds": 300,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	// Inspect the raw column. If encryption is active, the sentinel
	// must not appear — the ciphertext is a random-looking byte blob.
	var raw []byte
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT check_config FROM monitors WHERE id=$1`, m.ID).Scan(&raw); err != nil {
		t.Fatalf("select raw: %v", err)
	}
	if strings.Contains(string(raw), sentinel) {
		t.Fatalf("sentinel found in raw check_config column — encryption not applied:\n%s",
			string(raw))
	}

	// Read via API — decryption must restore the sentinel.
	get := s.GET(t, "/api/monitors/"+m.ID)
	harness.AssertStatus(t, get, 200)
	if !strings.Contains(string(get.Body), sentinel) {
		t.Fatalf("sentinel missing from API response — decrypt failed:\n%s",
			string(get.Body))
	}
}
