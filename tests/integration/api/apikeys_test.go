//go:build integration

package api

import (
	"strings"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §4 (API Keys): create/list/revoke + bearer auth + isolation.

func TestCreateAPIKey_Happy_ReturnsTokenOnce(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/api-keys", map[string]any{
		"name":   "ci-bot",
		"scopes": []string{"monitors:read"},
	})
	harness.AssertStatus(t, resp, 201)

	var body struct {
		RawKey string `json:"raw_key"`
		Key    struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"key"`
	}
	resp.JSON(t, &body)

	if !strings.HasPrefix(body.RawKey, "pc_live_") {
		t.Errorf("raw_key should start with pc_live_: %q", body.RawKey)
	}
	if body.Key.ID == "" {
		t.Error("id empty")
	}
}

func TestCreateAPIKey_MissingName_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/api-keys", map[string]any{
		"name":   "",
		"scopes": []string{"monitors:read"},
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestListAPIKeys_DoesNotReturnRawToken(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/api-keys", map[string]any{
		"name":   "ci-bot",
		"scopes": []string{"monitors:read"},
	})
	harness.AssertStatus(t, cr, 201)
	var created struct {
		RawKey string `json:"raw_key"`
	}
	cr.JSON(t, &created)

	resp := s.GET(t, "/api/api-keys")
	harness.AssertStatus(t, resp, 200)
	if strings.Contains(string(resp.Body), created.RawKey) {
		t.Error("list endpoint leaked raw token")
	}
}

func TestAPIKey_BearerAuth_Works(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/api-keys", map[string]any{
		"name":   "bot",
		"scopes": []string{"monitors:read"},
	})
	var key struct {
		RawKey string `json:"raw_key"`
	}
	cr.JSON(t, &key)

	bearer := h.App.WithBearer(key.RawKey)
	resp := bearer.GET(t, "/api/monitors")
	if resp.Status == 401 {
		t.Fatalf("bearer auth failed: %s", resp.Body)
	}
}

func TestRevokeAPIKey_InvalidatesFurtherCalls(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/api-keys", map[string]any{
		"name":   "doomed",
		"scopes": []string{"monitors:read"},
	})
	var key struct {
		RawKey string `json:"raw_key"`
		Key    struct {
			ID string `json:"id"`
		} `json:"key"`
	}
	cr.JSON(t, &key)

	del := s.DELETE(t, "/api/api-keys/"+key.Key.ID)
	harness.AssertStatus(t, del, 204)

	bearer := h.App.WithBearer(key.RawKey)
	resp := bearer.GET(t, "/api/monitors")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestListAPIKeys_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.GET(t, "/api/api-keys")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}
