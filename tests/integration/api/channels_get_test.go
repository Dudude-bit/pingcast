//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestGetChannel_Happy_ReturnsChannel(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	create := s.POST(t, "/api/channels", map[string]any{
		"name": "ops",
		"type": "webhook",
		"config": map[string]any{
			"url": "https://example.com/hook",
		},
	})
	harness.AssertStatus(t, create, 201)

	var created struct {
		ID string `json:"id"`
	}
	create.JSON(t, &created)
	if created.ID == "" {
		t.Fatalf("create response missing id: %s", create.Body)
	}

	resp := s.GET(t, "/api/channels/"+created.ID)
	harness.AssertStatus(t, resp, 200)

	var got struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	resp.JSON(t, &got)
	if got.ID != created.ID {
		t.Errorf("id mismatch: got %q, want %q", got.ID, created.ID)
	}
	if got.Name != "ops" {
		t.Errorf("name: got %q", got.Name)
	}
	if got.Type != "webhook" {
		t.Errorf("type: got %q", got.Type)
	}
}

func TestGetChannel_MalformedUUID_Returns400(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/channels/not-a-uuid")
	// Fiber's param parser handles UUID; when the OpenAPI route has a
	// UUID param, oapi-codegen emits a 400-family response for parse
	// failure. Spec §1: MALFORMED_PARAM.
	if resp.Status != 400 {
		t.Fatalf("status: want 400 got %d body=%s", resp.Status, resp.Body)
	}
}

func TestGetChannel_NotFound_Returns404(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/channels/00000000-0000-0000-0000-000000000000")
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}

func TestGetChannel_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)

	create := sA.POST(t, "/api/channels", map[string]any{
		"name": "a",
		"type": "webhook",
		"config": map[string]any{
			"url": "https://example.com/hook",
		},
	})
	harness.AssertStatus(t, create, 201)
	var created struct{ ID string }
	create.JSON(t, &created)

	resp := sB.GET(t, "/api/channels/"+created.ID)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

func TestGetChannel_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.GET(t, "/api/channels/00000000-0000-0000-0000-000000000000")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}
