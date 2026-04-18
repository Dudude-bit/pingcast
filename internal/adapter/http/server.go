package httpadapter

import (
	cryptoRand "crypto/rand"
	sha256Hash "crypto/sha256"
	"encoding/json"
	hexEncoding "encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
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
	rateLimiter port.RateLimiter
	apiKeys     port.APIKeyRepo
}

func NewServer(
	auth *app.AuthService,
	monitoring *app.MonitoringService,
	alerts *app.AlertService,
	rateLimiter port.RateLimiter,
	apiKeys port.APIKeyRepo,
) *Server {
	return &Server{
		auth:        auth,
		monitoring:  monitoring,
		alerts:      alerts,
		rateLimiter: rateLimiter,
		apiKeys:     apiKeys,
	}
}

// Compile-time check
var _ apigen.ServerInterface = (*Server)(nil)

func (s *Server) Register(c *fiber.Ctx) error {
	var req apigen.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}

	// Rate limit by IP (spec §5: register 10/hour/IP — current bucket is
	// the shared auth bucket at 5/15min; refine in Phase 4 polish)
	allowed, err := s.rateLimiter.Allow(c.UserContext(), c.IP())
	if err != nil {
		return httperr.Write(c, fmt.Errorf("rate limiter: %w", err))
	}
	if !allowed {
		return httperr.WriteRateLimited(c, 60)
	}

	user, sessionID, err := s.auth.Register(c.UserContext(), string(req.Email), req.Slug, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrUserExists) {
			slog.Info("duplicate registration attempt", "email", string(req.Email))
		} else {
			slog.Warn("registration failed", "error", err)
		}
		return httperr.Write(c, err)
	}

	setSessionCookie(c, sessionID)

	return c.Status(201).JSON(apigen.AuthResponse{
		User:      domainUserToAPI(user),
		SessionId: &sessionID,
	})
}

func (s *Server) Login(c *fiber.Ctx) error {
	var req apigen.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}

	allowed, err := s.rateLimiter.Allow(c.UserContext(), string(req.Email))
	if err != nil {
		return httperr.Write(c, fmt.Errorf("rate limiter: %w", err))
	}
	if !allowed {
		return httperr.WriteRateLimited(c, 60)
	}

	user, sessionID, err := s.auth.Login(c.UserContext(), string(req.Email), req.Password)
	if err != nil {
		return httperr.WriteUnauthorized(c)
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
	if sessionID == "" {
		return httperr.WriteUnauthorized(c)
	}

	if err := s.auth.Logout(c.UserContext(), sessionID); err != nil {
		slog.Warn("logout failed — session will expire via Redis TTL", "error", err)
	}
	c.ClearCookie("session_id")
	return c.SendStatus(204)
}

func (s *Server) ListMonitors(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	rows, err := s.monitoring.ListMonitorsWithUptime(c.UserContext(), user.ID)
	if err != nil {
		return httperr.Write(c, err)
	}

	result := make([]apigen.MonitorWithUptime, 0, len(rows))
	for _, r := range rows {
		uptimeF := float32(r.Uptime)
		item, err := s.domainMonitorToAPIWithUptime(&r.Monitor, &uptimeF)
		if err != nil {
			return httperr.Write(c, err)
		}
		result = append(result, item)
	}

	return c.JSON(result)
}

func (s *Server) CreateMonitor(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	var req apigen.CreateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}

	checkConfigJSON, err := json.Marshal(req.CheckConfig)
	if err != nil {
		return httperr.WriteMalformedJSON(c)
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
		slog.Warn("create monitor failed", "error", err)
		return httperr.Write(c, err)
	}

	resp, err := s.domainMonitorToAPI(mon)
	if err != nil {
		return httperr.Write(c, err)
	}
	return c.Status(201).JSON(resp)
}

// ListMonitorTypes returns available monitor types and their config schemas.
func (s *Server) ListMonitorTypes(c *fiber.Ctx) error {
	types := s.monitoring.Registry().Types()
	return c.JSON(types)
}

func (s *Server) GetMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	detail, err := s.monitoring.GetMonitorDetail(c.UserContext(), uuid.UUID(id))
	if err != nil {
		return httperr.Write(c, err)
	}
	if detail.Monitor.UserID != user.ID {
		return httperr.WriteForbiddenTenant(c)
	}

	apiIncidents := make([]apigen.Incident, 0, len(detail.Incidents))
	for _, inc := range detail.Incidents {
		apiIncidents = append(apiIncidents, domainIncidentToAPI(&inc))
	}

	apiChart := make([]apigen.ChartPoint, 0, len(detail.Chart24h))
	for _, p := range detail.Chart24h {
		ts := p.Timestamp
		avg := float32(p.AvgResponseMs)
		count := p.CheckCount
		apiChart = append(apiChart, apigen.ChartPoint{
			Timestamp:     &ts,
			AvgResponseMs: &avg,
			CheckCount:    &count,
		})
	}

	resp, err := s.domainMonitorToAPIDetail(
		&detail.Monitor,
		detail.Uptime24h, detail.Uptime7d, detail.Uptime30d,
		apiChart, apiIncidents,
	)
	if err != nil {
		return httperr.Write(c, err)
	}
	return c.JSON(resp)
}

