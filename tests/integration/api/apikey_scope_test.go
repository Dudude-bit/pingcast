//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §2: API-key scope boundaries. A key with monitors:read must be
// able to GET /api/monitors but must be blocked from POST. middleware.go
// already enforces this; these tests lock it in against regressions.

func TestAPIKeyScope_ReadKey_Can_GET_Monitors(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	cr := s.POST(t, "/api/api-keys", map[string]any{
		"name":   "read-only",
		"scopes": []string{"monitors:read"},
	})
	harness.AssertStatus(t, cr, 201)
	var key struct {
		RawKey string `json:"raw_key"`
	}
	cr.JSON(t, &key)

	bearer := h.App.WithBearer(key.RawKey)
	resp := bearer.GET(t, "/api/monitors")
	harness.AssertStatus(t, resp, 200)
}

func TestAPIKeyScope_ReadKey_Cannot_POST_Monitors(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	cr := s.POST(t, "/api/api-keys", map[string]any{
		"name":   "read-only",
		"scopes": []string{"monitors:read"},
	})
	harness.AssertStatus(t, cr, 201)
	var key struct {
		RawKey string `json:"raw_key"`
	}
	cr.JSON(t, &key)

	bearer := h.App.WithBearer(key.RawKey)
	resp := bearer.POST(t, "/api/monitors", map[string]any{
		"name":             "blocked",
		"type":             "http",
		"check_config":     map[string]any{"url": "https://example.com", "method": "GET"},
		"interval_seconds": 300,
	})
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

func TestAPIKeyScope_WriteKey_Can_POST_Monitors(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	cr := s.POST(t, "/api/api-keys", map[string]any{
		"name":   "writer",
		"scopes": []string{"monitors:read", "monitors:write"},
	})
	harness.AssertStatus(t, cr, 201)
	var key struct {
		RawKey string `json:"raw_key"`
	}
	cr.JSON(t, &key)

	bearer := h.App.WithBearer(key.RawKey)
	resp := bearer.POST(t, "/api/monitors", map[string]any{
		"name":             "via-key",
		"type":             "http",
		"check_config":     map[string]any{"url": "https://example.com", "method": "GET"},
		"interval_seconds": 300,
	})
	harness.AssertStatus(t, resp, 201)
}
