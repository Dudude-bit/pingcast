package httpadapter

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Server implements apigen.ServerInterface using app services.
type Server struct {
	auth        *app.AuthService
	monitoring  *app.MonitoringService
	alerts      *app.AlertService
	events      port.MonitorEventPublisher
	rateLimiter port.RateLimiter
}

func NewServer(
	auth *app.AuthService,
	monitoring *app.MonitoringService,
	alerts *app.AlertService,
	events port.MonitorEventPublisher,
	rateLimiter port.RateLimiter,
) *Server {
	return &Server{
		auth:        auth,
		monitoring:  monitoring,
		alerts:      alerts,
		events:      events,
		rateLimiter: rateLimiter,
	}
}

// Compile-time check
var _ apigen.ServerInterface = (*Server)(nil)

func (s *Server) Register(c *fiber.Ctx) error {
	var req apigen.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid request body")})
	}

	user, sessionID, err := s.auth.Register(c.UserContext(), string(req.Email), req.Slug, req.Password)
	if err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
	}

	setSessionCookie(c, sessionID)

	return c.JSON(apigen.AuthResponse{
		User:      domainUserToAPI(user),
		SessionId: &sessionID,
	})
}

func (s *Server) Login(c *fiber.Ctx) error {
	var req apigen.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid request body")})
	}

	allowed, err := s.rateLimiter.Allow(c.UserContext(), string(req.Email))
	if err != nil {
		return c.Status(503).JSON(apigen.ErrorResponse{Error: new("service temporarily unavailable")})
	}
	if !allowed {
		return c.Status(429).JSON(apigen.ErrorResponse{Error: new("too many login attempts")})
	}

	user, sessionID, err := s.auth.Login(c.UserContext(), string(req.Email), req.Password)
	if err != nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("invalid email or password")})
	}

	_ = s.rateLimiter.Reset(c.UserContext(), string(req.Email))
	setSessionCookie(c, sessionID)

	return c.JSON(apigen.AuthResponse{
		User:      domainUserToAPI(user),
		SessionId: &sessionID,
	})
}

func (s *Server) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		s.auth.Logout(c.UserContext(), sessionID)
	}
	c.ClearCookie("session_id")
	return c.SendStatus(200)
}

func (s *Server) ListMonitors(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}

	rows, err := s.monitoring.ListMonitorsWithUptime(c.UserContext(), user.ID)
	if err != nil {
		return c.Status(500).JSON(apigen.ErrorResponse{Error: new("failed to list monitors")})
	}

	result := make([]apigen.MonitorWithUptime, 0, len(rows))
	for _, r := range rows {
		uptimeF := float32(r.Uptime)
		result = append(result, s.domainMonitorToAPIWithUptime(&r.Monitor, &uptimeF))
	}

	return c.JSON(result)
}

func (s *Server) CreateMonitor(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}

	var req apigen.CreateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid request body")})
	}

	checkConfigJSON, err := json.Marshal(req.CheckConfig)
	if err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid check_config")})
	}

	input := app.CreateMonitorInput{
		Name:        req.Name,
		Type:        domain.MonitorType(req.Type),
		CheckConfig: checkConfigJSON,
	}
	if req.IntervalSeconds != nil {
		input.IntervalSeconds = int(*req.IntervalSeconds)
	}
	if req.AlertAfterFailures != nil {
		input.AlertAfterFailures = *req.AlertAfterFailures
	}
	if req.IsPublic != nil {
		input.IsPublic = *req.IsPublic
	}

	mon, err := s.monitoring.CreateMonitor(c.UserContext(), user, input)
	if err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
	}

	if s.events != nil {
		s.events.PublishMonitorChanged(c.UserContext(), domain.ActionCreate, mon.ID, mon)
	}

	return c.Status(201).JSON(s.domainMonitorToAPI(mon))
}

// ListMonitorTypes returns available monitor types and their config schemas.
func (s *Server) ListMonitorTypes(c *fiber.Ctx) error {
	types := s.monitoring.Registry().Types()
	return c.JSON(types)
}

