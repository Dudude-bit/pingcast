package httpadapter

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"

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
	rateLimiter *RateLimiter
	templates   map[string]*template.Template
}

func NewPageHandler(auth *app.AuthService, monitoring *app.MonitoringService, alerts *app.AlertService, rateLimiter *RateLimiter) *PageHandler {
	tmplFS, _ := fs.Sub(web.FS, "templates")

	// Parse each page template paired with layout.
	// This is required because Go's html/template only keeps the last {{define "content"}}
	// when all pages are parsed together.
	pages := []string{
		"landing.html", "login.html", "register.html",
		"dashboard.html", "monitor_detail.html", "monitor_form.html",
		"channels.html", "channel_form.html",
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

	if !h.rateLimiter.Allow(email) {
		return h.render(c, "login.html", fiber.Map{"Error": "Too many login attempts. Try again later."})
	}

	_, sessionID, err := h.auth.Login(c.UserContext(), email, password)
	if err != nil {
		h.rateLimiter.Record(email)
		return h.render(c, "login.html", fiber.Map{"Error": "Invalid email or password."})
	}

	h.rateLimiter.Reset(email)
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
		return h.render(c, "register.html", fiber.Map{"Error": err.Error()})
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

	rows, _ := h.monitoring.ListMonitorsWithUptime(c.UserContext(), user.ID)

	type MonitorRow struct {
		Monitor domain.Monitor
		Uptime  float64
		Target  string
	}

	viewRows := make([]MonitorRow, 0, len(rows))
	for _, r := range rows {
		viewRows = append(viewRows, MonitorRow{
			Monitor: r.Monitor,
			Uptime:  r.Uptime,
			Target:  registry.Target(r.Monitor.Type, r.Monitor.CheckConfig),
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

	chartJSON, _ := json.Marshal([]struct{}{}) // empty for now
	target := h.monitoring.Registry().Target(detail.Monitor.Type, detail.Monitor.CheckConfig)

	return h.render(c, "monitor_detail.html", fiber.Map{
		"User":      user,
		"Monitor":   detail.Monitor,
		"Target":    target,
		"Uptime24h": detail.Uptime24h,
		"Uptime7d":  detail.Uptime7d,
		"Uptime30d": detail.Uptime30d,
		"ChartData": template.JS(chartJSON),
		"Incidents": detail.Incidents,
	})
}

func (h *PageHandler) MonitorNewForm(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	channels, _ := h.alerts.ListChannels(c.UserContext(), user.ID)
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
		if parsed, err := strconv.Atoi(v); err == nil {
			interval = parsed
		}
	}
	alertAfter := 3
	if v := c.FormValue("alert_after_failures"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
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
		channels, _ := h.alerts.ListChannels(c.UserContext(), user.ID)
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
	channels, _ := h.alerts.ListChannels(c.UserContext(), user.ID)
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
		if parsed, err := strconv.Atoi(v); err == nil {
			interval = parsed
		}
	}

	alertAfter := 3
	if v := c.FormValue("alert_after_failures"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			alertAfter = parsed
		}
	}

	monType := domain.MonitorType(c.FormValue("type"))
	checkConfig := h.buildCheckConfigFromForm(c, monType)

	input := app.CreateMonitorInput{
		Name:               c.FormValue("name"),
		Type:               monType,
		CheckConfig:        checkConfig,
		IntervalSeconds:    interval,
		AlertAfterFailures: alertAfter,
		IsPublic:           c.FormValue("is_public") == "on",
	}

	mon, err := h.monitoring.CreateMonitor(c.UserContext(), user, input)
	if err != nil {
		channels, _ := h.alerts.ListChannels(c.UserContext(), user.ID)
		return h.render(c, "monitor_form.html", fiber.Map{
			"User":         user,
			"Error":        err.Error(),
			"MonitorTypes": h.monitoring.Registry().Types(),
			"Channels":     channels,
		})
	}

	// Bind selected channels
	channelIDs := c.Context().PostArgs().PeekMulti("channel_ids")
	for _, cidBytes := range channelIDs {
		cid, err := uuid.Parse(string(cidBytes))
		if err == nil {
			h.alerts.BindChannel(c.UserContext(), user.ID, mon.ID, cid)
		}
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

// configFieldsForType returns config schema fields for a given type (used by pages that need schema info).
func configFieldsForType(registry port.CheckerRegistry, monType domain.MonitorType) []port.ConfigField {
	chk, err := registry.Get(monType)
	if err != nil {
		return nil
	}
	return chk.ConfigSchema().Fields
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
	channels, _ := h.alerts.ListChannels(c.UserContext(), user.ID)
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
	channels, _ := h.alerts.ListChannels(c.UserContext(), user.ID)
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

func (h *PageHandler) render(c *fiber.Ctx, name string, data fiber.Map) error {
	tmpl, ok := h.templates[name]
	if !ok {
		return c.Status(500).SendString("template not found: " + name)
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
