package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/auth"
	natsbus "github.com/kirillinakin/pingcast/internal/nats"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Server implements apigen.ServerInterface.
type Server struct {
	queries     *gen.Queries
	authService *auth.Service
	rateLimiter *auth.RateLimiter
	onChanged func(action string, monitorID uuid.UUID, monitor *natsbus.MonitorData)
}

func NewServer(
	queries *gen.Queries,
	authService *auth.Service,
	rateLimiter *auth.RateLimiter,
	onChanged func(action string, monitorID uuid.UUID, monitor *natsbus.MonitorData),
) *Server {
	return &Server{
		queries:     queries,
		authService: authService,
		rateLimiter: rateLimiter,
		onChanged:   onChanged,
	}
}

// Compile-time check
var _ apigen.ServerInterface = (*Server)(nil)

func (s *Server) Register(c *fiber.Ctx) error {
	var req apigen.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: ptr("invalid request body")})
	}

	user, session, err := s.authService.Register(c.UserContext(), string(req.Email), req.Slug, req.Password)
	if err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: ptr(err.Error())})
	}

	setSessionCookie(c, session.ID)

	tgLinked := user.TgChatID.Valid
	return c.JSON(apigen.AuthResponse{
		User: &apigen.User{
			Id:        (*openapi_types.UUID)(&user.ID),
			Email:     &user.Email,
			Slug:      &user.Slug,
			Plan:      (*apigen.UserPlan)(&user.Plan),
			TgLinked:  &tgLinked,
			CreatedAt: &user.CreatedAt,
		},
		SessionId: &session.ID,
	})
}

func (s *Server) Login(c *fiber.Ctx) error {
	var req apigen.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: ptr("invalid request body")})
	}

	if !s.rateLimiter.Allow(string(req.Email)) {
		return c.Status(429).JSON(apigen.ErrorResponse{Error: ptr("too many login attempts")})
	}

	user, session, err := s.authService.Login(c.UserContext(), string(req.Email), req.Password)
	if err != nil {
		s.rateLimiter.Record(string(req.Email))
		return c.Status(401).JSON(apigen.ErrorResponse{Error: ptr("invalid email or password")})
	}

	s.rateLimiter.Reset(string(req.Email))
	setSessionCookie(c, session.ID)

	tgLinked := user.TgChatID.Valid
	return c.JSON(apigen.AuthResponse{
		User: &apigen.User{
			Id:        (*openapi_types.UUID)(&user.ID),
			Email:     &user.Email,
			Slug:      &user.Slug,
			Plan:      (*apigen.UserPlan)(&user.Plan),
			TgLinked:  &tgLinked,
			CreatedAt: &user.CreatedAt,
		},
		SessionId: &session.ID,
	})
}

func (s *Server) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		s.authService.Logout(c.UserContext(), sessionID)
	}
	c.ClearCookie("session_id")
	return c.SendStatus(200)
}

func (s *Server) ListMonitors(c *fiber.Ctx) error {
	user := auth.UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: ptr("unauthorized")})
	}

	monitors, err := s.queries.ListMonitorsByUserID(c.UserContext(), user.ID)
	if err != nil {
		return c.Status(500).JSON(apigen.ErrorResponse{Error: ptr("failed to list monitors")})
	}

	result := make([]apigen.MonitorWithUptime, 0, len(monitors))
	for _, m := range monitors {
		uptimeRaw, _ := s.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{
			MonitorID: m.ID,
			CheckedAt: time.Now().Add(-24 * time.Hour),
		})
		uptimeF := float32(toFloat64(uptimeRaw))
		result = append(result, toMonitorWithUptime(m, &uptimeF))
	}

	return c.JSON(result)
}