func (s *Server) GetMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}

	detail, err := s.monitoring.GetMonitorDetail(c.UserContext(), uuid.UUID(id))
	if err != nil || detail.Monitor.UserID != user.ID {
		return c.Status(404).JSON(apigen.ErrorResponse{Error: new("not found")})
	}

	apiIncidents := make([]apigen.Incident, 0, len(detail.Incidents))
	for _, inc := range detail.Incidents {
		apiIncidents = append(apiIncidents, domainIncidentToAPI(&inc))
	}

	return c.JSON(s.domainMonitorToAPIDetail(
		&detail.Monitor,
		detail.Uptime24h, detail.Uptime7d, detail.Uptime30d,
		nil, apiIncidents,
	))
}

func (s *Server) UpdateMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}

	var req apigen.UpdateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid request body")})
	}

	input := app.UpdateMonitorInput{
		Name:               req.Name,
		IntervalSeconds:    req.IntervalSeconds,
		AlertAfterFailures: req.AlertAfterFailures,
		IsPaused:           req.IsPaused,
		IsPublic:           req.IsPublic,
	}
	if req.CheckConfig != nil {
		configJSON, err := json.Marshal(*req.CheckConfig)
		if err != nil {
			return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid check_config")})
		}
		input.CheckConfig = configJSON
	}

	updated, err := s.monitoring.UpdateMonitor(c.UserContext(), user, uuid.UUID(id), input)
	if err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
	}

	if s.events != nil {
		s.events.PublishMonitorChanged(c.UserContext(), domain.ActionUpdate, updated.ID, updated)
	}

	return c.JSON(s.domainMonitorToAPI(updated))
}

func (s *Server) DeleteMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}

	if s.events != nil {
		s.events.PublishMonitorChanged(c.UserContext(), domain.ActionDelete, uuid.UUID(id), nil)
	}

	err := s.monitoring.DeleteMonitor(c.UserContext(), user.ID, uuid.UUID(id))
	if err != nil {
		return c.Status(500).JSON(apigen.ErrorResponse{Error: new("failed to delete monitor")})
	}

	return c.SendStatus(204)
}

func (s *Server) ToggleMonitorPause(c *fiber.Ctx, id openapi_types.UUID) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}

	updated, err := s.monitoring.TogglePause(c.UserContext(), user, uuid.UUID(id))
	if err != nil {
		return c.Status(404).JSON(apigen.ErrorResponse{Error: new("not found")})
	}

	if s.events != nil {
		if updated.IsPaused {
			s.events.PublishMonitorChanged(c.UserContext(), domain.ActionPause, updated.ID, nil)
		} else {
			s.events.PublishMonitorChanged(c.UserContext(), domain.ActionResume, updated.ID, updated)
		}
	}

	return c.JSON(s.domainMonitorToAPI(updated))
}

func (s *Server) GetStatusPage(c *fiber.Ctx, slug string) error {
	data, err := s.monitoring.GetStatusPage(c.UserContext(), slug)
	if err != nil {
		return c.Status(404).JSON(apigen.ErrorResponse{Error: new("not found")})
	}

	statusMonitors := make([]apigen.StatusMonitor, 0, len(data.Monitors))
	for _, m := range data.Monitors {
		uptimeF := float32(m.Uptime90d)
		status := string(m.CurrentStatus)
		statusMonitors = append(statusMonitors, apigen.StatusMonitor{
			Name:          &m.Name,
			CurrentStatus: &status,
			Uptime90d:     &uptimeF,
		})
	}

	apiIncidents := make([]apigen.Incident, 0, len(data.Incidents))
	for _, inc := range data.Incidents {
		apiIncidents = append(apiIncidents, domainIncidentToAPI(&inc))
	}

	return c.JSON(apigen.StatusPageResponse{
		Slug:         &data.Slug,
		AllUp:        &data.AllUp,
		ShowBranding: &data.ShowBranding,
		Monitors:     &statusMonitors,
		Incidents:    &apiIncidents,
	})
}

func (s *Server) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(apigen.HealthResponse{Status: new("ok")})
}

