package httpadapter

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// GetStatusBadge returns an SVG status badge for embedding in READMEs,
// docs sites, and anywhere else that accepts a raw <img>. Shields.io-
// shaped (left label "status", right value with state colour). 60s
// cached at the edge + client. Free tier embeds a tiny "via PingCast"
// link inside the SVG; Pro omits it.
//
// Path: /status/:slug/badge.svg (registered outside the apigen router
// so we can own the whole Content-Type).
func (s *Server) GetStatusBadge(c *fiber.Ctx) error {
	slug := c.Params("slug")

	data, err := s.monitoring.GetStatusPage(c.UserContext(), slug)
	if err != nil {
		return httperr.WriteNotFound(c, "status page")
	}

	state, color := badgeState(data.Monitors)
	includeCredit := data.ShowBranding // Free tier → embed "via PingCast"

	c.Set(fiber.HeaderContentType, "image/svg+xml")
	c.Set(fiber.HeaderCacheControl, "public, max-age=60")
	return c.SendString(renderBadgeSVG(state, color, includeCredit))
}

// badgeState collapses a list of monitor statuses into a single badge
// label + colour. Empty/unknown defaults to grey/"unknown" so an
// unconfigured slug still renders a sane badge instead of 5xx.
func badgeState(monitors []app.StatusMonitor) (label, color string) {
	if len(monitors) == 0 {
		return "unknown", "#94a3b8" // slate-400
	}
	anyDown := false
	anyDegraded := false
	for _, m := range monitors {
		switch m.CurrentStatus {
		case domain.StatusDown:
			anyDown = true
		case domain.StatusUnknown:
			anyDegraded = true
		}
	}
	if anyDown {
		return "down", "#ef4444" // red-500
	}
	if anyDegraded {
		return "degraded", "#f59e0b" // amber-500
	}
	return "operational", "#10b981" // emerald-500
}

// renderBadgeSVG renders a fixed-width shields.io-style badge. The left
// half always says "status"; the right half carries the computed label.
// No external font dependencies — uses system-stack via the enclosing
// <style> tag so GitHub's sanitiser leaves it alone.
func renderBadgeSVG(label, color string, includeCredit bool) string {
	// Widths are hand-tuned for the label strings above (5-11 chars).
	leftWidth := 48
	rightWidth := 68
	if len(label) > 10 {
		rightWidth = 8 * (len(label) + 2)
	}
	total := leftWidth + rightWidth
	height := 20

	// Role + title anchor the accessibility label for screen readers.
	role := fmt.Sprintf("status: %s", label)

	credit := ""
	if includeCredit {
		credit = `<a xlink:href="https://pingcast.io" target="_blank">` +
			`<text x="` + intToStr(total-2) + `" y="14" font-size="8" fill="#e2e8f0" text-anchor="end" font-family="system-ui,Verdana,sans-serif" opacity="0.7">via PingCast</text>` +
			`</a>`
	}

	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d" role="img" aria-label="%s">`+
			`<title>%s</title>`+
			`<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#fff" stop-opacity=".2"/><stop offset="1" stop-opacity=".2"/></linearGradient>`+
			`<rect width="%d" height="%d" rx="3" fill="#555"/>`+
			`<rect x="%d" width="%d" height="%d" rx="3" fill="%s"/>`+
			`<rect width="%d" height="%d" rx="3" fill="url(#s)"/>`+
			`<g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">`+
			`<text x="%d" y="14">status</text>`+
			`<text x="%d" y="14">%s</text>`+
			`</g>%s</svg>`,
		total, height, role,
		role,
		total, height,
		leftWidth, rightWidth, height, color,
		total, height,
		leftWidth/2,
		leftWidth+rightWidth/2, escapeXMLText(label),
		credit,
	)
}

// escapeXMLText drops the three characters the SVG parser really cares
// about. Labels come from our closed enum, so this is belt-and-braces.
func escapeXMLText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func intToStr(i int) string { return fmt.Sprintf("%d", i) }
