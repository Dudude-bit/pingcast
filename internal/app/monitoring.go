package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// ErrEventPublishFailed indicates the DB write succeeded but the event
// could not be published to the message bus. The caller should treat
// the mutation as applied but warn the user that sync may be delayed.
var ErrEventPublishFailed = fmt.Errorf("event publish failed")

type MonitoringService struct {
	monitors        port.MonitorRepo
	channels        port.ChannelRepo
	checkResults    port.CheckResultRepo
	incidents       port.IncidentRepo
	incidentUpdates port.IncidentUpdateRepo
	maintenance     port.MaintenanceWindowRepo
	groups          port.MonitorGroupRepo
	users           port.UserRepo
	uptime          port.UptimeRepo
	txm             port.TxManager
	alerts          port.AlertEventPublisher
	events          port.MonitorEventPublisher
	registry        port.CheckerRegistry
	metrics         port.Metrics
	clock           port.Clock
}

func NewMonitoringService(
	monitors port.MonitorRepo,
	channels port.ChannelRepo,
	checkResults port.CheckResultRepo,
	incidents port.IncidentRepo,
	incidentUpdates port.IncidentUpdateRepo,
	maintenance port.MaintenanceWindowRepo,
	groups port.MonitorGroupRepo,
	users port.UserRepo,
	uptime port.UptimeRepo,
	txm port.TxManager,
	alerts port.AlertEventPublisher,
	events port.MonitorEventPublisher,
	registry port.CheckerRegistry,
	metrics port.Metrics,
	clock port.Clock,
) *MonitoringService {
	return &MonitoringService{
		monitors:        monitors,
		channels:        channels,
		checkResults:    checkResults,
		incidents:       incidents,
		incidentUpdates: incidentUpdates,
		maintenance:     maintenance,
		groups:          groups,
		users:           users,
		uptime:          uptime,
		txm:             txm,
		alerts:          alerts,
		events:          events,
		registry:        registry,
		metrics:         metrics,
		clock:           clock,
	}
}

// --- Monitor groups (Pro-gated at HTTP layer) ---

func (s *MonitoringService) CreateMonitorGroup(ctx context.Context, userID uuid.UUID, name string, ordering int) (*domain.MonitorGroup, error) {
	return s.groups.Create(ctx, userID, name, ordering)
}

func (s *MonitoringService) ListMonitorGroups(ctx context.Context, userID uuid.UUID) ([]domain.MonitorGroup, error) {
	return s.groups.ListByUserID(ctx, userID)
}

func (s *MonitoringService) UpdateMonitorGroup(ctx context.Context, id int64, userID uuid.UUID, name string, ordering int) error {
	return s.groups.Update(ctx, id, userID, name, ordering)
}

func (s *MonitoringService) DeleteMonitorGroup(ctx context.Context, id int64, userID uuid.UUID) error {
	return s.groups.Delete(ctx, id, userID)
}

func (s *MonitoringService) AssignMonitorGroup(ctx context.Context, monitorID, userID uuid.UUID, groupID *int64) error {
	return s.groups.AssignMonitor(ctx, monitorID, userID, groupID)
}

// --- Maintenance windows (Pro-gated at HTTP layer) ---

type ScheduleMaintenanceInput struct {
	MonitorID uuid.UUID
	UserID    uuid.UUID
	StartsAt  time.Time
	EndsAt    time.Time
	Reason    string
}

func (s *MonitoringService) ScheduleMaintenance(ctx context.Context, in ScheduleMaintenanceInput) (*domain.MaintenanceWindow, error) {
	if !in.EndsAt.After(in.StartsAt) {
		return nil, fmt.Errorf("ends_at must be after starts_at")
	}
	monitor, err := s.monitors.GetByID(ctx, in.MonitorID)
	if err != nil {
		return nil, err
	}
	if monitor.UserID != in.UserID {
		return nil, domain.ErrForbidden
	}
	return s.maintenance.Create(ctx, port.CreateMaintenanceWindowInput{
		MonitorID: in.MonitorID,
		StartsAt:  in.StartsAt,
		EndsAt:    in.EndsAt,
		Reason:    in.Reason,
	})
}

func (s *MonitoringService) ListMaintenanceWindows(ctx context.Context, userID uuid.UUID) ([]domain.MaintenanceWindow, error) {
	return s.maintenance.ListByUserID(ctx, userID)
}

