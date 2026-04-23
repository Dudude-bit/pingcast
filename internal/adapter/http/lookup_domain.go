package httpadapter

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
)

// LookupCustomDomain maps hostname → slug for the Next.js middleware.
// Public, cached 5min. Consumed once per unique (non-canonical) host,
// so the in-process hostname cache + 5min HTTP cache gives us
// effectively zero Postgres load.
func (s *Server) LookupCustomDomain(c *fiber.Ctx, params apigen.LookupCustomDomainParams) error {
	hostname := strings.ToLower(strings.TrimSpace(params.Hostname))
	if hostname == "" {
		return httperr.WriteNotFound(c, "hostname")
	}

	userID, ok := s.customDomains.LookupHostname(hostname)
	if !ok {
		return httperr.WriteNotFound(c, "custom domain")
	}

	// Slug lives on the user record; we have user_id from the lookup
	// cache and need one more read to get the slug. The auth service
	// already exposes GetUserByID.
	user, err := s.auth.GetUserByID(c.UserContext(), userID)
	if err != nil {
		return httperr.Write(c, err)
	}

	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.JSON(fiber.Map{"slug": user.Slug})
}
