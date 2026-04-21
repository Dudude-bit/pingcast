package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// AtlassianImporter accepts an Atlassian Statuspage v1 JSON export and
// creates equivalent monitors + incidents + updates under a target user.
// The import is all-or-nothing inside one transaction via txm.Do.
type AtlassianImporter struct {
	monitors        port.MonitorRepo
	incidents       port.IncidentRepo
	incidentUpdates port.IncidentUpdateRepo
	txm             port.TxManager
	clock           port.Clock
}

func NewAtlassianImporter(
	monitors port.MonitorRepo,
	incidents port.IncidentRepo,
	updates port.IncidentUpdateRepo,
	txm port.TxManager,
	clock port.Clock,
) *AtlassianImporter {
	return &AtlassianImporter{
		monitors:        monitors,
		incidents:       incidents,
		incidentUpdates: updates,
		txm:             txm,
		clock:           clock,
	}
}

// atlassianExport is the JSON shape we accept. Pinned to schema_version
// "1.0" — unknown versions are rejected with a helpful error.
type atlassianExport struct {
	SchemaVersion string               `json:"schema_version"`
	Page          atlassianPage        `json:"page"`
	Components    []atlassianComponent `json:"components"`
	Incidents     []atlassianIncident  `json:"incidents"`
}

type atlassianPage struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type atlassianComponent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

type atlassianIncident struct {
	ID              string                    `json:"id"`
	Name            string                    `json:"name"`
	Status          string                    `json:"status"`
	StartedAt       time.Time                 `json:"started_at"`
	ResolvedAt      *time.Time                `json:"resolved_at"`
	Components      []string                  `json:"components"`
	IncidentUpdates []atlassianIncidentUpdate `json:"incident_updates"`
}

type atlassianIncidentUpdate struct {
	Status    string    `json:"status"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// ImportResult summarises what was created.
type ImportResult struct {
	MonitorsCreated  int
	IncidentsCreated int
	UpdatesCreated   int
	ComponentsSkipped int
}

// Import parses + validates + writes in one atomic transaction. Errors
// during any step roll back the whole import so a half-migrated SaaS
// doesn't show up in the dashboard.
func (i *AtlassianImporter) Import(ctx context.Context, userID uuid.UUID, src io.Reader) (ImportResult, error) {
	raw, err := io.ReadAll(src)
	if err != nil {
		return ImportResult{}, fmt.Errorf("read import body: %w", err)
	}
	var exp atlassianExport
	if err := json.Unmarshal(raw, &exp); err != nil {
		return ImportResult{}, fmt.Errorf("invalid atlassian JSON: %w", err)
	}
	if exp.SchemaVersion != "1.0" {
		return ImportResult{}, fmt.Errorf(
			"unsupported atlassian schema version %q (this importer pins to 1.0)",
			exp.SchemaVersion,
		)
	}

	res := ImportResult{}
	// componentToMonitor maps Atlassian component ID → our monitor ID
	// so we can wire incidents to the right monitor later in the same
	// transaction.
	componentToMonitor := make(map[string]uuid.UUID, len(exp.Components))

	if err := i.txm.Do(ctx, func(ctx context.Context) error {
		for _, comp := range exp.Components {
			if comp.URL == "" {
				// Atlassian supports components without probe URLs
				// (purely organisational groups). We can't create a
				// monitor without a target; count as skipped.
				res.ComponentsSkipped++
				continue
			}
			configJSON, mErr := json.Marshal(map[string]any{"url": comp.URL})
			if mErr != nil {
				return fmt.Errorf("marshal component %q config: %w", comp.ID, mErr)
			}
			mon := &domain.Monitor{
				UserID:             userID,
				Name:               comp.Name,
				Type:               domain.MonitorHTTP,
				CheckConfig:        configJSON,
				IntervalSeconds:    60,
				AlertAfterFailures: 2,
				IsPaused:           false,
				IsPublic:           true,
				CurrentStatus:      domain.StatusUnknown,
			}
			created, cErr := i.monitors.Create(ctx, mon)
			if cErr != nil {
				return fmt.Errorf("create monitor for component %q: %w", comp.Name, cErr)
			}
			componentToMonitor[comp.ID] = created.ID
			res.MonitorsCreated++
		}

		for _, inc := range exp.Incidents {
			monitorID, ok := pickMonitorForIncident(inc, componentToMonitor)
			if !ok {
				// Skip incidents that reference no known components —
				// can't attach them anywhere without a monitor_id.
				continue
			}
			state, sErr := mapAtlassianState(inc.Status)
			if sErr != nil {
				return sErr
			}
			title := inc.Name
			createdInc, cErr := i.incidents.Create(ctx, port.CreateIncidentInput{
				MonitorID: monitorID,
				Cause:     inc.Name,
				State:     state,
				IsManual:  true,
				Title:     &title,
			})
			if cErr != nil {
				return fmt.Errorf("create incident %q: %w", inc.Name, cErr)
			}
			res.IncidentsCreated++

			if inc.ResolvedAt != nil {
				if rErr := i.incidents.Resolve(ctx, createdInc.ID, *inc.ResolvedAt); rErr != nil {
					return fmt.Errorf("resolve incident %q: %w", inc.Name, rErr)
				}
			}

			for _, u := range inc.IncidentUpdates {
				uState, uErr := mapAtlassianState(u.Status)
				if uErr != nil {
					return uErr
				}
				if _, wErr := i.incidentUpdates.Create(ctx, port.CreateIncidentUpdateInput{
					IncidentID:     createdInc.ID,
					State:          uState,
					Body:           u.Body,
					PostedByUserID: userID,
				}); wErr != nil {
					return fmt.Errorf("create update on incident %q: %w", inc.Name, wErr)
				}
				res.UpdatesCreated++
			}
		}
		return nil
	}); err != nil {
		return ImportResult{}, err
	}
	return res, nil
}

func pickMonitorForIncident(inc atlassianIncident, m map[string]uuid.UUID) (uuid.UUID, bool) {
	// Atlassian allows an incident to affect multiple components; we
	// currently bind each incident to a single monitor. Pick the first
	// component we have a monitor for; skip the incident otherwise.
	for _, cid := range inc.Components {
		if id, ok := m[cid]; ok {
			return id, true
		}
	}
	return uuid.Nil, false
}

// mapAtlassianState converts Atlassian's state name to ours. Atlassian
// has a 'postmortem' value that we collapse to 'resolved' since we
// don't model post-incident reports separately.
func mapAtlassianState(s string) (domain.IncidentState, error) {
	switch s {
	case "investigating":
		return domain.IncidentStateInvestigating, nil
	case "identified":
		return domain.IncidentStateIdentified, nil
	case "monitoring":
		return domain.IncidentStateMonitoring, nil
	case "resolved", "postmortem":
		return domain.IncidentStateResolved, nil
	}
	return "", fmt.Errorf("unknown atlassian state: %q", s)
}
