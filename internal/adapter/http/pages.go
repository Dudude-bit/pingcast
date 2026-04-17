package httpadapter

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// buildCheckConfigFromForm reads config field values from the form based on the schema.
// Retained for use by channel form handlers until C3 migrates them.
//
//nolint:unused // kept for C3
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
	if err := h.alerts.DeleteChannel(c.UserContext(), user.ID, chID); err != nil {
		slog.Warn("failed to delete channel", "channel_id", chID, "user_id", user.ID, "error", err)
	}
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
	if err := h.apiKeyRepo.Delete(c.UserContext(), keyID, user.ID); err != nil {
		slog.Warn("failed to delete api key", "key_id", keyID, "user_id", user.ID, "error", err)
	}
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
