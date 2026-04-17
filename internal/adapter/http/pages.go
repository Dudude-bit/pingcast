package httpadapter

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/web"
)

type PageHandler struct {
	auth        *app.AuthService
	monitoring  *app.MonitoringService
	alerts      *app.AlertService
	rateLimiter port.RateLimiter
	apiKeyRepo  port.APIKeyRepo
	templates   map[string]*template.Template
}

func NewPageHandler(auth *app.AuthService, monitoring *app.MonitoringService, alerts *app.AlertService, rateLimiter port.RateLimiter, apiKeyRepo port.APIKeyRepo) *PageHandler {
	tmplFS, _ := fs.Sub(web.FS, "templates")

	// Parse each page template paired with layout.
	// This is required because Go's html/template only keeps the last {{define "content"}}
	// when all pages are parsed together.
	pages := []string{
		"landing.html", "login.html", "register.html",
		"dashboard.html", "monitor_detail.html", "monitor_form.html",
		"channels.html", "channel_form.html",
		"api_keys.html", "api_key_form.html",
	}

	templates := make(map[string]*template.Template, len(pages)+2)
	for _, page := range pages {
		templates[page] = template.Must(template.ParseFS(tmplFS, "layout.html", page))
	}
	// Statuspage is standalone (no layout)
	templates["statuspage.html"] = template.Must(template.ParseFS(tmplFS, "statuspage.html"))
	// Config fields partial (standalone, no layout)
	templates["monitor_config_fields.html"] = template.Must(template.ParseFS(tmplFS, "monitor_config_fields.html"))

	return &PageHandler{
		auth:        auth,
		monitoring:  monitoring,
		alerts:      alerts,
		rateLimiter: rateLimiter,
		apiKeyRepo:  apiKeyRepo,
		templates:   templates,
	}
}

func (h *PageHandler) Landing(c *fiber.Ctx) error {
	if h.isLoggedIn(c) {
		return c.Redirect("/dashboard")
	}
	return h.render(c, "landing.html", nil)
}

func (h *PageHandler) isLoggedIn(c *fiber.Ctx) bool {
	sessionID := c.Cookies("session_id")
	if sessionID == "" {
		return false
	}
	user, err := h.auth.ValidateSession(c.UserContext(), sessionID)
	return err == nil && user != nil
}

func (h *PageHandler) LoginPage(c *fiber.Ctx) error {
	if h.isLoggedIn(c) {
		return c.Redirect("/dashboard")
	}
	return h.render(c, "login.html", nil)
}

func (h *PageHandler) LoginSubmit(c *fiber.Ctx) error {
	if h.isLoggedIn(c) {
		return c.Redirect("/dashboard")
	}
	email := c.FormValue("email")
	password := c.FormValue("password")

	allowed, err := h.rateLimiter.Allow(c.UserContext(), email)
	if err != nil {
		return h.render(c, "login.html", fiber.Map{"Error": "Service temporarily unavailable. Try again later."})
	}
	if !allowed {
		return h.render(c, "login.html", fiber.Map{"Error": "Too many login attempts. Try again later."})
	}

	_, sessionID, err := h.auth.Login(c.UserContext(), email, password)
	if err != nil {
		return h.render(c, "login.html", fiber.Map{"Error": "Invalid email or password."})
	}

	if err := h.rateLimiter.Reset(c.UserContext(), email); err != nil {
		slog.Warn("rate limiter reset failed after successful login", "email", email, "error", err)
	}
	setSessionCookie(c, sessionID)
	return c.Redirect("/dashboard")
}

func (h *PageHandler) RegisterPage(c *fiber.Ctx) error {
	if h.isLoggedIn(c) {
		return c.Redirect("/dashboard")
	}
	return h.render(c, "register.html", nil)
}

func (h *PageHandler) RegisterSubmit(c *fiber.Ctx) error {
	if h.isLoggedIn(c) {
		return c.Redirect("/dashboard")
	}
	email := c.FormValue("email")
	slug := c.FormValue("slug")
	password := c.FormValue("password")

	_, sessionID, err := h.auth.Register(c.UserContext(), email, slug, password)
	if err != nil {
		if errors.Is(err, domain.ErrUserExists) {
			slog.Info("duplicate registration attempt", "email", email)
		} else {
			slog.Warn("registration failed", "error", err)
		}
		return h.render(c, "register.html", fiber.Map{"Error": "Registration failed"})
	}

	setSessionCookie(c, sessionID)
	return c.Redirect("/dashboard")
}