// --- helpers ---

func setSessionCookie(c *fiber.Ctx, sessionID string) {
	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

func domainUserToAPI(u *domain.User) *apigen.User {
	plan := apigen.UserPlan(u.Plan)
	return &apigen.User{
		Id:        (*openapi_types.UUID)(&u.ID),
		Email:     &u.Email,
		Slug:      &u.Slug,
		Plan:      &plan,
		CreatedAt: &u.CreatedAt,
	}
}

func (s *Server) domainMonitorToAPI(m *domain.Monitor) apigen.Monitor {
	status := apigen.MonitorCurrentStatus(m.CurrentStatus)
	intervalSeconds := m.IntervalSeconds
	alertAfter := m.AlertAfterFailures
	target, _ := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	monType := string(m.Type)
	checkConfig, err := m.ParseCheckConfig()
	if err != nil {
		slog.Error("failed to parse check config", "monitor_id", m.ID, "error", err)
	}
	return apigen.Monitor{
		Id:                 (*openapi_types.UUID)(&m.ID),
		Name:               &m.Name,
		Type:               &monType,
		CheckConfig:        &checkConfig,
		Target:             &target,
		IntervalSeconds:    &intervalSeconds,
		AlertAfterFailures: &alertAfter,
		IsPaused:           &m.IsPaused,
		IsPublic:           &m.IsPublic,
		CurrentStatus:      &status,
		CreatedAt:          &m.CreatedAt,
	}
}

func (s *Server) domainMonitorToAPIWithUptime(m *domain.Monitor, uptime *float32) apigen.MonitorWithUptime {
	status := apigen.MonitorWithUptimeCurrentStatus(m.CurrentStatus)
	intervalSeconds := m.IntervalSeconds
	alertAfter := m.AlertAfterFailures
	target, _ := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	monType := string(m.Type)
	checkConfig, err := m.ParseCheckConfig()
	if err != nil {
		slog.Error("failed to parse check config", "monitor_id", m.ID, "error", err)
	}
	return apigen.MonitorWithUptime{
		Id:                 (*openapi_types.UUID)(&m.ID),
		Name:               &m.Name,
		Type:               &monType,
		CheckConfig:        &checkConfig,
		Target:             &target,
		IntervalSeconds:    &intervalSeconds,
		AlertAfterFailures: &alertAfter,
		IsPaused:           &m.IsPaused,
		IsPublic:           &m.IsPublic,
		CurrentStatus:      &status,
		CreatedAt:          &m.CreatedAt,
		Uptime24h:          uptime,
	}
}

func (s *Server) domainMonitorToAPIDetail(m *domain.Monitor, u24h, u7d, u30d float64, chartData []apigen.ChartPoint, incidents []apigen.Incident) apigen.MonitorDetail {
	status := apigen.MonitorDetailCurrentStatus(m.CurrentStatus)
	intervalSeconds := m.IntervalSeconds
	alertAfter := m.AlertAfterFailures
	target, _ := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	monType := string(m.Type)
	checkConfig, err := m.ParseCheckConfig()
	if err != nil {
		slog.Error("failed to parse check config", "monitor_id", m.ID, "error", err)
	}
	u24 := float32(u24h)
	u7 := float32(u7d)
	u30 := float32(u30d)
	return apigen.MonitorDetail{
		Id:                 (*openapi_types.UUID)(&m.ID),
		Name:               &m.Name,
		Type:               &monType,
		CheckConfig:        &checkConfig,
		Target:             &target,
		IntervalSeconds:    &intervalSeconds,
		AlertAfterFailures: &alertAfter,
		IsPaused:           &m.IsPaused,
		IsPublic:           &m.IsPublic,
		CurrentStatus:      &status,
		CreatedAt:          &m.CreatedAt,
		Uptime24h:          &u24,
		Uptime7d:           &u7,
		Uptime30d:          &u30,
		ChartData:          &chartData,
		Incidents:          &incidents,
	}
}

func domainIncidentToAPI(i *domain.Incident) apigen.Incident {
	return apigen.Incident{
		Id:         &i.ID,
		MonitorId:  (*openapi_types.UUID)(&i.MonitorID),
		StartedAt:  &i.StartedAt,
		ResolvedAt: i.ResolvedAt,
		Cause:      &i.Cause,
	}
}

// --- Channel API handlers ---

func (s *Server) ListChannelTypes(c *fiber.Ctx) error {
	types := s.alerts.Registry().Types()
	result := make([]apigen.ChannelTypeInfo, 0, len(types))
	for _, t := range types {
		typ := string(t.Type)
		fields := make([]apigen.ConfigField, 0, len(t.Schema.Fields))
		for _, f := range t.Schema.Fields {
			fields = append(fields, apigen.ConfigField{
				Name: &f.Name, Label: &f.Label, Type: &f.Type,
				Required: &f.Required, Placeholder: &f.Placeholder,
			})
		}
		schema := apigen.ConfigSchema{Fields: &fields}
		result = append(result, apigen.ChannelTypeInfo{
			Type: &typ, Label: &t.Label, Schema: &schema,
		})
	}
	return c.JSON(result)
}

func (s *Server) ListChannels(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}
	channels, err := s.alerts.ListChannels(c.UserContext(), user.ID)
	if err != nil {
		return c.Status(500).JSON(apigen.ErrorResponse{Error: new("failed to list channels")})
	}
	result := make([]apigen.NotificationChannel, 0, len(channels))
	for _, ch := range channels {
		result = append(result, domainChannelToAPI(&ch))
	}
	return c.JSON(result)
}