func (s *Server) UpdateMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	var req apigen.UpdateMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
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
			return httperr.WriteMalformedJSON(c)
		}
		input.CheckConfig = configJSON
	}

	updated, err := s.monitoring.UpdateMonitor(c.UserContext(), user, uuid.UUID(id), input)
	if err != nil {
		slog.Warn("update monitor failed", "error", err)
		return httperr.Write(c, err)
	}

	resp, err := s.domainMonitorToAPI(updated)
	if err != nil {
		return httperr.Write(c, err)
	}
	return c.JSON(resp)
}

func (s *Server) DeleteMonitor(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	if err := s.monitoring.DeleteMonitor(c.UserContext(), user.ID, uuid.UUID(id)); err != nil {
		return httperr.Write(c, err)
	}

	return c.SendStatus(204)
}

func (s *Server) ToggleMonitorPause(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	updated, err := s.monitoring.TogglePause(c.UserContext(), user, uuid.UUID(id))
	if err != nil {
		return httperr.Write(c, err)
	}

	resp, err := s.domainMonitorToAPI(updated)
	if err != nil {
		return httperr.Write(c, err)
	}
	return c.JSON(resp)
}

func (s *Server) GetStatusPage(c *fiber.Ctx, slug string) error {
	data, err := s.monitoring.GetStatusPage(c.UserContext(), slug)
	if err != nil {
		// Public endpoint — spec §4: unknown slug is a straight 404.
		return httperr.WriteNotFound(c, "status page")
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

func (s *Server) domainMonitorToAPI(m *domain.Monitor) (apigen.Monitor, error) {
	status := apigen.MonitorCurrentStatus(m.CurrentStatus)
	intervalSeconds := m.IntervalSeconds
	alertAfter := m.AlertAfterFailures
	target, err := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	if err != nil {
		return apigen.Monitor{}, fmt.Errorf("resolve target for monitor %s: %w", m.ID, err)
	}
	monType := string(m.Type)
	checkConfig, err := m.ParseCheckConfig()
	if err != nil {
		return apigen.Monitor{}, fmt.Errorf("parse config for monitor %s: %w", m.ID, err)
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
	}, nil
}

func (s *Server) domainMonitorToAPIWithUptime(m *domain.Monitor, uptime *float32) (apigen.MonitorWithUptime, error) {
	target, err := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	if err != nil {
		return apigen.MonitorWithUptime{}, fmt.Errorf("resolve target for monitor %s: %w", m.ID, err)
	}
	checkConfig, err := m.ParseCheckConfig()
	if err != nil {
		return apigen.MonitorWithUptime{}, fmt.Errorf("parse config for monitor %s: %w", m.ID, err)
	}
	status := apigen.MonitorWithUptimeCurrentStatus(m.CurrentStatus)
	intervalSeconds := m.IntervalSeconds
	alertAfter := m.AlertAfterFailures
	monType := string(m.Type)
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
	}, nil
}

func (s *Server) domainMonitorToAPIDetail(m *domain.Monitor, u24h, u7d, u30d float64, chartData []apigen.ChartPoint, incidents []apigen.Incident) (apigen.MonitorDetail, error) {
	target, err := s.monitoring.Registry().Target(m.Type, m.CheckConfig)
	if err != nil {
		return apigen.MonitorDetail{}, fmt.Errorf("resolve target for monitor %s: %w", m.ID, err)
	}
	checkConfig, err := m.ParseCheckConfig()
	if err != nil {
		return apigen.MonitorDetail{}, fmt.Errorf("parse config for monitor %s: %w", m.ID, err)
	}
	status := apigen.MonitorDetailCurrentStatus(m.CurrentStatus)
	intervalSeconds := m.IntervalSeconds
	alertAfter := m.AlertAfterFailures
	monType := string(m.Type)
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
	}, nil
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
	user := requireUser(c)
	if user == nil {
		return nil
	}
	channels, err := s.alerts.ListChannels(c.UserContext(), user.ID)
	if err != nil {
		return httperr.Write(c, err)
	}
	result := make([]apigen.NotificationChannel, 0, len(channels))
	for _, ch := range channels {
		result = append(result, domainChannelToAPI(&ch))
	}
	return c.JSON(result)
}

