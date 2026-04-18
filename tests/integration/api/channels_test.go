//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §4 (Channels): CRUD + validation. GET /api/channels/{id} is
// covered in channels_get_test.go.

func webhookChannelBody(name string) map[string]any {
	return map[string]any{
		"name": name,
		"type": "webhook",
		"config": map[string]any{
			"url": "https://example.com/hook",
		},
	}
}

func telegramChannelBody(name string) map[string]any {
	return map[string]any{
		"name": name,
		"type": "telegram",
		"config": map[string]any{
			"bot_token": "12345:SECRETDEADBEEF",
			"chat_id":   777, // int64 per the telegram config schema
		},
	}
}

// --- List -----------------------------------------------------------

func TestListChannels_Empty_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/channels")
	harness.AssertStatus(t, resp, 200)
}

func TestListChannels_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.GET(t, "/api/channels")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

// --- Create ---------------------------------------------------------

func TestCreateChannel_Webhook_Happy_Returns201(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/channels", webhookChannelBody("alerts"))
	harness.AssertStatus(t, resp, 201)

	var ch struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	resp.JSON(t, &ch)
	if ch.Name != "alerts" {
		t.Errorf("name: %q", ch.Name)
	}
	if ch.Type != "webhook" {
		t.Errorf("type: %q", ch.Type)
	}
	if ch.ID == "" {
		t.Error("id empty")
	}
}

func TestCreateChannel_Telegram_Happy_Returns201(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/channels", telegramChannelBody("ops"))
	harness.AssertStatus(t, resp, 201)
}

func TestCreateChannel_EmptyName_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	body := webhookChannelBody("")
	resp := s.POST(t, "/api/channels", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestCreateChannel_UnknownType_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/channels", map[string]any{
		"name":   "weird",
		"type":   "carrier-pigeon",
		"config": map[string]any{},
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestCreateChannel_MalformedJSON_Returns400(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.DoRaw(t, "POST", "/api/channels", "application/json", []byte("{"))
	harness.AssertError(t, resp, 400, "MALFORMED_JSON")
}

func TestCreateChannel_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/channels", webhookChannelBody("x"))
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

// --- Update ---------------------------------------------------------

func TestUpdateChannel_Happy_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/channels", webhookChannelBody("ops"))
	harness.AssertStatus(t, cr, 201)
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	name := "ops-v2"
	resp := s.PUT(t, "/api/channels/"+ch.ID, map[string]any{
		"name": name,
	})
	harness.AssertStatus(t, resp, 200)
}

func TestUpdateChannel_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/channels", webhookChannelBody("a"))
	harness.AssertStatus(t, cr, 201)
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	resp := sB.PUT(t, "/api/channels/"+ch.ID, map[string]any{
		"name": "hacked",
	})
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Delete ---------------------------------------------------------

func TestDeleteChannel_Happy_Returns204(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/channels", webhookChannelBody("ops"))
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	resp := s.DELETE(t, "/api/channels/"+ch.ID)
	harness.AssertStatus(t, resp, 204)

	resp2 := s.DELETE(t, "/api/channels/"+ch.ID)
	harness.AssertError(t, resp2, 404, "NOT_FOUND")
}

func TestDeleteChannel_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/channels", webhookChannelBody("a"))
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	resp := sB.DELETE(t, "/api/channels/"+ch.ID)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Channel types --------------------------------------------------

func TestListChannelTypes_Authenticated_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/channel-types")
	harness.AssertStatus(t, resp, 200)
}