func (s *Server) CreateMonitor(c *fiber.Ctx) error {
	user := auth.UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: ptr("unauthorized")})
	}

	var req apigen.CreateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: ptr("invalid request body")})
	}

	count, _ := s.queries.CountMonitorsByUserID(c.UserContext(), user.ID)
	limit := int32(5)
	if user.Plan == "pro" {
		limit = 50
	}
	if count >= limit {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: ptr("monitor limit reached")})
	}

	minInterval := int32(300)
	if user.Plan == "pro" {
		minInterval = 30
	}

	interval := int32(300)
	if req.IntervalSeconds != nil {
		interval = int32(*req.IntervalSeconds)
	}
	if interval < minInterval {
		interval = minInterval
	}

	method := "GET"
	if req.Method != nil {
		method = string(*req.Method)
	}

	expectedStatus := int32(200)
	if req.ExpectedStatus != nil {
		expectedStatus = int32(*req.ExpectedStatus)
	}

	alertAfter := int32(3)
	if req.AlertAfterFailures != nil {
		alertAfter = int32(*req.AlertAfterFailures)
	}

	isPublic := false
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	var keyword *string
	if req.Keyword != nil {
		keyword = req.Keyword
	}

	mon, err := s.queries.CreateMonitor(c.UserContext(), gen.CreateMonitorParams{
		UserID:             user.ID,
		Name:               req.Name,
		Url:                req.Url,
		Method:             method,
		IntervalSeconds:    interval,
		ExpectedStatus:     expectedStatus,
		Keyword:            keyword,
		AlertAfterFailures: alertAfter,
		IsPaused:           false,
		IsPublic:           isPublic,
	})
	if err != nil {
		return c.Status(500).JSON(apigen.ErrorResponse{Error: ptr("failed to create monitor")})
	}

	if s.onChanged != nil {
		s.onChanged("create", mon.ID, monitorToNats(mon))
	}

	return c.Status(201).JSON(toMonitor(mon))
}

func (s *Server) GetMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := auth.UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: ptr("unauthorized")})
	}

	mon, err := s.queries.GetMonitorByID(c.UserContext(), uuid.UUID(id))
	if err != nil || mon.UserID != user.ID {
		return c.Status(404).JSON(apigen.ErrorResponse{Error: ptr("not found")})
	}

	now := time.Now()
	uptime24hRaw, _ := s.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{MonitorID: mon.ID, CheckedAt: now.Add(-24 * time.Hour)})
	uptime7dRaw, _ := s.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{MonitorID: mon.ID, CheckedAt: now.Add(-7 * 24 * time.Hour)})
	uptime30dRaw, _ := s.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{MonitorID: mon.ID, CheckedAt: now.Add(-30 * 24 * time.Hour)})

	incidents, _ := s.queries.ListIncidentsByMonitorID(c.UserContext(), gen.ListIncidentsByMonitorIDParams{MonitorID: mon.ID, Limit: 10})

	apiIncidents := make([]apigen.Incident, 0, len(incidents))
	for _, inc := range incidents {
		apiIncidents = append(apiIncidents, toIncident(inc))
	}

	detail := toMonitorDetail(mon, toFloat64(uptime24hRaw), toFloat64(uptime7dRaw), toFloat64(uptime30dRaw), nil, apiIncidents)
	return c.JSON(detail)
}

func (s *Server) UpdateMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := auth.UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: ptr("unauthorized")})
	}

	mon, err := s.queries.GetMonitorByID(c.UserContext(), uuid.UUID(id))
	if err != nil || mon.UserID != user.ID {
		return c.Status(404).JSON(apigen.ErrorResponse{Error: ptr("not found")})
	}

	var req apigen.UpdateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apigen.ErrorResponse{Error: ptr("invalid request body")})
	}

	name := mon.Name
	if req.Name != nil {
		name = *req.Name
	}
	url := mon.Url
	if req.Url != nil {
		url = *req.Url
	}
	method := mon.Method
	if req.Method != nil {
		method = string(*req.Method)
	}
	interval := mon.IntervalSeconds
	if req.IntervalSeconds != nil {
		interval = int32(*req.IntervalSeconds)
	}
	expectedStatus := mon.ExpectedStatus
	if req.ExpectedStatus != nil {
		expectedStatus = int32(*req.ExpectedStatus)
	}
	keyword := mon.Keyword
	if req.Keyword != nil {
		keyword = req.Keyword
	}
	alertAfter := mon.AlertAfterFailures
	if req.AlertAfterFailures != nil {
		alertAfter = int32(*req.AlertAfterFailures)
	}
	isPaused := mon.IsPaused
	if req.IsPaused != nil {
		isPaused = *req.IsPaused
	}
	isPublic := mon.IsPublic
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	err = s.queries.UpdateMonitor(c.UserContext(), gen.UpdateMonitorParams{
		ID:                 mon.ID,
		Name:               name,
		Url:                url,
		Method:             method,
		IntervalSeconds:    interval,
		ExpectedStatus:     expectedStatus,
		Keyword:            keyword,
		AlertAfterFailures: alertAfter,
		IsPaused:           isPaused,
		IsPublic:           isPublic,
		UserID:             user.ID,
	})
	if err != nil {
		return c.Status(500).JSON(apigen.ErrorResponse{Error: ptr("failed to update monitor")})
	}

	updated, _ := s.queries.GetMonitorByID(c.UserContext(), mon.ID)

	if s.onChanged != nil {
		s.onChanged("update", updated.ID, monitorToNats(updated))
	}

	return c.JSON(toMonitor(updated))
}

