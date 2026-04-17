package httpadapter

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kirillinakin/pingcast/internal/domain"
)

// formRange describes an integer form field with inclusive bounds.
type formRange struct {
	field string
	min   int
	max   int
}

// parseIntInRange reads an integer form value. If the field is empty,
// defaultVal is returned. If the value parses but is out of [min, max],
// a user-facing error is returned — callers should re-render the form
// with the message instead of silently clamping.
func parseIntInRange(c *fiber.Ctx, r formRange, defaultVal int) (int, error) {
	v := c.FormValue(r.field)
	if v == "" {
		return defaultVal, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", r.field)
	}
	if parsed < r.min || parsed > r.max {
		return 0, fmt.Errorf("%s must be between %d and %d", r.field, r.min, r.max)
	}
	return parsed, nil
}

// renderMonitorFormError re-renders the monitor form with an error message,
// preserving the channel list so the form remains usable.
func (h *PageHandler) renderMonitorFormError(c *fiber.Ctx, user *domain.User, msg string) error {
	channels, err := h.alerts.ListChannels(c.UserContext(), user.ID)
	if err != nil {
		slog.Error("failed to list channels", "user_id", user.ID, "error", err)
	}
	return h.render(c, "monitor_form.html", fiber.Map{
		"User":         user,
		"Error":        msg,
		"MonitorTypes": h.monitoring.Registry().Types(),
		"Channels":     channels,
	})
}
