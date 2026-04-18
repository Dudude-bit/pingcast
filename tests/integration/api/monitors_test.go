//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §4 (Monitors): CRUD + pause + cross-tenant. All tests assume
// the canonical envelope for errors.

func newMonitor(name string) map[string]any {
	return map[string]any{
		"name":             name,
		"type":             "http",
		"check_config":     map[string]any{"url": "https://example.com", "method": "GET"},
		"interval_seconds": 300,
	}
}

// --- List -----------------------------------------------------------

func TestListMonitors_Empty_Returns200EmptyArray(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitors")
	harness.AssertStatus(t, resp, 200)

	var items []map[string]any
	resp.JSON(t, &items)
	if len(items) != 0 {
		t.Errorf("want empty list, got %d items", len(items))
	}
}

func TestListMonitors_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.GET(t, "/api/monitors")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

// --- Create ---------------------------------------------------------

func TestCreateMonitor_Happy_Returns201(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/monitors", newMonitor("api"))
	harness.AssertStatus(t, resp, 201)

	var m struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	resp.JSON(t, &m)
	if m.Name != "api" {
		t.Errorf("name: %q", m.Name)
	}
	if m.ID == "" {
		t.Error("id empty")
	}
}

func TestCreateMonitor_IntervalBelowMin_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	body := newMonitor("too-fast")
	body["interval_seconds"] = 10 // below 30s minimum
	resp := s.POST(t, "/api/monitors", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestCreateMonitor_MissingName_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	body := newMonitor("")
	resp := s.POST(t, "/api/monitors", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestCreateMonitor_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/monitors", newMonitor("x"))
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

// --- Get ------------------------------------------------------------

func TestGetMonitor_Happy_ReturnsDetail(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", newMonitor("api"))
	harness.AssertStatus(t, cr, 201)
	var created struct{ ID string }
	cr.JSON(t, &created)

	resp := s.GET(t, "/api/monitors/"+created.ID)
	harness.AssertStatus(t, resp, 200)
}

func TestGetMonitor_MalformedID_Returns400(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitors/not-a-uuid")
	if resp.Status != 400 {
		t.Fatalf("want 400 for malformed UUID, got %d body=%s", resp.Status, resp.Body)
	}
}

func TestGetMonitor_NotFound_Returns404(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitors/00000000-0000-0000-0000-000000000000")
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}

func TestGetMonitor_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/monitors", newMonitor("alpha"))
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := sB.GET(t, "/api/monitors/"+m.ID)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Update ---------------------------------------------------------

func TestUpdateMonitor_Happy_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", newMonitor("api"))
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := s.PUT(t, "/api/monitors/"+m.ID, map[string]any{
		"name": "api-v2",
	})
	harness.AssertStatus(t, resp, 200)

	var updated struct{ Name string }
	resp.JSON(t, &updated)
	if updated.Name != "api-v2" {
		t.Errorf("name not updated: got %q", updated.Name)
	}
}

func TestUpdateMonitor_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/monitors", newMonitor("alpha"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := sB.PUT(t, "/api/monitors/"+m.ID, map[string]any{"name": "hack"})
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Delete ---------------------------------------------------------

func TestDeleteMonitor_Happy_Returns204(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", newMonitor("api"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := s.DELETE(t, "/api/monitors/" + m.ID)
	harness.AssertStatus(t, resp, 204)

	// Second delete is 404
	resp2 := s.DELETE(t, "/api/monitors/" + m.ID)
	harness.AssertError(t, resp2, 404, "NOT_FOUND")
}

func TestDeleteMonitor_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/monitors", newMonitor("alpha"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := sB.DELETE(t, "/api/monitors/" + m.ID)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Pause ----------------------------------------------------------

func TestPauseMonitor_Toggles(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", newMonitor("api"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	r1 := s.POST(t, "/api/monitors/"+m.ID+"/pause", nil)
	harness.AssertStatus(t, r1, 200)
	var after1 struct {
		IsPaused bool `json:"is_paused"`
	}
	r1.JSON(t, &after1)
	if !after1.IsPaused {
		t.Error("expected paused=true after first toggle")
	}

	r2 := s.POST(t, "/api/monitors/"+m.ID+"/pause", nil)
	harness.AssertStatus(t, r2, 200)
	var after2 struct {
		IsPaused bool `json:"is_paused"`
	}
	r2.JSON(t, &after2)
	if after2.IsPaused {
		t.Error("expected paused=false after second toggle")
	}
}

func TestPauseMonitor_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/monitors", newMonitor("alpha"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := sB.POST(t, "/api/monitors/"+m.ID+"/pause", nil)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Monitor types --------------------------------------------------

func TestListMonitorTypes_Authenticated_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitor-types")
	harness.AssertStatus(t, resp, 200)
}
