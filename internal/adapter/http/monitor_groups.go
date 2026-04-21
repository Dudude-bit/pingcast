package httpadapter

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
)

// ListMonitorGroups returns every group the caller owns, ordered by
// (ordering, id). Any plan can read — Pro-gating is on writes only.
func (s *Server) ListMonitorGroups(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	groups, err := s.monitoring.ListMonitorGroups(c.UserContext(), user.ID)
	if err != nil {
		return httperr.Write(c, err)
	}
	out := make([]apigen.MonitorGroup, len(groups))
	for i, g := range groups {
		ordering := int(g.Ordering)
		out[i] = apigen.MonitorGroup{
			Id:        g.ID,
			Name:      g.Name,
			Ordering:  ordering,
			CreatedAt: g.CreatedAt,
		}
	}
	return c.JSON(out)
}

// CreateMonitorGroup opens a new group. Pro-gated upstream.
func (s *Server) CreateMonitorGroup(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req apigen.CreateMonitorGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	ordering := 0
	if req.Ordering != nil {
		ordering = *req.Ordering
	}
	g, err := s.monitoring.CreateMonitorGroup(c.UserContext(), user.ID, req.Name, ordering)
	if err != nil {
		return httperr.Write(c, err)
	}
	ord := int(g.Ordering)
	return c.Status(fiber.StatusCreated).JSON(apigen.MonitorGroup{
		Id:        g.ID,
		Name:      g.Name,
		Ordering:  ord,
		CreatedAt: g.CreatedAt,
	})
}

// UpdateMonitorGroup renames + re-orders a group. Pro-gated.
func (s *Server) UpdateMonitorGroup(c *fiber.Ctx, id int64) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req apigen.CreateMonitorGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	ordering := 0
	if req.Ordering != nil {
		ordering = *req.Ordering
	}
	if err := s.monitoring.UpdateMonitorGroup(c.UserContext(), id, user.ID, req.Name, ordering); err != nil {
		return httperr.Write(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DeleteMonitorGroup drops a group (monitors inside fall to group_id
// NULL via ON DELETE SET NULL). Pro-gated.
func (s *Server) DeleteMonitorGroup(c *fiber.Ctx, id int64) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	if err := s.monitoring.DeleteMonitorGroup(c.UserContext(), id, user.ID); err != nil {
		return httperr.Write(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AssignMonitorToGroup re-parents a monitor (group_id=null to unassign).
// Pro-gated.
func (s *Server) AssignMonitorToGroup(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req apigen.AssignMonitorGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	if err := s.monitoring.AssignMonitorGroup(c.UserContext(), uuid.UUID(id), user.ID, req.GroupId); err != nil {
		return httperr.Write(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