func (h *PageHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		h.auth.Logout(c.UserContext(), sessionID)
	}
	c.ClearCookie("session_id")
	return c.Redirect("/")
}

func (h *PageHandler) Dashboard(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	registry := h.monitoring.Registry()

	rows, err := h.monitoring.ListMonitorsWithUptime(c.UserContext(), user.ID)
	if err != nil {
		slog.Error("failed to list monitors", "user_id", user.ID, "error", err)
		return h.render(c, "dashboard.html", fiber.Map{"User": user, "Error": "Failed to load monitors."})
	}

	type MonitorRow struct {
		Monitor domain.Monitor
		Uptime  float64
		Target  string
	}

	viewRows := make([]MonitorRow, 0, len(rows))
	for _, r := range rows {
		target, err := registry.Target(r.Monitor.Type, r.Monitor.CheckConfig)
		if err != nil {
			target = "(config error)"
		}
		viewRows = append(viewRows, MonitorRow{
			Monitor: r.Monitor,
			Uptime:  r.Uptime,
			Target:  target,
		})
	}

	return h.render(c, "dashboard.html", fiber.Map{
		"User":     user,
		"Monitors": viewRows,
	})
}

func (h *PageHandler) MonitorDetail(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	monID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/dashboard")
	}

	detail, err := h.monitoring.GetMonitorDetail(c.UserContext(), monID)
	if err != nil || detail.Monitor.UserID != user.ID {
		return c.Redirect("/dashboard")
	}

	target, err := h.monitoring.Registry().Target(detail.Monitor.Type, detail.Monitor.CheckConfig)
	if err != nil {
		return h.render(c, "monitor_detail.html", fiber.Map{
			"User": user, "Monitor": detail.Monitor, "Error": "Failed to resolve monitor target: " + err.Error(),
		})
	}

	return h.render(c, "monitor_detail.html", fiber.Map{
		"User":      user,
		"Monitor":   detail.Monitor,
		"Target":    target,
		"Uptime24h": detail.Uptime24h,
		"Uptime7d":  detail.Uptime7d,
		"Uptime30d": detail.Uptime30d,
		"Incidents": detail.Incidents,
	})
}

func (h *PageHandler) MonitorNewForm(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	channels, err := h.alerts.ListChannels(c.UserContext(), user.ID)
	if err != nil {
		slog.Error("failed to list channels", "user_id", user.ID, "error", err)
		return h.render(c, "monitor_form.html", fiber.Map{"User": user, "Error": "Failed to load channels."})
	}
	return h.render(c, "monitor_form.html", fiber.Map{
		"User":         user,
		"MonitorTypes": h.monitoring.Registry().Types(),
		"Channels":     channels,
	})
}

func (h *PageHandler) MonitorUpdate(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	monID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/dashboard")
	}

	monType := domain.MonitorType(c.FormValue("type"))
	checkConfig := h.buildCheckConfigFromForm(c, monType)

	interval := 300
	if v := c.FormValue("interval_seconds"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 30 && parsed <= 86400 {
			interval = parsed
		}
	}
	alertAfter := 3
	if v := c.FormValue("alert_after_failures"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 1 && parsed <= 10 {
			alertAfter = parsed
		}
	}

	name := c.FormValue("name")
	isPublic := c.FormValue("is_public") == "on"

	input := app.UpdateMonitorInput{
		Name:               &name,
		CheckConfig:        checkConfig,
		IntervalSeconds:    &interval,
		AlertAfterFailures: &alertAfter,
		IsPublic:           &isPublic,
	}

	_, err = h.monitoring.UpdateMonitor(c.UserContext(), user, monID, input)
	if err != nil {
		channels, chErr := h.alerts.ListChannels(c.UserContext(), user.ID)
		if chErr != nil {
			slog.Error("failed to list channels", "user_id", user.ID, "error", chErr)
			return h.render(c, "monitor_form.html", fiber.Map{"User": user, "Error": "Failed to load channels."})
		}
		return h.render(c, "monitor_form.html", fiber.Map{
			"User":         user,
			"Error":        err.Error(),
			"MonitorTypes": h.monitoring.Registry().Types(),
			"Channels":     channels,
		})
	}

	return c.Redirect("/monitors/" + monID.String())
}

func (h *PageHandler) MonitorDelete(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	monID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/dashboard")
	}
	h.monitoring.DeleteMonitor(c.UserContext(), user.ID, monID)
	return c.Redirect("/dashboard")
}