func (s *MonitoringService) DeleteMaintenanceWindow(ctx context.Context, id int64, userID uuid.UUID) error {
	return s.maintenance.Delete(ctx, id, userID)
}

// ListIncidentsForExport returns every incident belonging to userID as
// flat rows, joined to the monitor name. Powers the Pro-only CSV
// export at /api/incidents/export.csv.
func (s *MonitoringService) ListIncidentsForExport(ctx context.Context, userID uuid.UUID) ([]port.IncidentExportRow, error) {
	return s.incidents.ListForExport(ctx, userID)
}

// ListIncidentUpdates returns the timeline for an incident. Public read
// — the HTTP layer exposes this on /api/incidents/{id}/updates without
// auth for rendering status-page timelines.
func (s *MonitoringService) ListIncidentUpdates(ctx context.Context, incidentID int64) ([]domain.IncidentUpdate, error) {
	return s.incidentUpdates.ListByIncidentID(ctx, incidentID)
}

// --- Manual incident lifecycle (Pro-gated at HTTP layer) ---

// ChangeIncidentStateInput captures a user-driven state transition on
// an incident, paired with a narrative body posted to the timeline.
type ChangeIncidentStateInput struct {
	IncidentID int64
	UserID     uuid.UUID
	NewState   domain.IncidentState
	UpdateBody string
}

// ChangeIncidentState validates ownership + transition, then atomically
// updates the incident and appends an IncidentUpdate. Resolving also
// sets resolved_at. Returns the IncidentUpdate that represents the post.
func (s *MonitoringService) ChangeIncidentState(ctx context.Context, in ChangeIncidentStateInput) (*domain.IncidentUpdate, error) {
	if !in.NewState.Valid() {
		return nil, fmt.Errorf("invalid incident state %q", in.NewState)
	}
	if strings.TrimSpace(in.UpdateBody) == "" {
		return nil, fmt.Errorf("update body is required")
	}

	inc, err := s.incidents.GetByID(ctx, in.IncidentID)
	if err != nil {
		return nil, err
	}
	monitor, err := s.monitors.GetByID(ctx, inc.MonitorID)
	if err != nil {
		return nil, err
	}
	if monitor.UserID != in.UserID {
		return nil, domain.ErrForbidden
	}
	if tErr := inc.State.CanTransitionTo(in.NewState); tErr != nil {
		return nil, tErr
	}

	var created *domain.IncidentUpdate
	if err := s.txm.Do(ctx, func(ctx context.Context) error {
		if updErr := s.incidents.UpdateState(ctx, in.IncidentID, in.NewState); updErr != nil {
			return fmt.Errorf("update state: %w", updErr)
		}
		if in.NewState == domain.IncidentStateResolved {
			if rErr := s.incidents.Resolve(ctx, in.IncidentID, s.clock.Now()); rErr != nil {
				return fmt.Errorf("resolve: %w", rErr)
			}
		}
		u, cErr := s.incidentUpdates.Create(ctx, port.CreateIncidentUpdateInput{
			IncidentID:     in.IncidentID,
			State:          in.NewState,
			Body:           in.UpdateBody,
			PostedByUserID: in.UserID,
		})
		if cErr != nil {
			return fmt.Errorf("append update: %w", cErr)
		}
		created = u
		return nil
	}); err != nil {
		return nil, err
	}
	return created, nil
}

// CreateManualIncidentInput opens a new user-authored incident against
// a monitor the user owns.
type CreateManualIncidentInput struct {
	MonitorID uuid.UUID
	UserID    uuid.UUID
	Title     string
	Body      string
}