func (s *Server) DeleteMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := auth.UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: ptr("unauthorized")})
	}

	if s.onChanged != nil {
		s.onChanged("delete", uuid.UUID(id), nil)
	}

	err := s.queries.DeleteMonitor(c.UserContext(), gen.DeleteMonitorParams{
		ID:     uuid.UUID(id),
		UserID: user.ID,
	})
	if err != nil {
		return c.Status(500).JSON(apigen.ErrorResponse{Error: ptr("failed to delete monitor")})
	}

	return c.SendStatus(204)
}

func (s *Server) ToggleMonitorPause(c *fiber.Ctx, id openapi_types.UUID) error {
	user := auth.UserFromCtx(c)
	if user == nil {
		return c.Status(401).JSON(apigen.ErrorResponse{Error: ptr("unauthorized")})
	}

	mon, err := s.queries.GetMonitorByID(c.UserContext(), uuid.UUID(id))
	if err != nil || mon.UserID != user.ID {
		return c.Status(404).JSON(apigen.ErrorResponse{Error: ptr("not found")})
	}

	newPaused := !mon.IsPaused
	err = s.queries.UpdateMonitor(c.UserContext(), gen.UpdateMonitorParams{
		ID:                 mon.ID,
		Name:               mon.Name,
		Url:                mon.Url,
		Method:             mon.Method,
		IntervalSeconds:    mon.IntervalSeconds,
		ExpectedStatus:     mon.ExpectedStatus,
		Keyword:            mon.Keyword,
		AlertAfterFailures: mon.AlertAfterFailures,
		IsPaused:           newPaused,
		IsPublic:           mon.IsPublic,
		UserID:             user.ID,
	})
	if err != nil {
		return c.Status(500).JSON(apigen.ErrorResponse{Error: ptr("failed to toggle pause")})
	}

	updated, _ := s.queries.GetMonitorByID(c.UserContext(), mon.ID)

	if s.onChanged != nil {
		if newPaused {
			s.onChanged("pause", mon.ID, nil)
		} else {
			s.onChanged("resume", mon.ID, monitorToNats(updated))
		}
	}

	return c.JSON(toMonitor(updated))
}

func (s *Server) GetStatusPage(c *fiber.Ctx, slug string) error {
	user, err := s.queries.GetUserBySlug(c.UserContext(), slug)
	if err != nil {
		return c.Status(404).JSON(apigen.ErrorResponse{Error: ptr("not found")})
	}

	monitors, _ := s.queries.ListPublicMonitorsByUserSlug(c.UserContext(), slug)

	allUp := true
	statusMonitors := make([]apigen.StatusMonitor, 0, len(monitors))
	var incidents []apigen.Incident

	for _, m := range monitors {
		uptimeRaw, _ := s.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{
			MonitorID: m.ID,
			CheckedAt: time.Now().Add(-90 * 24 * time.Hour),
		})
		if m.CurrentStatus != "up" {
			allUp = false
		}
		uptimeF := float32(toFloat64(uptimeRaw))
		statusMonitors = append(statusMonitors, apigen.StatusMonitor{
			Name:          &m.Name,
			CurrentStatus: &m.CurrentStatus,
			Uptime90d:     &uptimeF,
		})

		dbIncidents, _ := s.queries.ListIncidentsByMonitorID(c.UserContext(), gen.ListIncidentsByMonitorIDParams{
			MonitorID: m.ID,
			Limit:     5,
		})
		for _, inc := range dbIncidents {
			incidents = append(incidents, toIncident(inc))
		}
	}

	showBranding := user.Plan == "free"
	return c.JSON(apigen.StatusPageResponse{
		Slug:         &slug,
		AllUp:        &allUp,
		ShowBranding: &showBranding,
		Monitors:     &statusMonitors,
		Incidents:    &incidents,
	})
}