func (h *PageHandler) MonitorTogglePause(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	monID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/dashboard")
	}
	h.monitoring.TogglePause(c.UserContext(), user, monID)
	return c.Redirect("/dashboard")
}

func (h *PageHandler) MonitorEditForm(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	monID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/dashboard")
	}
	detail, err := h.monitoring.GetMonitorDetail(c.UserContext(), monID)
	if err != nil || detail.Monitor.UserID != user.ID {
		return c.Redirect("/dashboard")
	}
	channels, err := h.alerts.ListChannels(c.UserContext(), user.ID)
	if err != nil {
		slog.Error("failed to list channels", "user_id", user.ID, "error", err)
		return h.render(c, "monitor_form.html", fiber.Map{"User": user, "Error": "Failed to load channels."})
	}
	return h.render(c, "monitor_form.html", fiber.Map{
		"User":         user,
		"Monitor":      detail.Monitor,
		"MonitorTypes": h.monitoring.Registry().Types(),
		"Channels":     channels,
	})
}

func (h *PageHandler) MonitorCreate(c *fiber.Ctx) error {
	user := UserFromCtx(c)

	interval := 300
	if v := c.FormValue("interval_seconds"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 30 && parsed <= 86400 {
			interval = parsed
		}
	}

	alertAfter := 3
	if v := c.FormValue("alert_after_failures"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 1 && parsed <= 10 {
			alertAfter = parsed
		}
	}

	monType := domain.MonitorType(c.FormValue("type"))
	checkConfig := h.buildCheckConfigFromForm(c, monType)

	// Parse selected channel IDs
	var channelIDs []uuid.UUID
	for _, cidBytes := range c.Context().PostArgs().PeekMulti("channel_ids") {
		if cid, err := uuid.Parse(string(cidBytes)); err == nil {
			channelIDs = append(channelIDs, cid)
		}
	}

	input := app.CreateMonitorInput{
		Name:               c.FormValue("name"),
		Type:               monType,
		CheckConfig:        checkConfig,
		IntervalSeconds:    interval,
		AlertAfterFailures: alertAfter,
		IsPublic:           c.FormValue("is_public") == "on",
		ChannelIDs:         channelIDs,
	}

	_, err := h.monitoring.CreateMonitor(c.UserContext(), user, input)
	if err != nil {
		channels, chErr := h.alerts.ListChannels(c.UserContext(), user.ID)
		if chErr != nil {
			slog.Error("failed to list channels", "user_id", user.ID, "error", chErr)
			return h.render(c, "monitor_form.html", fiber.Map{"User": user, "Error": "Failed to load channels."})
		}
		return h.render(c, "monitor_form.html", fiber.Map{
			"User":         user,
			"Error":        err.Error(),
			"MonitorTypes": h.monitoring.Registry().Types(),
			"Channels":     channels,
		})
	}

	return c.Redirect("/dashboard")
}

// MonitorConfigFields returns the dynamic config fields for a monitor type (HTMX endpoint).
func (h *PageHandler) MonitorConfigFields(c *fiber.Ctx) error {
	monType := domain.MonitorType(c.Query("type"))
	registry := h.monitoring.Registry()

	chk, err := registry.Get(monType)
	if err != nil {
		return c.Status(400).SendString("unknown monitor type")
	}

	schema := chk.ConfigSchema()
	return h.render(c, "monitor_config_fields.html", fiber.Map{
		"Fields": schema.Fields,
	})
}

// buildCheckConfigFromForm reads config field values from the form based on the schema.
func (h *PageHandler) buildCheckConfigFromForm(c *fiber.Ctx, monType domain.MonitorType) json.RawMessage {
	registry := h.monitoring.Registry()
	chk, err := registry.Get(monType)
	if err != nil {
		return json.RawMessage("{}")
	}

	schema := chk.ConfigSchema()
	cfg := make(map[string]any, len(schema.Fields))
	for _, f := range schema.Fields {
		val := c.FormValue("config_" + f.Name)
		if val == "" {
			continue
		}
		switch f.Type {
		case "number":
			if n, err := strconv.Atoi(val); err == nil {
				cfg[f.Name] = n
			}
		default:
			cfg[f.Name] = val
		}
	}
	data, _ := json.Marshal(cfg)
	return data
}