// CreateManualIncident opens an incident in state=investigating with
// is_manual=true, and appends the initial update with the given body.
// Returns the created incident and its first update.
func (s *MonitoringService) CreateManualIncident(ctx context.Context, in CreateManualIncidentInput) (*domain.Incident, *domain.IncidentUpdate, error) {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" {
		return nil, nil, fmt.Errorf("title is required")
	}
	if body == "" {
		return nil, nil, fmt.Errorf("body is required")
	}

	monitor, err := s.monitors.GetByID(ctx, in.MonitorID)
	if err != nil {
		return nil, nil, err
	}
	if monitor.UserID != in.UserID {
		return nil, nil, domain.ErrForbidden
	}

	var inc *domain.Incident
	var upd *domain.IncidentUpdate
	if err := s.txm.Do(ctx, func(ctx context.Context) error {
		created, cErr := s.incidents.Create(ctx, port.CreateIncidentInput{
			MonitorID: in.MonitorID,
			Cause:     title, // cause falls back to title for auto-rendered summaries
			State:     domain.IncidentStateInvestigating,
			IsManual:  true,
			Title:     &title,
		})
		if cErr != nil {
			return fmt.Errorf("create incident: %w", cErr)
		}
		u, uErr := s.incidentUpdates.Create(ctx, port.CreateIncidentUpdateInput{
			IncidentID:     created.ID,
			State:          domain.IncidentStateInvestigating,
			Body:           body,
			PostedByUserID: in.UserID,
		})
		if uErr != nil {
			return fmt.Errorf("append initial update: %w", uErr)
		}
		inc = created
		upd = u
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return inc, upd, nil
}

// Registry returns the checker registry.
func (s *MonitoringService) Registry() port.CheckerRegistry {
	return s.registry
}

// RunCheck executes a health check via the registry and processes the result
// (status transitions, incidents, alerts).
func (s *MonitoringService) RunCheck(ctx context.Context, monitor *domain.Monitor) error {
	chk, err := s.registry.Get(monitor.Type)
	if err != nil {
		return fmt.Errorf("unknown monitor type %s: %w", monitor.Type, err)
	}
	result := chk.Check(ctx, monitor)
	return s.ProcessCheckResult(ctx, monitor, result)
}

type CreateMonitorInput struct {
	Name               string
	Type               domain.MonitorType
	CheckConfig        json.RawMessage
	IntervalSeconds    int
	AlertAfterFailures int
	IsPublic           bool
	ChannelIDs         []uuid.UUID
}

func (s *MonitoringService) CreateMonitor(ctx context.Context, user *domain.User, input CreateMonitorInput) (*domain.Monitor, error) {
	count, err := s.monitors.CountByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("count monitors: %w", err)
	}
	if count >= user.MonitorLimit() {
		return nil, domain.NewValidationError(
			"MONITOR_LIMIT_REACHED",
			fmt.Sprintf("monitor limit reached for %s plan", user.Plan),
		)
	}

	interval := input.IntervalSeconds
	if interval == 0 {
		interval = user.MinInterval()
	}
	if interval < user.MinInterval() {
		return nil, domain.NewValidationError(
			"INTERVAL_BELOW_TIER_MIN",
			fmt.Sprintf("interval must be at least %d seconds on the %s plan", user.MinInterval(), user.Plan),
		)
	}

	alertAfter := input.AlertAfterFailures
	if alertAfter == 0 {
		alertAfter = 3
	}

	if vErr := domain.ValidateMonitorInput(input.Name, interval, alertAfter); vErr != nil {
		return nil, vErr
	}
	if cfgErr := s.registry.ValidateConfig(input.Type, input.CheckConfig); cfgErr != nil {
		return nil, fmt.Errorf("invalid check config: %w", cfgErr)
	}

	mon := &domain.Monitor{
		UserID:             user.ID,
		Name:               strings.TrimSpace(input.Name),
		Type:               input.Type,
		CheckConfig:        input.CheckConfig,
		IntervalSeconds:    interval,
		AlertAfterFailures: alertAfter,
		IsPublic:           input.IsPublic,
		CurrentStatus:      domain.StatusUnknown,
	}

	// No channels to bind — simple create without transaction
	if len(input.ChannelIDs) == 0 {
		created, createErr := s.monitors.Create(ctx, mon)
		if createErr != nil {
			return nil, createErr
		}
		if s.metrics != nil {
			s.metrics.MonitorCreated(ctx)
		}
		if pubErr := s.publishMonitorEvent(ctx, domain.ActionCreate, created); pubErr != nil {
			return created, fmt.Errorf("%w: %w", ErrEventPublishFailed, pubErr)
		}
		return created, nil
	}

	// Transactional: create monitor + bind channels atomically via go-trm.
	// The txm.Do() auto-commits on nil, auto-rollbacks on error.
	// ctx carries the active tx — repos extract it transparently.
	var created *domain.Monitor
	err = s.txm.Do(ctx, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.monitors.Create(txCtx, mon)
		if createErr != nil {
			return fmt.Errorf("create monitor: %w", createErr)
		}
		for _, cid := range input.ChannelIDs {
			if bindErr := s.channels.BindToMonitor(txCtx, created.ID, cid); bindErr != nil {
				return fmt.Errorf("bind channel %s: %w", cid, bindErr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.metrics != nil {
		s.metrics.MonitorCreated(ctx)
	}
	if pubErr := s.publishMonitorEvent(ctx, domain.ActionCreate, created); pubErr != nil {
		return created, fmt.Errorf("%w: %w", ErrEventPublishFailed, pubErr)
	}
	return created, nil
}

type UpdateMonitorInput struct {
	Name               *string
	CheckConfig        json.RawMessage
	IntervalSeconds    *int
	AlertAfterFailures *int
	IsPaused           *bool
	IsPublic           *bool
}

func (s *MonitoringService) UpdateMonitor(ctx context.Context, user *domain.User, id uuid.UUID, input UpdateMonitorInput) (*domain.Monitor, error) {
	mon, err := s.loadOwnedMonitor(ctx, id, user.ID)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		mon.Name = strings.TrimSpace(*input.Name)
	}
	if input.CheckConfig != nil {
		if err := s.registry.ValidateConfig(mon.Type, input.CheckConfig); err != nil {
			return nil, fmt.Errorf("invalid check config: %w", err)
		}
		mon.CheckConfig = input.CheckConfig
	}
	if input.IntervalSeconds != nil {
		mon.IntervalSeconds = max(*input.IntervalSeconds, user.MinInterval())
	}
	if input.AlertAfterFailures != nil {
		mon.AlertAfterFailures = *input.AlertAfterFailures
	}
	if input.IsPaused != nil {
		mon.IsPaused = *input.IsPaused
	}
	if input.IsPublic != nil {
		mon.IsPublic = *input.IsPublic
	}

	if err := domain.ValidateMonitorInput(mon.Name, mon.IntervalSeconds, mon.AlertAfterFailures); err != nil {
		return nil, err
	}

	if err := s.monitors.Update(ctx, mon); err != nil {
		return nil, fmt.Errorf("update monitor: %w", err)
	}

	if err := s.publishMonitorEvent(ctx, domain.ActionUpdate, mon); err != nil {
		return mon, fmt.Errorf("%w: %w", ErrEventPublishFailed, err)
	}
	return mon, nil
}

func (s *MonitoringService) DeleteMonitor(ctx context.Context, userID, monitorID uuid.UUID) error {
	if _, err := s.loadOwnedMonitor(ctx, monitorID, userID); err != nil {
		return err
	}
	if err := s.monitors.Delete(ctx, monitorID, userID); err != nil {
		return err
	}
	if s.metrics != nil {
		s.metrics.MonitorDeleted(ctx)
	}
	// Publish after DB delete (per spec 1.1a). If publish fails, checker
	// will eventually sync via periodic monitor reload.
	ev := port.MonitorChangedEvent{Action: domain.ActionDelete, MonitorID: monitorID}
	if err := s.events.PublishMonitorChanged(ctx, ev); err != nil {
		slog.Warn("failed to publish monitor delete event", "monitor_id", monitorID, "error", err)
	}
	return nil
}

func (s *MonitoringService) TogglePause(ctx context.Context, user *domain.User, monitorID uuid.UUID) (*domain.Monitor, error) {
	// Pre-flight ownership check so cross-tenant returns 403, not the
	// same error shape as "not found" (which the DB-level toggle would
	// otherwise collapse to a pgx.ErrNoRows).
	if _, err := s.loadOwnedMonitor(ctx, monitorID, user.ID); err != nil {
		return nil, err
	}

	// Atomic toggle — single SQL query prevents race condition (Issue 4.3).
	mon, err := s.monitors.TogglePause(ctx, monitorID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("toggle pause: %w", err)
	}

	action := domain.ActionResume
	if mon.IsPaused {
		action = domain.ActionPause
	}
	if err := s.publishMonitorEvent(ctx, action, mon); err != nil {
		return mon, fmt.Errorf("%w: %w", ErrEventPublishFailed, err)
	}
	return mon, nil
}

// loadOwnedMonitor fetches a monitor by ID and verifies the caller owns
// it. Maps pgx.ErrNoRows to domain.ErrNotFound and cross-tenant access
// to domain.ErrForbidden — the HTTP boundary translates those into 404
// NOT_FOUND and 403 FORBIDDEN_TENANT respectively.
func (s *MonitoringService) loadOwnedMonitor(ctx context.Context, id, userID uuid.UUID) (*domain.Monitor, error) {
	mon, err := s.monitors.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if mon == nil {
		return nil, domain.ErrNotFound
	}
	if mon.UserID != userID {
		return nil, domain.ErrForbidden
	}
	return mon, nil
}

func (s *MonitoringService) publishMonitorEvent(ctx context.Context, action domain.MonitorAction, m *domain.Monitor) error {
	if s.events == nil {
		return nil
	}
	return s.events.PublishMonitorChanged(ctx, monitorToEvent(action, m))
}

func monitorToEvent(action domain.MonitorAction, m *domain.Monitor) port.MonitorChangedEvent {
	ev := port.MonitorChangedEvent{
		Action:    action,
		MonitorID: m.ID,
	}
	if action == domain.ActionDelete || action == domain.ActionPause {
		return ev
	}
	ev.Name = m.Name
	ev.Type = m.Type
	ev.CheckConfig = m.CheckConfig
	ev.IntervalSeconds = m.IntervalSeconds
	ev.AlertAfterFailures = m.AlertAfterFailures
	ev.IsPaused = m.IsPaused
	return ev
}

// ProcessCheckResult is the core business logic.
func (s *MonitoringService) ProcessCheckResult(ctx context.Context, monitor *domain.Monitor, result *domain.CheckResult) error {
	// Record business metrics
	if s.metrics != nil {
		s.metrics.RecordCheck(ctx,
			string(monitor.Type),
			string(result.Status),
			time.Duration(result.ResponseTimeMs)*time.Millisecond,
		)
	}

	if err := s.checkResults.Insert(ctx, result); err != nil {
		return fmt.Errorf("insert check result: %w", err)
	}

	// Update hourly uptime aggregation
	if err := s.uptime.RecordCheck(ctx, monitor.ID, result.CheckedAt, result.Status == domain.StatusUp); err != nil {
		slog.Error("failed to record uptime check", "monitor_id", monitor.ID, "error", err)
	}

	// Atomically update status and get previous value (Issue 4.1).
	// CTE-based query prevents race condition between concurrent check results.
	previousStatus, err := s.monitors.UpdateStatus(ctx, monitor.ID, result.Status)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// handleDown is called on every down result (not only on the
	// transition) so that multi-check thresholds like
	// alert_after_failures=3 evaluate on each subsequent failure.
	// handleDown is idempotent: cooldown + ErrIncidentExists prevent
	// duplicate incidents for the same monitor.
	if result.Status == domain.StatusDown {
		return s.handleDown(ctx, monitor, result)
	}

	// Recovery fires only on the up transition after a prior down.
	if result.Status == domain.StatusUp && previousStatus == domain.StatusDown {
		return s.handleRecovery(ctx, monitor)
	}

	return nil
}

func (s *MonitoringService) handleDown(ctx context.Context, monitor *domain.Monitor, result *domain.CheckResult) error {
	failures, err := s.checkResults.ConsecutiveFailures(ctx, monitor.ID)
	if err != nil {
		return fmt.Errorf("consecutive failures: %w", err)
	}

	if failures < monitor.AlertAfterFailures {
		return nil
	}

	inCooldown, err := s.incidents.IsInCooldown(ctx, monitor.ID)
	if err != nil {
		return fmt.Errorf("cooldown check: %w", err)
	}
	if inCooldown {
		return nil
	}

	// Skip incident creation if the monitor is currently inside a
	// scheduled maintenance window. We still record the failed check
	// result so uptime math stays honest; we just don't alert and
	// don't open an incident.
	if s.maintenance != nil {
		inMaintenance, mErr := s.maintenance.HasActive(ctx, monitor.ID)
		if mErr != nil {
			slog.Error("maintenance window check failed", "monitor_id", monitor.ID, "error", mErr)
		} else if inMaintenance {
			return nil
		}
	}

	cause := ""
	if result.ErrorMessage != nil {
		cause = *result.ErrorMessage
	}

	incident, err := s.incidents.Create(ctx, port.CreateIncidentInput{
		MonitorID: monitor.ID,
		Cause:     cause,
		State:     domain.IncidentStateInvestigating,
		IsManual:  false,
	})
	if err != nil {
		// Partial unique index caught a concurrent Create — another goroutine
		// already opened the incident. Skip (Issue 4.6).
		if errors.Is(err, domain.ErrIncidentExists) {
			return nil
		}
		return fmt.Errorf("create incident: %w", err)
	}

	if s.metrics != nil {
		s.metrics.IncidentOpened(ctx)
	}

	return s.publishAlert(ctx, monitor, domain.AlertDown, cause, incident.ID)
}

func (s *MonitoringService) handleRecovery(ctx context.Context, monitor *domain.Monitor) error {
	incident, err := s.incidents.GetOpen(ctx, monitor.ID)
	if err != nil {
		return nil
	}

	if err := s.incidents.Resolve(ctx, incident.ID, s.clock.Now()); err != nil {
		return fmt.Errorf("resolve incident: %w", err)
	}

	if s.metrics != nil {
		s.metrics.IncidentResolved(ctx)
	}

	return s.publishAlert(ctx, monitor, domain.AlertUp, "", incident.ID)
}

func (s *MonitoringService) publishAlert(ctx context.Context, monitor *domain.Monitor, eventType domain.AlertEventType, cause string, incidentID int64) error {
	target, err := s.registry.Target(monitor.Type, monitor.CheckConfig)
	if err != nil {
		slog.Error("failed to resolve monitor target", "monitor_id", monitor.ID, "error", err)
	}

	event := &domain.AlertEvent{
		MonitorID:     monitor.ID,
		UserID:        monitor.UserID,
		IncidentID:    incidentID,
		MonitorName:   monitor.Name,
		MonitorTarget: target,
		Event:         eventType,
		Cause:         cause,
	}

	return s.alerts.PublishAlert(ctx, event)
}

// --- Query helpers ---

type MonitorWithUptime struct {
	Monitor domain.Monitor
	Uptime  float64
}

func (s *MonitoringService) ListMonitorsWithUptime(ctx context.Context, userID uuid.UUID) ([]MonitorWithUptime, error) {
	monitors, err := s.monitors.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, len(monitors))
	for i, m := range monitors {
		ids[i] = m.ID
	}

	uptimeMap, err := s.uptime.GetUptimeBatch(ctx, ids, s.clock.Now().Add(-24*time.Hour))
	if err != nil {
		slog.Error("failed to get uptime batch", "error", err)
		uptimeMap = make(map[uuid.UUID]float64)
	}

	result := make([]MonitorWithUptime, 0, len(monitors))
	for _, m := range monitors {
		result = append(result, MonitorWithUptime{Monitor: m, Uptime: uptimeMap[m.ID]})
	}
	return result, nil
}

type MonitorDetail struct {
	Monitor   domain.Monitor
	Uptime24h float64
	Uptime7d  float64
	Uptime30d float64
	Incidents []domain.Incident
	Chart24h  []domain.ChartPoint
}

func (s *MonitoringService) GetMonitorDetail(ctx context.Context, monitorID uuid.UUID) (*MonitorDetail, error) {
	mon, err := s.monitors.GetByID(ctx, monitorID)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	u24, err := s.uptime.GetUptime(ctx, monitorID, now.Add(-24*time.Hour))
	if err != nil {
		slog.Error("failed to get 24h uptime", "monitor_id", monitorID, "error", err)
	}
	u7, err := s.uptime.GetUptime(ctx, monitorID, now.Add(-7*24*time.Hour))
	if err != nil {
		slog.Error("failed to get 7d uptime", "monitor_id", monitorID, "error", err)
	}
	u30, err := s.uptime.GetUptime(ctx, monitorID, now.Add(-30*24*time.Hour))
	if err != nil {
		slog.Error("failed to get 30d uptime", "monitor_id", monitorID, "error", err)
	}

	incidents, err := s.incidents.ListByMonitorID(ctx, monitorID, 10)
	if err != nil {
		slog.Error("failed to list incidents", "monitor_id", monitorID, "error", err)
	}

	chart, err := s.checkResults.GetResponseTimeChart(ctx, monitorID, now.Add(-24*time.Hour))
	if err != nil {
		slog.Error("failed to get response-time chart", "monitor_id", monitorID, "error", err)
	}

	return &MonitorDetail{
		Monitor:   *mon,
		Uptime24h: u24,
		Uptime7d:  u7,
		Uptime30d: u30,
		Incidents: incidents,
		Chart24h:  chart,
	}, nil
}

type StatusMonitor struct {
	ID            uuid.UUID
	Name          string
	CurrentStatus domain.MonitorStatus
	Uptime90d     float64
	// GroupID, if set, pairs with StatusPageData.Groups for UI grouping.
	// nil → renders in the ungrouped default section.
	GroupID *int64
	// InMaintenance is true when an active maintenance_windows row
	// exists for this monitor. UI renders "scheduled maintenance"
	// instead of the raw status.
	InMaintenance bool
}

type StatusPageData struct {
	Slug         string
	AllUp        bool
	ShowBranding bool
	Branding     port.Branding
	Monitors     []StatusMonitor
	Incidents    []domain.Incident
	// Groups is the set of user-defined monitor groups. Monitors inside
	// StatusMonitor are ordered and carry GroupID so the renderer can
	// collapse them into sections.
	Groups []domain.MonitorGroup
}

func (s *MonitoringService) GetStatusPage(ctx context.Context, slug string) (*StatusPageData, error) {
	user, err := s.users.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	monitors, err := s.monitors.ListPublicBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	allUp := true
	statusMons := make([]StatusMonitor, 0, len(monitors))
	var incidents []domain.Incident

	// Pre-fetch group memberships once. Nil on error so the loop falls
	// back to "everything ungrouped" rather than failing the page.
	groupMemberships, gmErr := s.groups.ListMemberships(ctx, user.ID)
	if gmErr != nil {
		slog.Error("failed to load group memberships for status page", "user_id", user.ID, "error", gmErr)
		groupMemberships = nil
	}

	for _, m := range monitors {
		uptime, err := s.uptime.GetUptime(ctx, m.ID, s.clock.Now().Add(-90*24*time.Hour))
		if err != nil {
			slog.Error("failed to get 90d uptime", "monitor_id", m.ID, "error", err)
		}

		// Maintenance-window check. N+1 is acceptable — a status page
		// has ≤50 monitors in practice, and the result isn't cacheable
		// across requests (windows tick with wall-clock).
		inMaintenance := false
		if s.maintenance != nil {
			if active, mErr := s.maintenance.HasActive(ctx, m.ID); mErr != nil {
				slog.Error("maintenance probe failed", "monitor_id", m.ID, "error", mErr)
			} else {
				inMaintenance = active
			}
		}

		// Maintenance suppresses the all_up roll-up — otherwise a
		// scheduled window would flip the page banner red.
		if m.CurrentStatus != domain.StatusUp && !inMaintenance {
			allUp = false
		}

		var gid *int64
		if groupMemberships != nil {
			if id, ok := groupMemberships[m.ID]; ok {
				gid = &id
			}
		}

		statusMons = append(statusMons, StatusMonitor{
			ID:            m.ID,
			Name:          m.Name,
			CurrentStatus: m.CurrentStatus,
			Uptime90d:     uptime,
			GroupID:       gid,
			InMaintenance: inMaintenance,
		})

		monIncidents, err := s.incidents.ListByMonitorID(ctx, m.ID, 5)
		if err != nil {
			slog.Error("failed to list incidents for status page", "monitor_id", m.ID, "error", err)
		}
		incidents = append(incidents, monIncidents...)
	}

	// Branding is Pro-only: fetch but only include it in the response
	// when the owner is on Pro. Free tier keeps the PingCast watermark
	// (ShowBranding=true) and ignores any stored logo/accent/footer.
	var branding port.Branding
	if user.Plan == domain.PlanPro {
		if b, bErr := s.users.GetBranding(ctx, user.ID); bErr != nil {
			slog.Error("failed to load user branding", "user_id", user.ID, "error", bErr)
		} else {
			branding = b
		}
	}

	// Groups are Pro-only on the write side, but we surface them on the
	// public read regardless — free users just won't have any to list.
	groups, gErr := s.groups.ListByUserID(ctx, user.ID)
	if gErr != nil {
		slog.Error("failed to load monitor groups for status page", "user_id", user.ID, "error", gErr)
		groups = nil
	}

	return &StatusPageData{
		Slug:         slug,
		AllUp:        allUp,
		ShowBranding: user.Plan == domain.PlanFree,
		Branding:     branding,
		Monitors:     statusMons,
		Incidents:    incidents,
		Groups:       groups,
	}, nil
}
