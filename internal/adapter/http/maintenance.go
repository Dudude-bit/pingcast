package httpadapter

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	openapi_types "github.com/oapi-codegen/runtime/types"

	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// ListMaintenanceWindows returns every maintenance window owned by the
// caller across all their monitors. Any plan can read — the scheduling
// side (POST/DELETE) is Pro-gated upstream.
func (s *Server) ListMaintenanceWindows(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	windows, err := s.monitoring.ListMaintenanceWindows(c.UserContext(), user.ID)
	if err != nil {
		return httperr.Write(c, err)
	}
	out := make([]apigen.MaintenanceWindow, len(windows))
	for i, w := range windows {
		out[i] = apigen.MaintenanceWindow{
			Id:        w.ID,
			MonitorId: openapi_types.UUID(w.MonitorID),
			StartsAt:  w.StartsAt,
			EndsAt:    w.EndsAt,
			Reason:    w.Reason,
			CreatedAt: w.CreatedAt,
		}
	}
	return c.JSON(out)
}

// ScheduleMaintenanceWindow opens a new window. Pro-gated.
func (s *Server) ScheduleMaintenanceWindow(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req apigen.ScheduleMaintenanceWindowRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	w, err := s.monitoring.ScheduleMaintenance(c.UserContext(), app.ScheduleMaintenanceInput{
		MonitorID: req.MonitorId,
		UserID:    user.ID,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
		Reason:    req.Reason,
	})
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return httperr.WriteForbiddenTenant(c)
		}
		return httperr.Write(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(apigen.MaintenanceWindow{
		Id:        w.ID,
		MonitorId: openapi_types.UUID(w.MonitorID),
		StartsAt:  w.StartsAt,
		EndsAt:    w.EndsAt,
		Reason:    w.Reason,
		CreatedAt: w.CreatedAt,
	})
}

// DeleteMaintenanceWindow removes a window by id (only if the caller
// owns the parent monitor — enforced in the repo query). Pro-gated.
func (s *Server) DeleteMaintenanceWindow(c *fiber.Ctx, id int64) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	if err := s.monitoring.DeleteMaintenanceWindow(c.UserContext(), id, user.ID); err != nil {
		return httperr.Write(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