func (h *PageHandler) StatusPage(c *fiber.Ctx) error {
	slug := c.Params("slug")

	data, err := h.monitoring.GetStatusPage(c.UserContext(), slug)
	if err != nil {
		return c.Status(http.StatusNotFound).SendString("Not found")
	}

	type StatusMon struct {
		Name          string
		CurrentStatus string
		Uptime90d     float64
	}
	var statusMons []StatusMon
	for _, m := range data.Monitors {
		statusMons = append(statusMons, StatusMon{
			Name:          m.Name,
			CurrentStatus: string(m.CurrentStatus),
			Uptime90d:     m.Uptime90d,
		})
	}

	return h.render(c, "statuspage.html", fiber.Map{
		"Slug": slug, "AllUp": data.AllUp, "Monitors": statusMons, "ShowBranding": data.ShowBranding,
	})
}

// --- Channel Pages ---

func (h *PageHandler) ChannelList(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	channels, err := h.alerts.ListChannels(c.UserContext(), user.ID)
	if err != nil {
		slog.Error("failed to list channels", "user_id", user.ID, "error", err)
		return h.render(c, "channels.html", fiber.Map{"User": user, "Error": "Failed to load channels."})
	}
	return h.render(c, "channels.html", fiber.Map{
		"User": user, "Channels": channels,
	})
}

func (h *PageHandler) ChannelNewForm(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	return h.render(c, "channel_form.html", fiber.Map{
		"User": user, "ChannelTypes": h.alerts.Registry().Types(),
	})
}

func (h *PageHandler) ChannelCreate(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	channelType := domain.ChannelType(c.FormValue("type"))
	if !channelType.Valid() {
		return h.render(c, "channel_form.html", fiber.Map{
			"User": user, "Error": "Invalid channel type", "ChannelTypes": h.alerts.Registry().Types(),
		})
	}

	configData := h.buildChannelConfigFromForm(c, channelType)

	_, err := h.alerts.CreateChannel(c.UserContext(), user.ID, app.CreateChannelInput{
		Name:   c.FormValue("name"),
		Type:   channelType,
		Config: configData,
	})
	if err != nil {
		return h.render(c, "channel_form.html", fiber.Map{
			"User": user, "Error": err.Error(), "ChannelTypes": h.alerts.Registry().Types(),
		})
	}
	return c.Redirect("/channels")
}

func (h *PageHandler) ChannelEditForm(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	chID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/channels")
	}
	channels, err := h.alerts.ListChannels(c.UserContext(), user.ID)
	if err != nil {
		slog.Error("failed to list channels", "user_id", user.ID, "error", err)
		return h.render(c, "channel_form.html", fiber.Map{"User": user, "Error": "Failed to load channels."})
	}
	var channel *domain.NotificationChannel
	for i := range channels {
		if channels[i].ID == chID {
			channel = &channels[i]
			break
		}
	}
	if channel == nil {
		return c.Redirect("/channels")
	}
	return h.render(c, "channel_form.html", fiber.Map{
		"User":         user,
		"Channel":      channel,
		"ChannelTypes": h.alerts.Registry().Types(),
	})
}

func (h *PageHandler) ChannelUpdate(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	chID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/channels")
	}

	channelType := domain.ChannelType(c.FormValue("type"))
	configData := h.buildChannelConfigFromForm(c, channelType)
	isEnabled := c.FormValue("is_enabled") == "on"

	_, err = h.alerts.UpdateChannel(c.UserContext(), user.ID, chID, c.FormValue("name"), configData, isEnabled)
	if err != nil {
		return h.render(c, "channel_form.html", fiber.Map{
			"User": user, "Error": err.Error(), "ChannelTypes": h.alerts.Registry().Types(),
		})
	}
	return c.Redirect("/channels")
}

func (h *PageHandler) ChannelDelete(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	chID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/channels")
	}
	h.alerts.DeleteChannel(c.UserContext(), user.ID, chID)
	return c.Redirect("/channels")
}

func (h *PageHandler) ChannelConfigFields(c *fiber.Ctx) error {
	channelType := domain.ChannelType(c.Query("type"))
	if !channelType.Valid() {
		return c.SendString("")
	}
	factory, err := h.alerts.Registry().Get(channelType)
	if err != nil {
		return c.SendString("")
	}
	schema := factory.ConfigSchema()
	return h.render(c, "monitor_config_fields.html", fiber.Map{"Fields": schema.Fields})
}

