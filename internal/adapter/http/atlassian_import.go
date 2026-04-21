package httpadapter

import (
	"bytes"

	"github.com/gofiber/fiber/v2"

	apigen "github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/app"
)

// ImportAtlassian wraps the AtlassianImporter behind the apigen
// interface. Pro-gated at the setup.go level via proGateSelector (see
// /api/import/atlassian match). A 400 indicates a malformed or
// unsupported export — the importer returns a descriptive error, which
// we surface verbatim (it's safe — the message describes the file the
// user uploaded, not anything sensitive server-side).
func (s *Server) ImportAtlassian(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	body := c.Body()
	res, err := s.atlassianImporter.Import(c.UserContext(), user.ID, bytes.NewReader(body))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(httperr.Envelope{
			Error: httperr.Inner{
				Code:    "ATLASSIAN_IMPORT_FAILED",
				Message: err.Error(),
			},
		})
	}

	//nolint:gosec // G115: counters come from trusted in-process code, always small
	return c.JSON(apigen.AtlassianImportResult{
		MonitorsCreated:   res.MonitorsCreated,
		IncidentsCreated:  res.IncidentsCreated,
		UpdatesCreated:    res.UpdatesCreated,
		ComponentsSkipped: res.ComponentsSkipped,
	})
}

// Compile-time check that the field is wired. See Server in server.go.
var _ = (*app.AtlassianImporter)(nil)
