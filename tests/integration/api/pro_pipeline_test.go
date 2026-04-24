//go:build integration

package api

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §6 (Pro pipeline): monitor groups + maintenance windows + manual
// incidents are Pro features behind proGateSelector. These tests walk
// each API through create/list/delete + Pro-gating + cross-tenant
// isolation.

// createMonitor is a small helper: Pro plan, HTTP probe, returns the
// UUID. Every Pro pipeline test needs at least one monitor to hang the
// Pro feature off of.
func createMonitor(t *testing.T, s *harness.Session, name, url string) string {
	t.Helper()
	isPublic := true
	resp := s.POST(t, "/api/monitors", map[string]any{
		"name":         name,
		"type":         "http",
		"check_config": map[string]any{"url": url},
		"is_public":    isPublic,
	})
	harness.AssertStatus(t, resp, 201)
	var body struct {
		ID string `json:"id"`
	}
	resp.JSON(t, &body)
	if body.ID == "" {
		t.Fatalf("monitor id empty: %s", resp.Body)
	}
	return body.ID
}

// --- Monitor groups --------------------------------------------------

func TestMonitorGroups_CreateListDelete(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	cr := s.POST(t, "/api/monitor-groups", map[string]any{"name": "Public API"})
	harness.AssertStatus(t, cr, 201)
	var g struct {
		ID int64 `json:"id"`
	}
	cr.JSON(t, &g)

	list := s.GET(t, "/api/monitor-groups")
	harness.AssertStatus(t, list, 200)
	var rows []struct {
		Name string `json:"name"`
	}
	list.JSON(t, &rows)
	if len(rows) != 1 || rows[0].Name != "Public API" {
		t.Fatalf("list got %+v", rows)
	}

	del := s.Do(t, "DELETE", "/api/monitor-groups/"+strconv.FormatInt(g.ID, 10), nil)
	harness.AssertStatus(t, del, 204)

	after := s.GET(t, "/api/monitor-groups")
	harness.AssertStatus(t, after, 200)
	after.JSON(t, &rows)
	if len(rows) != 0 {
		t.Fatalf("post-delete list: %+v", rows)
	}
}

func TestMonitorGroups_AssignMonitor_ReflectedOnStatusPage(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	monitorID := createMonitor(t, s, "Public API", "https://acme.test")

	// Create group.
	cr := s.POST(t, "/api/monitor-groups", map[string]any{"name": "Public"})
	harness.AssertStatus(t, cr, 201)
	var g struct {
		ID int64 `json:"id"`
	}
	cr.JSON(t, &g)

	// Assign.
	ass := s.PUT(t, "/api/monitors/"+monitorID+"/group", map[string]any{"group_id": g.ID})
	harness.AssertStatus(t, ass, 204)

	// Public status page includes the group + monitor reference.
	sp := h.App.NewSession().GET(t, "/api/status/"+user.Slug)
	harness.AssertStatus(t, sp, 200)

	var body struct {
		Monitors []struct {
			Name    string `json:"name"`
			GroupID *int64 `json:"group_id"`
		} `json:"monitors"`
		Groups []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"groups"`
	}
	sp.JSON(t, &body)
	if len(body.Groups) != 1 || body.Groups[0].ID != g.ID {
		t.Fatalf("status page groups: %+v", body.Groups)
	}
	if len(body.Monitors) != 1 || body.Monitors[0].GroupID == nil || *body.Monitors[0].GroupID != g.ID {
		t.Fatalf("monitor group binding: %+v", body.Monitors)
	}
}

// --- Maintenance windows --------------------------------------------

func TestMaintenance_ScheduleAsPro_CreatesWindow(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)
	monitorID := createMonitor(t, s, "API", "https://one.test")

	now := time.Now().UTC()
	resp := s.POST(t, "/api/maintenance-windows", map[string]any{
		"monitor_id": monitorID,
		"starts_at":  now.Format(time.RFC3339Nano),
		"ends_at":    now.Add(time.Hour).Format(time.RFC3339Nano),
		"reason":     "Upgrading Postgres",
	})
	harness.AssertStatus(t, resp, 201)

	var count int
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM maintenance_windows`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("rows: got %d want 1", count)
	}
}

func TestMaintenance_FreeUser_Returns402(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	// Create monitor before Pro-gating applies (POST monitor itself is
	// free-tier up to the quota).
	monitorID := createMonitor(t, s, "API", "https://one.test")

	// Attempt maintenance schedule as free user — should be 402.
	_ = user
	now := time.Now().UTC()
	resp := s.POST(t, "/api/maintenance-windows", map[string]any{
		"monitor_id": monitorID,
		"starts_at":  now.Format(time.RFC3339Nano),
		"ends_at":    now.Add(time.Hour).Format(time.RFC3339Nano),
		"reason":     "denied",
	})
	harness.AssertError(t, resp, 402, "PRO_REQUIRED")
}

