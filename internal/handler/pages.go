package handler

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/auth"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
	"github.com/kirillinakin/pingcast/internal/web"
)

type PageHandler struct {
	queries     *gen.Queries
	authService *auth.Service
	rateLimiter *auth.RateLimiter
	templates   *template.Template
}

func NewPageHandler(queries *gen.Queries, authService *auth.Service, rateLimiter *auth.RateLimiter) *PageHandler {
	tmplFS, _ := fs.Sub(web.FS, "templates")
	templates := template.Must(template.ParseFS(tmplFS,
		"layout.html", "landing.html", "login.html", "register.html",
		"dashboard.html", "monitor_detail.html", "monitor_form.html", "statuspage.html",
	))

	return &PageHandler{
		queries:     queries,
		authService: authService,
		rateLimiter: rateLimiter,
		templates:   templates,
	}
}

func (h *PageHandler) Landing(c *fiber.Ctx) error {
	return h.render(c, "landing.html", fiber.Map{"User": auth.UserFromCtx(c)})
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

	_, session, err := h.authService.Login(c.UserContext(), email, password)
	if err != nil {
		h.rateLimiter.Record(email)
		return h.render(c, "login.html", fiber.Map{"Error": "Invalid email or password."})
	}

	h.rateLimiter.Reset(email)
	setSessionCookie(c, session.ID)
	return c.Redirect("/dashboard")
}

func (h *PageHandler) RegisterPage(c *fiber.Ctx) error {
	return h.render(c, "register.html", nil)
}

func (h *PageHandler) RegisterSubmit(c *fiber.Ctx) error {
	email := c.FormValue("email")
	slug := c.FormValue("slug")
	password := c.FormValue("password")

	_, session, err := h.authService.Register(c.UserContext(), email, slug, password)
	if err != nil {
		return h.render(c, "register.html", fiber.Map{"Error": err.Error()})
	}

	setSessionCookie(c, session.ID)
	return c.Redirect("/dashboard")
}

func (h *PageHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		h.authService.Logout(c.UserContext(), sessionID)
	}
	c.ClearCookie("session_id")
	return c.Redirect("/")
}

func (h *PageHandler) Dashboard(c *fiber.Ctx) error {
	user := auth.UserFromCtx(c)

	monitors, _ := h.queries.ListMonitorsByUserID(c.UserContext(), user.ID)

	type MonitorRow struct {
		Monitor gen.Monitor
		Uptime  float64
	}

	var rows []MonitorRow
	for _, m := range monitors {
		uptimeRaw, _ := h.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{
			MonitorID: m.ID,
			CheckedAt: time.Now().Add(-24 * time.Hour),
		})
		rows = append(rows, MonitorRow{Monitor: m, Uptime: toFloat64(uptimeRaw)})
	}

	return h.render(c, "dashboard.html", fiber.Map{
		"User":     user,
		"Monitors": rows,
	})
}

func (h *PageHandler) MonitorDetail(c *fiber.Ctx) error {
	user := auth.UserFromCtx(c)
	monID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/dashboard")
	}

	mon, err := h.queries.GetMonitorByID(c.UserContext(), monID)
	if err != nil || mon.UserID != user.ID {
		return c.Redirect("/dashboard")
	}

	now := time.Now()
	u24Raw, _ := h.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{MonitorID: monID, CheckedAt: now.Add(-24 * time.Hour)})
	u7Raw, _ := h.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{MonitorID: monID, CheckedAt: now.Add(-7 * 24 * time.Hour)})
	u30Raw, _ := h.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{MonitorID: monID, CheckedAt: now.Add(-30 * 24 * time.Hour)})

	incidents, _ := h.queries.ListIncidentsByMonitorID(c.UserContext(), gen.ListIncidentsByMonitorIDParams{MonitorID: monID, Limit: 10})

	chartJSON, _ := json.Marshal([]struct{}{}) // empty for now

	return h.render(c, "monitor_detail.html", fiber.Map{
		"User":      user,
		"Monitor":   mon,
		"Uptime24h": toFloat64(u24Raw),
		"Uptime7d":  toFloat64(u7Raw),
		"Uptime30d": toFloat64(u30Raw),
		"ChartData": template.JS(chartJSON),
		"Incidents": incidents,
	})
}

func (h *PageHandler) StatusPage(c *fiber.Ctx) error {
	slug := c.Params("slug")

	user, err := h.queries.GetUserBySlug(c.UserContext(), slug)
	if err != nil {
		return c.Status(http.StatusNotFound).SendString("Not found")
	}

	monitors, _ := h.queries.ListPublicMonitorsByUserSlug(c.UserContext(), slug)

	allUp := true
	type StatusMon struct {
		Name          string
		CurrentStatus string
		Uptime90d     float64
	}
	var statusMons []StatusMon
	for _, m := range monitors {
		uptimeRaw, _ := h.queries.GetUptimePercent(c.UserContext(), gen.GetUptimePercentParams{
			MonitorID: m.ID,
			CheckedAt: time.Now().Add(-90 * 24 * time.Hour),
		})
		if m.CurrentStatus != "up" {
			allUp = false
		}
		statusMons = append(statusMons, StatusMon{Name: m.Name, CurrentStatus: m.CurrentStatus, Uptime90d: toFloat64(uptimeRaw)})
	}

	showBranding := user.Plan == "free"
	return h.render(c, "statuspage.html", fiber.Map{
		"Slug": slug, "AllUp": allUp, "Monitors": statusMons, "ShowBranding": showBranding,
	})
}

func (h *PageHandler) render(c *fiber.Ctx, name string, data fiber.Map) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return c.Status(500).SendString("template error: " + err.Error())
	}
	return c.Send(buf.Bytes())
}
