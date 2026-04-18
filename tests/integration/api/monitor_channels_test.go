//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §4 (Monitor-Channel): bind/unbind with tenant isolation.

func setupMonitorAndChannel(t *testing.T, s *harness.Session) (monitorID, channelID string) {
	t.Helper()

	m := s.POST(t, "/api/monitors", newMonitor("bound-monitor"))
	harness.AssertStatus(t, m, 201)
	var mm struct{ ID string }
	m.JSON(t, &mm)

	c := s.POST(t, "/api/channels", webhookChannelBody("bound-channel"))
	harness.AssertStatus(t, c, 201)
	var cc struct{ ID string }
	c.JSON(t, &cc)

	return mm.ID, cc.ID
}

func TestBindChannel_Happy_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	mID, cID := setupMonitorAndChannel(t, s)

	resp := s.POST(t, "/api/monitors/"+mID+"/channels",
		map[string]any{"channel_id": cID})
	harness.AssertStatus(t, resp, 200)
}

func TestBindChannel_ForeignMonitor_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	mID, _ := setupMonitorAndChannel(t, sA)
	_, cID := setupMonitorAndChannel(t, sB)

	resp := sB.POST(t, "/api/monitors/"+mID+"/channels",
		map[string]any{"channel_id": cID})
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

func TestBindChannel_ForeignChannel_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	mA, _ := setupMonitorAndChannel(t, sA)
	_, cB := setupMonitorAndChannel(t, sB)

	resp := sA.POST(t, "/api/monitors/"+mA+"/channels",
		map[string]any{"channel_id": cB})
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

func TestUnbindChannel_Happy_Returns204(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	mID, cID := setupMonitorAndChannel(t, s)

	s.POST(t, "/api/monitors/"+mID+"/channels",
		map[string]any{"channel_id": cID})

	resp := s.DELETE(t, "/api/monitors/"+mID+"/channels/"+cID)
	harness.AssertStatus(t, resp, 204)
}

func TestBindChannel_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/monitors/00000000-0000-0000-0000-000000000000/channels",
		map[string]any{"channel_id": "00000000-0000-0000-0000-000000000000"})
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}