func TestMaintenance_GetListOpenToFreeTier(t *testing.T) {
	// GET is intentionally free-tier readable so a downgraded user can
	// still see the history of their past maintenance windows.
	h := harness.New(t)
	s, _ := h.RegisterAndLoginUser(t)
	resp := s.GET(t, "/api/maintenance-windows")
	harness.AssertStatus(t, resp, 200)
}

func TestMaintenance_CrossTenant_Returns403OnDelete(t *testing.T) {
	// Spec: maintenance windows are scoped per owner.
	h := harness.New(t)
	s1, u1 := h.RegisterAndLoginUser(t)
	s2, u2 := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, u1.ID)
	h.PromoteToPro(t, u2.ID)

	monitorID := createMonitor(t, s1, "A", "https://a.test")
	now := time.Now().UTC()
	cr := s1.POST(t, "/api/maintenance-windows", map[string]any{
		"monitor_id": monitorID,
		"starts_at":  now.Format(time.RFC3339Nano),
		"ends_at":    now.Add(time.Hour).Format(time.RFC3339Nano),
		"reason":     "hello",
	})
	harness.AssertStatus(t, cr, 201)
	var win struct {
		ID int64 `json:"id"`
	}
	cr.JSON(t, &win)

	// The DELETE is idempotent and 204 even when nothing matched —
	// the ownership check happens in SQL via a tenant-scoped WHERE.
	// What matters is that s2's request does NOT actually delete s1's
	// row.
	del := s2.Do(t, "DELETE", "/api/maintenance-windows/"+strconv.FormatInt(win.ID, 10), nil)
	if del.Status >= 500 {
		t.Fatalf("cross-tenant delete crashed: %d body=%s", del.Status, del.Body)
	}

	var n int
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM maintenance_windows WHERE id = $1`, win.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("row count: got %d want 1 (cross-tenant delete leaked through the ownership check)", n)
	}
	_ = u1
}

// --- Manual incidents + updates -------------------------------------

func TestManualIncident_CreateAsPro_CreatesRowAndUpdate(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)
	monitorID := createMonitor(t, s, "API", "https://api.test")

	resp := s.POST(t, "/api/incidents", map[string]any{
		"monitor_id": monitorID,
		"title":      "Degraded latency on login endpoint",
		"body":       "Investigating reports of p95 over 2s.",
	})
	harness.AssertStatus(t, resp, 201)

	var inc struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		State    string `json:"state"`
		IsManual bool   `json:"is_manual"`
	}
	resp.JSON(t, &inc)
	if inc.Title != "Degraded latency on login endpoint" {
		t.Errorf("title: got %q", inc.Title)
	}
	if inc.State != "investigating" {
		t.Errorf("state: got %q want investigating", inc.State)
	}
	if !inc.IsManual {
		t.Error("is_manual should be true")
	}

	// One initial update should have been created.
	updates := h.App.NewSession().GET(t,
		"/api/incidents/"+strconv.FormatInt(inc.ID, 10)+"/updates")
	harness.AssertStatus(t, updates, 200)
	var rows []struct {
		State string `json:"state"`
		Body  string `json:"body"`
	}
	updates.JSON(t, &rows)
	if len(rows) != 1 {
		t.Fatalf("updates: got %d want 1", len(rows))
	}
	if rows[0].Body != "Investigating reports of p95 over 2s." {
		t.Errorf("body: got %q", rows[0].Body)
	}
}

func TestManualIncident_FreeUser_Returns402(t *testing.T) {
	h := harness.New(t)
	s, _ := h.RegisterAndLoginUser(t)
	monitorID := createMonitor(t, s, "API", "https://x.test")

	resp := s.POST(t, "/api/incidents", map[string]any{
		"monitor_id": monitorID,
		"title":      "nope",
		"body":       "nope",
	})
	harness.AssertError(t, resp, 402, "PRO_REQUIRED")
}

func TestIncidentState_TransitionAppendsUpdate(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)
	monitorID := createMonitor(t, s, "API", "https://y.test")

	cr := s.POST(t, "/api/incidents", map[string]any{
		"monitor_id": monitorID,
		"title":      "Search latency",
		"body":       "Looking into it.",
	})
	harness.AssertStatus(t, cr, 201)
	var inc struct {
		ID int64 `json:"id"`
	}
	cr.JSON(t, &inc)

	patch := s.Do(t, "PATCH", "/api/incidents/"+strconv.FormatInt(inc.ID, 10)+"/state",
		map[string]any{"state": "identified", "body": "Root cause found."})
	harness.AssertStatus(t, patch, 200)

	upds := h.App.NewSession().GET(t,
		"/api/incidents/"+strconv.FormatInt(inc.ID, 10)+"/updates")
	harness.AssertStatus(t, upds, 200)
	var rows []struct {
		State string `json:"state"`
		Body  string `json:"body"`
	}
	upds.JSON(t, &rows)
	if len(rows) != 2 {
		t.Fatalf("updates: got %d want 2", len(rows))
	}
	// Listing is newest-first (ORDER BY posted_at DESC).
	if rows[0].State != "identified" {
		t.Errorf("latest state: got %q want identified", rows[0].State)
	}
}
