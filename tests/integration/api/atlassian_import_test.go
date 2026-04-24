//go:build integration

package api

import (
	"context"
	"strings"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §7 (Atlassian importer): valid schema_version=1.0 export goes
// through in one transaction; unknown schema / malformed JSON rolls
// back; components without URLs are counted as skipped. Pro-gated.

const atlassianHappyExport = `{
  "schema_version": "1.0",
  "page": {"name": "Acme", "url": "https://status.acme.example"},
  "components": [
    {"id": "c1", "name": "API", "url": "https://api.acme.example/health", "status": "operational"},
    {"id": "c2", "name": "Dashboard", "url": "https://app.acme.example", "status": "operational"},
    {"id": "cg", "name": "Public API Services", "url": "", "status": "operational"}
  ],
  "incidents": [
    {
      "id": "i1",
      "name": "API slowness",
      "status": "resolved",
      "started_at": "2026-04-01T10:00:00Z",
      "resolved_at": "2026-04-01T10:45:00Z",
      "components": ["c1"],
      "incident_updates": [
        {"status": "investigating", "body": "Looking into it.", "created_at": "2026-04-01T10:02:00Z"},
        {"status": "monitoring",    "body": "Fix deployed.",    "created_at": "2026-04-01T10:30:00Z"},
        {"status": "resolved",      "body": "All clear.",       "created_at": "2026-04-01T10:45:00Z"}
      ]
    },
    {
      "id": "i2",
      "name": "Dashboard 500s",
      "status": "postmortem",
      "started_at": "2026-04-02T12:00:00Z",
      "resolved_at": "2026-04-02T13:00:00Z",
      "components": ["c2"],
      "incident_updates": [
        {"status": "resolved", "body": "Rolled back.", "created_at": "2026-04-02T13:00:00Z"}
      ]
    }
  ]
}`

func TestAtlassianImport_HappyPath_CreatesMonitorsAndIncidents(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	resp := s.DoRaw(t, "POST", "/api/import/atlassian",
		"application/json", []byte(atlassianHappyExport))
	harness.AssertStatus(t, resp, 200)

	var body struct {
		MonitorsCreated   int `json:"monitors_created"`
		IncidentsCreated  int `json:"incidents_created"`
		UpdatesCreated    int `json:"updates_created"`
		ComponentsSkipped int `json:"components_skipped"`
	}
	resp.JSON(t, &body)

	if body.MonitorsCreated != 2 {
		t.Errorf("MonitorsCreated: got %d want 2", body.MonitorsCreated)
	}
	if body.IncidentsCreated != 2 {
		t.Errorf("IncidentsCreated: got %d want 2", body.IncidentsCreated)
	}
	if body.UpdatesCreated != 4 {
		t.Errorf("UpdatesCreated: got %d want 4", body.UpdatesCreated)
	}
	if body.ComponentsSkipped != 1 {
		t.Errorf("ComponentsSkipped: got %d want 1 (one component without URL)", body.ComponentsSkipped)
	}

	ctx := context.Background()
	var monitorCount, incidentCount, updateCount int
	if err := h.App.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM monitors WHERE user_id = $1`, user.ID).Scan(&monitorCount); err != nil {
		t.Fatalf("count monitors: %v", err)
	}
	if monitorCount != 2 {
		t.Errorf("DB monitors: got %d want 2", monitorCount)
	}
	if err := h.App.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM incidents i JOIN monitors m ON m.id = i.monitor_id
		 WHERE m.user_id = $1 AND i.is_manual = TRUE`, user.ID).Scan(&incidentCount); err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	if incidentCount != 2 {
		t.Errorf("DB incidents: got %d want 2", incidentCount)
	}
	if err := h.App.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM incident_updates iu JOIN incidents i ON i.id = iu.incident_id
		 JOIN monitors m ON m.id = i.monitor_id WHERE m.user_id = $1`, user.ID).Scan(&updateCount); err != nil {
		t.Fatalf("count updates: %v", err)
	}
	if updateCount != 4 {
		t.Errorf("DB updates: got %d want 4", updateCount)
	}
}

func TestAtlassianImport_UnsupportedSchema_Returns400AndNoWrites(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	resp := s.DoRaw(t, "POST", "/api/import/atlassian",
		"application/json", []byte(`{"schema_version":"2.0","components":[],"incidents":[]}`))
	harness.AssertError(t, resp, 400, "ATLASSIAN_IMPORT_FAILED")

	var count int
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM monitors WHERE user_id = $1`, user.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected zero monitors on failed import, got %d", count)
	}
}

func TestAtlassianImport_MalformedJSON_Returns400(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	resp := s.DoRaw(t, "POST", "/api/import/atlassian",
		"application/json", []byte("{not-json"))
	harness.AssertError(t, resp, 400, "ATLASSIAN_IMPORT_FAILED")
}

func TestAtlassianImport_UnknownIncidentState_RollsBack(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)
	h.PromoteToPro(t, user.ID)

	// Two valid components so monitor create succeeds; one incident
	// with a state we don't know about → the whole transaction must
	// roll back, including the monitor rows.
	bad := `{
	  "schema_version": "1.0",
	  "page": {"name":"x","url":"https://x"},
	  "components": [
	    {"id":"c1","name":"A","url":"https://a.example","status":"operational"}
	  ],
	  "incidents": [
	    {"id":"i1","name":"bad","status":"scheduled","started_at":"2026-04-01T10:00:00Z",
	     "resolved_at":null,"components":["c1"],"incident_updates":[]}
	  ]
	}`
	resp := s.DoRaw(t, "POST", "/api/import/atlassian", "application/json", []byte(bad))
	harness.AssertError(t, resp, 400, "ATLASSIAN_IMPORT_FAILED")

	var count int
	if err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM monitors WHERE user_id = $1`, user.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("monitors left after rolled-back import: got %d want 0", count)
	}
}

func TestAtlassianImport_FreeUser_Returns402(t *testing.T) {
	h := harness.New(t)
	s, _ := h.RegisterAndLoginUser(t)
	// No PromoteToPro — free tier must be gated.

	resp := s.DoRaw(t, "POST", "/api/import/atlassian",
		"application/json", []byte(atlassianHappyExport))
	harness.AssertError(t, resp, 402, "PRO_REQUIRED")
}

func TestAtlassianImport_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	anon := h.App.NewSession()

	resp := anon.DoRaw(t, "POST", "/api/import/atlassian",
		"application/json", []byte(atlassianHappyExport))
	if resp.Status != 401 {
		t.Fatalf("want 401 got %d body=%s", resp.Status, resp.Body)
	}
	if !strings.Contains(string(resp.Body), "error") {
		t.Fatalf("want error envelope, got %s", resp.Body)
	}
}