func (s *Server) CreateChannel(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req apigen.CreateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	ch, err := s.alerts.CreateChannel(c.UserContext(), user.ID, app.CreateChannelInput{
		Name:   req.Name,
		Type:   domain.ChannelType(req.Type),
		Config: configJSON,
	})
	if err != nil {
		slog.Warn("channel handler error", "path", c.Path(), "error", err)
		return httperr.Write(c, err)
	}
	return c.Status(201).JSON(domainChannelToAPI(ch))
}

func (s *Server) UpdateChannel(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req apigen.UpdateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
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
		var err error
		configJSON, err = json.Marshal(*req.Config)
		if err != nil {
			return httperr.WriteMalformedJSON(c)
		}
	}
	ch, err := s.alerts.UpdateChannel(c.UserContext(), user.ID, uuid.UUID(id), name, configJSON, isEnabled)
	if err != nil {
		slog.Warn("channel handler error", "path", c.Path(), "error", err)
		return httperr.Write(c, err)
	}
	return c.JSON(domainChannelToAPI(ch))
}

func (s *Server) DeleteChannel(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	if err := s.alerts.DeleteChannel(c.UserContext(), user.ID, uuid.UUID(id)); err != nil {
		slog.Warn("channel handler error", "path", c.Path(), "error", err)
		return httperr.Write(c, err)
	}
	return c.SendStatus(204)
}

func (s *Server) BindChannel(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req struct {
		ChannelID uuid.UUID `json:"channel_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	if err := s.alerts.BindChannel(c.UserContext(), user.ID, uuid.UUID(id), req.ChannelID); err != nil {
		slog.Warn("channel handler error", "path", c.Path(), "error", err)
		return httperr.Write(c, err)
	}
	return c.SendStatus(200)
}

func (s *Server) UnbindChannel(c *fiber.Ctx, id openapi_types.UUID, channelId openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	if err := s.alerts.UnbindChannel(c.UserContext(), user.ID, uuid.UUID(id), uuid.UUID(channelId)); err != nil {
		slog.Warn("channel handler error", "path", c.Path(), "error", err)
		return httperr.Write(c, err)
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

// --- API Key JSON endpoints ---

func (s *Server) ListAPIKeys(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	keys, err := s.apiKeys.ListByUser(c.UserContext(), user.ID)
	if err != nil {
		return httperr.Write(c, err)
	}
	result := make([]apigen.APIKey, len(keys))
	for i, k := range keys {
		result[i] = domainAPIKeyToAPI(&k)
	}
	return c.JSON(result)
}

func (s *Server) CreateAPIKey(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req apigen.CreateAPIKeyJSONRequestBody
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	if req.Name == "" || len(req.Scopes) == 0 {
		return httperr.WriteValidation(c, "name and scopes required")
	}

	randomBytes := make([]byte, 32)
	if _, err := cryptoRand.Read(randomBytes); err != nil {
		return httperr.Write(c, fmt.Errorf("generate api key: %w", err))
	}
	rawKey := "pc_live_" + hexEncoding.EncodeToString(randomBytes)

	hash := sha256Hash.Sum256([]byte(rawKey))
	keyHash := hexEncoding.EncodeToString(hash[:])

	scopes := make([]string, len(req.Scopes))
	for i, s := range req.Scopes {
		scopes[i] = string(s)
	}

	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}

	apiKey := &domain.APIKey{
		UserID:    user.ID,
		KeyHash:   keyHash,
		Name:      req.Name,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
	}

	created, err := s.apiKeys.Create(c.UserContext(), apiKey)
	if err != nil {
		return httperr.Write(c, err)
	}

	apiKeyResp := domainAPIKeyToAPI(created)
	return c.Status(201).JSON(apigen.APIKeyCreated{
		Key:    &apiKeyResp,
		RawKey: &rawKey,
	})
}

func (s *Server) RevokeAPIKey(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	if err := s.apiKeys.Delete(c.UserContext(), uuid.UUID(id), user.ID); err != nil {
		return httperr.Write(c, err)
	}
	return c.SendStatus(204)
}

func domainAPIKeyToAPI(k *domain.APIKey) apigen.APIKey {
	result := apigen.APIKey{
		Id:        (*openapi_types.UUID)(&k.ID),
		Name:      &k.Name,
		Scopes:    &k.Scopes,
		CreatedAt: &k.CreatedAt,
	}
	if k.LastUsedAt != nil {
		result.LastUsedAt = k.LastUsedAt
	}
	if k.ExpiresAt != nil {
		result.ExpiresAt = k.ExpiresAt
	}
	return result
}
