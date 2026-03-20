package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

type MonitoringService struct {
	monitors     port.MonitorRepo
	checkResults port.CheckResultRepo
	incidents    port.IncidentRepo
	users        port.UserRepo
	alerts       port.AlertEventPublisher
	checker      port.MonitorChecker
}

func NewMonitoringService(
	monitors port.MonitorRepo,
	checkResults port.CheckResultRepo,
	incidents port.IncidentRepo,
	users port.UserRepo,
	alerts port.AlertEventPublisher,
	checker port.MonitorChecker,
) *MonitoringService {
	return &MonitoringService{
		monitors:     monitors,
		checkResults: checkResults,
		incidents:    incidents,
		users:        users,
		alerts:       alerts,
		checker:      checker,
	}
}

// RunCheck executes a health check via the injected MonitorChecker port
// and processes the result (status transitions, incidents, alerts).
func (s *MonitoringService) RunCheck(ctx context.Context, monitor *domain.Monitor) error {
	result := s.checker.Check(ctx, monitor)
	return s.ProcessCheckResult(ctx, monitor, result)
}

type CreateMonitorInput struct {
	Name               string
	URL                string
	Method             domain.HTTPMethod
	IntervalSeconds    int
	ExpectedStatus     int
	Keyword            *string
	AlertAfterFailures int
	IsPublic           bool
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

	alertAfter := max(input.AlertAfterFailures, 1)
	if input.AlertAfterFailures == 0 {
		alertAfter = 3
	}

	method := input.Method
	if method == "" {
		method = domain.MethodGET
	}

	expectedStatus := input.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	mon := &domain.Monitor{
		UserID:             user.ID,
		Name:               input.Name,
		URL:                input.URL,
		Method:             method,
		IntervalSeconds:    interval,
		ExpectedStatus:     expectedStatus,
		Keyword:            input.Keyword,
		AlertAfterFailures: alertAfter,
		IsPublic:           input.IsPublic,
		CurrentStatus:      domain.StatusUnknown,
	}

	return s.monitors.Create(ctx, mon)
}

type UpdateMonitorInput struct {
	Name               *string
	URL                *string
	Method             *domain.HTTPMethod
	IntervalSeconds    *int
	ExpectedStatus     *int
	Keyword            *string
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
		mon.Name = *input.Name
	}
	if input.URL != nil {
		mon.URL = *input.URL
	}
	if input.Method != nil {
		mon.Method = *input.Method
	}
	if input.IntervalSeconds != nil {
		mon.IntervalSeconds = *input.IntervalSeconds
	}
	if input.ExpectedStatus != nil {
		mon.ExpectedStatus = *input.ExpectedStatus
	}
	if input.Keyword != nil {
		mon.Keyword = input.Keyword
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

	mon.IntervalSeconds = max(mon.IntervalSeconds, user.MinInterval())

	if err := s.monitors.Update(ctx, mon); err != nil {
		return nil, fmt.Errorf("update monitor: %w", err)
	}

	return mon, nil
}

func (s *MonitoringService) DeleteMonitor(ctx context.Context, userID, monitorID uuid.UUID) error {
	return s.monitors.Delete(ctx, monitorID, userID)
}

func (s *MonitoringService) TogglePause(ctx context.Context, user *domain.User, monitorID uuid.UUID) (*domain.Monitor, error) {
	mon, err := s.monitors.GetByID(ctx, monitorID)
	if err != nil || mon.UserID != user.ID {
		return nil, fmt.Errorf("monitor not found")
	}

	mon.IsPaused = !mon.IsPaused
	if err := s.monitors.Update(ctx, mon); err != nil {
		return nil, fmt.Errorf("update monitor: %w", err)
	}

	return mon, nil
}

// ProcessCheckResult is the core business logic.
func (s *MonitoringService) ProcessCheckResult(ctx context.Context, monitor *domain.Monitor, result *domain.CheckResult) error {
	if err := s.checkResults.Insert(ctx, result); err != nil {
		return fmt.Errorf("insert check result: %w", err)
	}

	current, err := s.monitors.GetByID(ctx, monitor.ID)
	previousStatus := domain.StatusUnknown
	if err == nil {
		previousStatus = current.CurrentStatus
	}

	if err := s.monitors.UpdateStatus(ctx, monitor.ID, result.Status); err != nil {
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
		return fmt.Errorf("create incident: %w", err)
	}

	return s.publishAlert(ctx, monitor, domain.AlertDown, cause, incident.ID)
}

func (s *MonitoringService) handleRecovery(ctx context.Context, monitor *domain.Monitor) error {
	incident, err := s.incidents.GetOpen(ctx, monitor.ID)
	if err != nil {
		return nil
	}

	if err := s.incidents.Resolve(ctx, incident.ID, time.Now()); err != nil {
		return fmt.Errorf("resolve incident: %w", err)
	}

	return s.publishAlert(ctx, monitor, domain.AlertUp, "", incident.ID)
}

func (s *MonitoringService) publishAlert(ctx context.Context, monitor *domain.Monitor, eventType domain.AlertEventType, cause string, incidentID int64) error {
	user, err := s.users.GetByID(ctx, monitor.UserID)
	if err != nil {
		return fmt.Errorf("get user for alert: %w", err)
	}

	event := &domain.AlertEvent{
		MonitorID:   monitor.ID,
		IncidentID:  incidentID,
		MonitorName: monitor.Name,
		MonitorURL:  monitor.URL,
		Event:       eventType,
		Cause:       cause,
		TgChatID:    user.TgChatID,
		Email:       user.Email,
		Plan:        user.Plan,
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

	result := make([]MonitorWithUptime, 0, len(monitors))
	for _, m := range monitors {
		uptime, _ := s.checkResults.GetUptime(ctx, m.ID, time.Now().Add(-24*time.Hour))
		result = append(result, MonitorWithUptime{Monitor: m, Uptime: uptime})
	}
	return result, nil
}

type MonitorDetail struct {
	Monitor   domain.Monitor
	Uptime24h float64
	Uptime7d  float64
	Uptime30d float64
	Incidents []domain.Incident
}

func (s *MonitoringService) GetMonitorDetail(ctx context.Context, monitorID uuid.UUID) (*MonitorDetail, error) {
	mon, err := s.monitors.GetByID(ctx, monitorID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	u24, _ := s.checkResults.GetUptime(ctx, monitorID, now.Add(-24*time.Hour))
	u7, _ := s.checkResults.GetUptime(ctx, monitorID, now.Add(-7*24*time.Hour))
	u30, _ := s.checkResults.GetUptime(ctx, monitorID, now.Add(-30*24*time.Hour))

	incidents, _ := s.incidents.ListByMonitorID(ctx, monitorID, 10)

	return &MonitorDetail{
		Monitor:   *mon,
		Uptime24h: u24,
		Uptime7d:  u7,
		Uptime30d: u30,
		Incidents: incidents,
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
		uptime, _ := s.checkResults.GetUptime(ctx, m.ID, time.Now().Add(-90*24*time.Hour))
		if m.CurrentStatus != domain.StatusUp {
			allUp = false
		}
		statusMons = append(statusMons, StatusMonitor{
			Name:          m.Name,
			CurrentStatus: m.CurrentStatus,
			Uptime90d:     uptime,
		})

		monIncidents, _ := s.incidents.ListByMonitorID(ctx, m.ID, 5)
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
