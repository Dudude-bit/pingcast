package httpadapter

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/xcontext"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// notifyWindow caps the detached fan-out goroutine. Long enough for a
// batch of SMTP sends, short enough that a misbehaving relay doesn't
// leak goroutines if the process is shutting down.
const notifyWindow = 30 * time.Second

// notifyIncidentAsync runs SubscriptionService.NotifyIncident in a
// detached context so the HTTP response doesn't block on SMTP. Parent
// trace is linked via xcontext.Detached for observability.
func (s *Server) notifyIncidentAsync(parent context.Context, slug, headline, state, body string) {
	go func() {
		ctx, cancel := xcontext.Detached(parent, notifyWindow, "notify-incident")
		defer cancel()
		s.subscriptions.NotifyIncident(ctx, slug, headline, state, body)
	}()
}

// CreateIncident opens a manual incident. Pro-only (gated upstream by
// proGateSelector). Body: {monitor_id, title, body}. The initial
// IncidentUpdate is created with state=investigating and body=body.
func (s *Server) CreateIncident(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	var req apigen.CreateIncidentRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}

	inc, _, err := s.monitoring.CreateManualIncident(c.UserContext(), app.CreateManualIncidentInput{
		MonitorID: req.MonitorId,
		UserID:    user.ID,
		Title:     req.Title,
		Body:      req.Body,
	})
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return httperr.WriteForbiddenTenant(c)
		}
		return httperr.Write(c, err)
	}

	s.notifyIncidentAsync(c.UserContext(), user.Slug, req.Title,
		string(domain.IncidentStateInvestigating), req.Body)

	return c.Status(fiber.StatusCreated).JSON(domainIncidentToAPI(inc))
}

// UpdateIncidentState transitions an incident and posts a narrative
// update. Pro-only (gated upstream). 409 on invalid transition, 403 on
// wrong owner.
func (s *Server) UpdateIncidentState(c *fiber.Ctx, id int64) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	var req apigen.UpdateIncidentStateRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}

	upd, err := s.monitoring.ChangeIncidentState(c.UserContext(), app.ChangeIncidentStateInput{
		IncidentID: id,
		UserID:     user.ID,
		NewState:   domain.IncidentState(req.State),
		UpdateBody: req.Body,
	})
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return httperr.WriteForbiddenTenant(c)
		}
		// Invalid transitions come back as raw errors from
		// IncidentState.CanTransitionTo — surface as 409 so the client
		// can retry with a different state.
		msg := err.Error()
		if msg != "" && (containsAny(msg, "cannot move", "invalid incident state")) {
			return c.Status(fiber.StatusConflict).JSON(httperr.Envelope{
				Error: httperr.Inner{Code: "INVALID_STATE_TRANSITION", Message: msg},
			})
		}
		return httperr.Write(c, err)
	}

	// Fire-and-forget fan-out to confirmed status-page subscribers.
	// Request may end before SMTP sends complete; the user doesn't
	// wait on email delivery to see the PATCH response.
	s.notifyIncidentAsync(c.UserContext(), user.Slug,
		"Incident update: "+string(upd.State), string(upd.State), upd.Body)

	return c.JSON(domainIncidentUpdateToAPI(upd))
}

// ListIncidentUpdates returns the public timeline for an incident. No
// auth; safe because incidents are already scoped to a public status
// page via the monitor.is_public flag downstream of the status-page
// rendering path.
func (s *Server) ListIncidentUpdates(c *fiber.Ctx, id int64) error {
	updates, err := s.monitoring.ListIncidentUpdates(c.UserContext(), id)
	if err != nil {
		return httperr.Write(c, err)
	}
	out := make([]apigen.IncidentUpdate, len(updates))
	for i := range updates {
		out[i] = domainIncidentUpdateToAPI(&updates[i])
	}
	return c.JSON(out)
}

func domainIncidentUpdateToAPI(u *domain.IncidentUpdate) apigen.IncidentUpdate {
	return apigen.IncidentUpdate{
		Id:              u.ID,
		IncidentId:      u.IncidentID,
		State:           apigen.IncidentUpdateState(u.State),
		Body:            u.Body,
		PostedByUserId:  openapi_types.UUID(u.PostedByUserID),
		PostedAt:        u.PostedAt,
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			match := true
			for j := 0; j < len(sub); j++ {
				if s[i+j] != sub[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}
