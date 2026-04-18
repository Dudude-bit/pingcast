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
	monitors     port.MonitorRepo
	channels     port.ChannelRepo
	checkResults port.CheckResultRepo
	incidents    port.IncidentRepo
	users        port.UserRepo
	uptime       port.UptimeRepo
	txm          port.TxManager
	alerts       port.AlertEventPublisher
	events       port.MonitorEventPublisher
	registry     port.CheckerRegistry
	metrics      port.Metrics
	clock        port.Clock
}

func NewMonitoringService(
	monitors port.MonitorRepo,
	channels port.ChannelRepo,
	checkResults port.CheckResultRepo,
	incidents port.IncidentRepo,
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
		monitors:     monitors,
		channels:     channels,
		checkResults: checkResults,
		incidents:    incidents,
		users:        users,
		uptime:       uptime,
		txm:          txm,
		alerts:       alerts,
		events:       events,
		registry:     registry,
		metrics:      metrics,
		clock:        clock,
	}
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
		return nil, fmt.Errorf("monitor limit reached")
	}

	interval := max(input.IntervalSeconds, user.MinInterval())
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
	mon, err := s.monitors.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("monitor not found: %w", err)
	}
	if mon.UserID != user.ID {
		return nil, fmt.Errorf("monitor not found")
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

	if previousStatus == result.Status {
		return nil
	}

	if result.Status == domain.StatusDown {
		return s.handleDown(ctx, monitor, result)
	}

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

	cause := ""
	if result.ErrorMessage != nil {
		cause = *result.ErrorMessage
	}

	incident, err := s.incidents.Create(ctx, monitor.ID, cause)
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
	Name          string
	CurrentStatus domain.MonitorStatus
	Uptime90d     float64
}

type StatusPageData struct {
	Slug         string
	AllUp        bool
	ShowBranding bool
	Monitors     []StatusMonitor
	Incidents    []domain.Incident
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

	for _, m := range monitors {
		uptime, err := s.uptime.GetUptime(ctx, m.ID, s.clock.Now().Add(-90*24*time.Hour))
		if err != nil {
			slog.Error("failed to get 90d uptime", "monitor_id", m.ID, "error", err)
		}
		if m.CurrentStatus != domain.StatusUp {
			allUp = false
		}
		statusMons = append(statusMons, StatusMonitor{
			Name:          m.Name,
			CurrentStatus: m.CurrentStatus,
			Uptime90d:     uptime,
		})

		monIncidents, err := s.incidents.ListByMonitorID(ctx, m.ID, 5)
		if err != nil {
			slog.Error("failed to list incidents for status page", "monitor_id", m.ID, "error", err)
		}
		incidents = append(incidents, monIncidents...)
	}

	return &StatusPageData{
		Slug:         slug,
		AllUp:        allUp,
		ShowBranding: user.Plan == domain.PlanFree,
		Monitors:     statusMons,
		Incidents:    incidents,
	}, nil
}