func (s *Server) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(apigen.HealthResponse{Status: ptr("ok")})
}

// --- helpers ---

func ptr[T any](v T) *T { return &v }

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

func toMonitor(m gen.Monitor) apigen.Monitor {
	status := apigen.MonitorCurrentStatus(m.CurrentStatus)
	method := apigen.MonitorMethod(m.Method)
	intervalSeconds := int(m.IntervalSeconds)
	expectedStatus := int(m.ExpectedStatus)
	alertAfter := int(m.AlertAfterFailures)
	return apigen.Monitor{
		Id:                (*openapi_types.UUID)(&m.ID),
		Name:              &m.Name,
		Url:               &m.Url,
		Method:            &method,
		IntervalSeconds:   &intervalSeconds,
		ExpectedStatus:    &expectedStatus,
		Keyword:           m.Keyword,
		AlertAfterFailures: &alertAfter,
		IsPaused:          &m.IsPaused,
		IsPublic:          &m.IsPublic,
		CurrentStatus:     &status,
		CreatedAt:         &m.CreatedAt,
	}
}

func toMonitorWithUptime(m gen.Monitor, uptime *float32) apigen.MonitorWithUptime {
	status := apigen.MonitorWithUptimeCurrentStatus(m.CurrentStatus)
	method := apigen.MonitorWithUptimeMethod(m.Method)
	intervalSeconds := int(m.IntervalSeconds)
	expectedStatus := int(m.ExpectedStatus)
	alertAfter := int(m.AlertAfterFailures)
	return apigen.MonitorWithUptime{
		Id:                 (*openapi_types.UUID)(&m.ID),
		Name:               &m.Name,
		Url:                &m.Url,
		Method:             &method,
		IntervalSeconds:    &intervalSeconds,
		ExpectedStatus:     &expectedStatus,
		Keyword:            m.Keyword,
		AlertAfterFailures: &alertAfter,
		IsPaused:           &m.IsPaused,
		IsPublic:           &m.IsPublic,
		CurrentStatus:      &status,
		CreatedAt:          &m.CreatedAt,
		Uptime24h:          uptime,
	}
}

func toMonitorDetail(m gen.Monitor, u24h, u7d, u30d float64, chartData []apigen.ChartPoint, incidents []apigen.Incident) apigen.MonitorDetail {
	status := apigen.MonitorDetailCurrentStatus(m.CurrentStatus)
	method := apigen.MonitorDetailMethod(m.Method)
	intervalSeconds := int(m.IntervalSeconds)
	expectedStatus := int(m.ExpectedStatus)
	alertAfter := int(m.AlertAfterFailures)
	u24 := float32(u24h)
	u7 := float32(u7d)
	u30 := float32(u30d)
	return apigen.MonitorDetail{
		Id:                 (*openapi_types.UUID)(&m.ID),
		Name:               &m.Name,
		Url:                &m.Url,
		Method:             &method,
		IntervalSeconds:    &intervalSeconds,
		ExpectedStatus:     &expectedStatus,
		Keyword:            m.Keyword,
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

func toIncident(i gen.Incident) apigen.Incident {
	id := int64(i.ID)
	var resolvedAt *time.Time
	if i.ResolvedAt.Valid {
		resolvedAt = &i.ResolvedAt.Time
	}
	return apigen.Incident{
		Id:         &id,
		MonitorId:  (*openapi_types.UUID)(&i.MonitorID),
		StartedAt:  &i.StartedAt,
		ResolvedAt: resolvedAt,
		Cause:      &i.Cause,
	}
}

func monitorToNats(m gen.Monitor) *natsbus.MonitorData {
	return &natsbus.MonitorData{
		ID:                 m.ID,
		Name:               m.Name,
		URL:                m.Url,
		Method:             m.Method,
		IntervalSeconds:    int(m.IntervalSeconds),
		ExpectedStatus:     int(m.ExpectedStatus),
		Keyword:            m.Keyword,
		AlertAfterFailures: int(m.AlertAfterFailures),
		UserID:             m.UserID,
	}
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}
