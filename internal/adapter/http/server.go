package httpadapter

import (
	"encoding/json"
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
	events      port.MonitorEventPublisher
	rateLimiter *RateLimiter
}

func NewServer(
	auth *app.AuthService,
	monitoring *app.MonitoringService,
	events port.MonitorEventPublisher,
	rateLimiter *RateLimiter,
) *Server {
	return &Server{
		auth:        auth,
		monitoring:  monitoring,
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

	if !s.rateLimiter.Allow(string(req.Email)) {
		return c.Status(429).JSON(apigen.ErrorResponse{Error: new("too many login attempts")})
	}

	user, sessionID, err := s.auth.Login(c.UserContext(), string(req.Email), req.Password)
	if err != nil {
		s.rateLimiter.Record(string(req.Email))
		return c.Status(401).JSON(apigen.ErrorResponse{Error: new("invalid email or password")})
	}

	s.rateLimiter.Reset(string(req.Email))
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

	// Build check config from legacy API fields for backward compatibility.
	monType := domain.MonitorHTTP
	var methodStr *string
	if req.Method != nil {
		ms := string(*req.Method)
		methodStr = &ms
	}
	checkConfig := buildHTTPCheckConfig(req.Url, methodStr, req.ExpectedStatus, req.Keyword)

	input := app.CreateMonitorInput{
		Name:        req.Name,
		Type:        monType,
		CheckConfig: checkConfig,
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
		AlertAfterFailures: req.AlertAfterFailures,
		IsPaused:           req.IsPaused,
		IsPublic:           req.IsPublic,
	}
	if req.IntervalSeconds != nil {
		v := int(*req.IntervalSeconds)
		input.IntervalSeconds = &v
	}
	// Build check config from legacy fields if URL is provided.
	if req.Url != nil {
		var mStr *string
		if req.Method != nil {
			ms := string(*req.Method)
			mStr = &ms
		}
		input.CheckConfig = buildHTTPCheckConfig(*req.Url, mStr, req.ExpectedStatus, req.Keyword)
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
	tgLinked := u.TgChatID != nil
	plan := apigen.UserPlan(u.Plan)
	return &apigen.User{
		Id:        (*openapi_types.UUID)(&u.ID),
		Email:     &u.Email,
		Slug:      &u.Slug,
		Plan:      &plan,
		TgLinked:  &tgLinked,
		CreatedAt: &u.CreatedAt,
	}
}

func (s *Server) domainMonitorToAPI(m *domain.Monitor) apigen.Monitor {
	status := apigen.MonitorCurrentStatus(m.CurrentStatus)
	intervalSeconds := m.IntervalSeconds
	alertAfter := m.AlertAfterFailures
	target := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	return apigen.Monitor{
		Id:                 (*openapi_types.UUID)(&m.ID),
		Name:               &m.Name,
		Url:                &target,
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
	target := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	return apigen.MonitorWithUptime{
		Id:                 (*openapi_types.UUID)(&m.ID),
		Name:               &m.Name,
		Url:                &target,
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
	target := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	u24 := float32(u24h)
	u7 := float32(u7d)
	u30 := float32(u30d)
	return apigen.MonitorDetail{
		Id:                 (*openapi_types.UUID)(&m.ID),
		Name:               &m.Name,
		Url:                &target,
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

// buildHTTPCheckConfig constructs a JSON check_config for HTTP monitors from legacy API fields.
func buildHTTPCheckConfig(url string, method *string, expectedStatus *int, keyword *string) json.RawMessage {
	cfg := map[string]any{"url": url}
	if method != nil {
		cfg["method"] = *method
	} else {
		cfg["method"] = "GET"
	}
	if expectedStatus != nil {
		cfg["expected_status"] = *expectedStatus
	} else {
		cfg["expected_status"] = 200
	}
	if keyword != nil {
		cfg["keyword"] = *keyword
	}
	data, _ := json.Marshal(cfg)
	return data
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
