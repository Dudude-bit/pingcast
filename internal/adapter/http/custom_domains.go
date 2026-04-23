package httpadapter

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// ListCustomDomains returns every domain the caller has requested.
// Readable on any plan so a downgraded user can still see what they had.
func (s *Server) ListCustomDomains(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	rows, err := s.customDomains.ListDomains(c.UserContext(), user.ID)
	if err != nil {
		return httperr.Write(c, err)
	}
	out := make([]apigen.CustomDomain, len(rows))
	for i, d := range rows {
		out[i] = customDomainToAPI(&d)
	}
	return c.JSON(out)
}

// RequestCustomDomain opens a pending row and returns the token the
// user needs to serve at /.well-known/pingcast/<token>. Pro-gated.
func (s *Server) RequestCustomDomain(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	var req apigen.RequestCustomDomainJSONRequestBody
	if err := c.BodyParser(&req); err != nil {
		return httperr.WriteMalformedJSON(c)
	}
	d, err := s.customDomains.RequestDomain(c.UserContext(), user.ID, req.Hostname)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return httperr.WriteForbiddenTenant(c)
		}
		return c.Status(fiber.StatusUnprocessableEntity).JSON(httperr.Envelope{
			Error: httperr.Inner{Code: "INVALID_HOSTNAME", Message: err.Error()},
		})
	}
	return c.Status(fiber.StatusCreated).JSON(customDomainToAPI(d))
}

// DeleteCustomDomain removes a pending or active domain. Pro-gated.
func (s *Server) DeleteCustomDomain(c *fiber.Ctx, id int64) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	if err := s.customDomains.DeleteDomain(c.UserContext(), id, user.ID); err != nil {
		return httperr.Write(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func customDomainToAPI(d *domain.CustomDomain) apigen.CustomDomain {
	status := apigen.CustomDomainStatus(d.Status)
	return apigen.CustomDomain{
		Id:              d.ID,
		Hostname:        d.Hostname,
		ValidationToken: d.ValidationToken,
		Status:          status,
		LastError:       d.LastError,
		DnsValidatedAt:  d.DNSValidatedAt,
		CertIssuedAt:    d.CertIssuedAt,
		CreatedAt:       d.CreatedAt,
	}
}

// Ensure the app package is imported somewhere in this file so the
// anonymous reference survives linting if the explicit calls above go
// through a type alias later.
var _ = (*app.CustomDomainService)(nil)