func (s *Server) CreateChannel(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}
	var req apigen.CreateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid request body")})
	}
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid config")})
	}
	ch, err := s.alerts.CreateChannel(c.UserContext(), user.ID, app.CreateChannelInput{
		Name:   req.Name,
		Type:   domain.ChannelType(req.Type),
		Config: configJSON,
	})
	if err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
	}
	return c.Status(201).JSON(domainChannelToAPI(ch))
}

func (s *Server) UpdateChannel(c *fiber.Ctx, id openapi_types.UUID) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}
	var req apigen.UpdateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid request body")})
	}
	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}
	var configJSON json.RawMessage
	if req.Config != nil {
		configJSON, _ = json.Marshal(*req.Config)
	}
	ch, err := s.alerts.UpdateChannel(c.UserContext(), user.ID, uuid.UUID(id), name, configJSON, isEnabled)
	if err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
	}
	return c.JSON(domainChannelToAPI(ch))
}

func (s *Server) DeleteChannel(c *fiber.Ctx, id openapi_types.UUID) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}
	if err := s.alerts.DeleteChannel(c.UserContext(), user.ID, uuid.UUID(id)); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
	}
	return c.SendStatus(204)
}

func (s *Server) BindChannel(c *fiber.Ctx, id openapi_types.UUID) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}
	var req struct {
		ChannelID uuid.UUID `json:"channel_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid request body")})
	}
	if err := s.alerts.BindChannel(c.UserContext(), user.ID, uuid.UUID(id), req.ChannelID); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
	}
	return c.SendStatus(200)
}

func (s *Server) UnbindChannel(c *fiber.Ctx, id openapi_types.UUID, channelId openapi_types.UUID) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
	}
	if err := s.alerts.UnbindChannel(c.UserContext(), user.ID, uuid.UUID(id), uuid.UUID(channelId)); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
	}
	return c.SendStatus(204)
}

func domainChannelToAPI(ch *domain.NotificationChannel) apigen.NotificationChannel {
	typ := string(ch.Type)
	config, err := ch.ParseConfig()
	if err != nil {
		slog.Error("failed to parse channel config", "channel_id", ch.ID, "error", err)
	}
	return apigen.NotificationChannel{
		Id:        (*openapi_types.UUID)(&ch.ID),
		Name:      &ch.Name,
		Type:      &typ,
		Config:    &config,
		IsEnabled: &ch.IsEnabled,
		CreatedAt: &ch.CreatedAt,
	}
}
