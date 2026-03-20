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
	"github.com/kirillinakin/pingcast/internal/web"
)

type PageHandler struct {
	auth        *app.AuthService
	monitoring  *app.MonitoringService
	rateLimiter *RateLimiter
	templates   map[string]*template.Template
}

func NewPageHandler(auth *app.AuthService, monitoring *app.MonitoringService, rateLimiter *RateLimiter) *PageHandler {
	tmplFS, _ := fs.Sub(web.FS, "templates")

	// Parse each page template paired with layout.
	// This is required because Go's html/template only keeps the last {{define "content"}}
	// when all pages are parsed together.
	pages := []string{
		"landing.html", "login.html", "register.html",
		"dashboard.html", "monitor_detail.html", "monitor_form.html",
	}

	templates := make(map[string]*template.Template, len(pages)+1)
	for _, page := range pages {
		templates[page] = template.Must(template.ParseFS(tmplFS, "layout.html", page))
	}
	// Statuspage is standalone (no layout)
	templates["statuspage.html"] = template.Must(template.ParseFS(tmplFS, "statuspage.html"))

	return &PageHandler{
		auth:        auth,
		monitoring:  monitoring,
		rateLimiter: rateLimiter,
		templates:   templates,
	}
}

func (h *PageHandler) Landing(c *fiber.Ctx) error {
	return h.render(c, "landing.html", fiber.Map{"User": UserFromCtx(c)})
}

func (h *PageHandler) LoginPage(c *fiber.Ctx) error {
	return h.render(c, "login.html", nil)
}

func (h *PageHandler) LoginSubmit(c *fiber.Ctx) error {
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
	return h.render(c, "register.html", nil)
}

func (h *PageHandler) RegisterSubmit(c *fiber.Ctx) error {
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

	rows, _ := h.monitoring.ListMonitorsWithUptime(c.UserContext(), user.ID)

	type MonitorRow struct {
		Monitor domain.Monitor
		Uptime  float64
	}

	viewRows := make([]MonitorRow, 0, len(rows))
	for _, r := range rows {
		viewRows = append(viewRows, MonitorRow{Monitor: r.Monitor, Uptime: r.Uptime})
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

	return h.render(c, "monitor_detail.html", fiber.Map{
		"User":      user,
		"Monitor":   detail.Monitor,
		"Uptime24h": detail.Uptime24h,
		"Uptime7d":  detail.Uptime7d,
		"Uptime30d": detail.Uptime30d,
		"ChartData": template.JS(chartJSON),
		"Incidents": detail.Incidents,
	})
}

func (h *PageHandler) MonitorNewForm(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	return h.render(c, "monitor_form.html", fiber.Map{"User": user})
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
	return h.render(c, "monitor_form.html", fiber.Map{"User": user, "Monitor": detail.Monitor})
}

func (h *PageHandler) MonitorCreate(c *fiber.Ctx) error {
	user := UserFromCtx(c)

	interval := 300
	if v := c.FormValue("interval_seconds"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			interval = parsed
		}
	}

	expectedStatus := 200
	if v := c.FormValue("expected_status"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			expectedStatus = parsed
		}
	}

	alertAfter := 3
	if v := c.FormValue("alert_after_failures"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			alertAfter = parsed
		}
	}

	input := app.CreateMonitorInput{
		Name:               c.FormValue("name"),
		URL:                c.FormValue("url"),
		Method:             domain.HTTPMethod(c.FormValue("method")),
		IntervalSeconds:    interval,
		ExpectedStatus:     expectedStatus,
		AlertAfterFailures: alertAfter,
		IsPublic:           c.FormValue("is_public") == "on",
	}

	_, err := h.monitoring.CreateMonitor(c.UserContext(), user, input)
	if err != nil {
		return h.render(c, "monitor_form.html", fiber.Map{
			"User": user, "Error": err.Error(),
		})
	}

	return c.Redirect("/dashboard")
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

func (h *PageHandler) render(c *fiber.Ctx, name string, data fiber.Map) error {
	tmpl, ok := h.templates[name]
	if !ok {
		return c.Status(500).SendString("template not found: " + name)
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer

	// Layout-based pages render via "layout.html", standalone pages by their own name
	execName := "layout.html"
	if name == "statuspage.html" {
		execName = name
	}

	if err := tmpl.ExecuteTemplate(&buf, execName, data); err != nil {
		return c.Status(500).SendString("template error: " + err.Error())
	}
	return c.Send(buf.Bytes())
}
