package httpadapter

import (
	"bytes"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/app"
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


func (h *PageHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		if err := h.auth.Logout(c.UserContext(), sessionID); err != nil {
			slog.Warn("logout failed — session will expire via Redis TTL", "error", err)
		}
	}
	c.ClearCookie("session_id")
	return c.Redirect("/")
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

// Channel + API-Key page handlers migrated to Next.js frontend in C3.
// JSON endpoints under /api/channels* and /api/api-keys* serve the new UI.

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