func (h *PageHandler) buildChannelConfigFromForm(c *fiber.Ctx, chType domain.ChannelType) json.RawMessage {
	factory, err := h.alerts.Registry().Get(chType)
	if err != nil {
		return json.RawMessage("{}")
	}
	schema := factory.ConfigSchema()
	cfg := make(map[string]any, len(schema.Fields))
	for _, f := range schema.Fields {
		val := c.FormValue("config_" + f.Name)
		if val == "" {
			continue
		}
		switch f.Type {
		case "number":
			if n, err := strconv.Atoi(val); err == nil {
				cfg[f.Name] = n
			}
		default:
			cfg[f.Name] = val
		}
	}
	data, _ := json.Marshal(cfg)
	return data
}

// --- API Key Pages ---

func (h *PageHandler) APIKeyList(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	keys, err := h.apiKeyRepo.ListByUser(c.UserContext(), user.ID)
	if err != nil {
		slog.Error("failed to list API keys", "user_id", user.ID, "error", err)
		return h.render(c, "api_keys.html", fiber.Map{"User": user, "Error": "Failed to load API keys."})
	}
	return h.render(c, "api_keys.html", fiber.Map{
		"User": user, "APIKeys": keys,
	})
}

func (h *PageHandler) APIKeyCreate(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	return h.render(c, "api_key_form.html", fiber.Map{
		"User": user,
	})
}

func (h *PageHandler) APIKeyCreateSubmit(c *fiber.Ctx) error {
	user := UserFromCtx(c)

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		return h.render(c, "api_key_form.html", fiber.Map{
			"User": user, "Error": "Name is required.",
		})
	}

	// Collect scopes from multi-value checkboxes
	var scopes []string
	for _, v := range c.Context().PostArgs().PeekMulti("scopes") {
		if s := strings.TrimSpace(string(v)); s != "" {
			scopes = append(scopes, s)
		}
	}
	if len(scopes) == 0 {
		return h.render(c, "api_key_form.html", fiber.Map{
			"User": user, "Error": "Select at least one scope.",
		})
	}

	// Parse expiry
	var expiresAt *time.Time
	if days := c.FormValue("expires"); days != "" && days != "0" {
		d, err := strconv.Atoi(days)
		if err == nil && d > 0 {
			t := time.Now().Add(time.Duration(d) * 24 * time.Hour)
			expiresAt = &t
		}
	}

	// Generate raw key: pc_live_ + 32 random bytes hex
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		slog.Error("failed to generate api key", "error", err)
		return h.render(c, "api_key_form.html", fiber.Map{
			"User": user, "Error": "Failed to generate key. Try again.",
		})
	}
	rawKey := fmt.Sprintf("pc_live_%s", hex.EncodeToString(randomBytes))

	// Hash the full key string with sha256
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	apiKey := &domain.APIKey{
		ID:        uuid.New(),
		UserID:    user.ID,
		KeyHash:   keyHash,
		Name:      name,
		Scopes:    scopes,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if _, err := h.apiKeyRepo.Create(c.UserContext(), apiKey); err != nil {
		slog.Error("failed to store api key", "error", err)
		return h.render(c, "api_key_form.html", fiber.Map{
			"User": user, "Error": "Failed to create key. Try again.",
		})
	}

	// Show list with the raw key displayed once
	keys, err := h.apiKeyRepo.ListByUser(c.UserContext(), user.ID)
	if err != nil {
		slog.Error("failed to list API keys", "user_id", user.ID, "error", err)
		return h.render(c, "api_keys.html", fiber.Map{"User": user, "Error": "Failed to load API keys."})
	}
	return h.render(c, "api_keys.html", fiber.Map{
		"User": user, "APIKeys": keys, "RawKey": rawKey,
	})
}

func (h *PageHandler) APIKeyRevoke(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	keyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/api-keys")
	}
	h.apiKeyRepo.Delete(c.UserContext(), keyID, user.ID)
	return c.Redirect("/api-keys")
}

func (h *PageHandler) render(c *fiber.Ctx, name string, data fiber.Map) error {
	tmpl, ok := h.templates[name]
	if !ok {
		return c.Status(500).SendString("template not found: " + name)
	}

	// Inject CSRF token into all template data
	if data == nil {
		data = fiber.Map{}
	}
	if token, ok := c.Locals("csrf").(string); ok {
		data["CsrfToken"] = token
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer

	// Layout-based pages render via "layout.html", standalone pages by their own name
	execName := "layout.html"
	if name == "statuspage.html" || name == "monitor_config_fields.html" {
		execName = name
	}

	if err := tmpl.ExecuteTemplate(&buf, execName, data); err != nil {
		return c.Status(500).SendString("template error: " + err.Error())
	}
	return c.Send(buf.Bytes())
}
