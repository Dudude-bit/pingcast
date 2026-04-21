package httpadapter

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"

	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
)

// ExportIncidentsCSV streams every incident belonging to the caller as
// a CSV attachment. Pro-gated — registered via auth → RequirePro chain
// in setup.go. Column order:
//
//	id, monitor_id, monitor_name, title, state, is_manual, started_at,
//	resolved_at, cause
//
// Timestamps are ISO-8601 UTC so spreadsheet imports don't trip over
// locale-specific date formats.
func (s *Server) ExportIncidentsCSV(c *fiber.Ctx) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}
	rows, err := s.monitoring.ListIncidentsForExport(c.UserContext(), user.ID)
	if err != nil {
		return httperr.Write(c, err)
	}

	filename := fmt.Sprintf("pingcast-incidents-%s.csv", time.Now().UTC().Format("20060102"))
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Set(fiber.HeaderCacheControl, "no-store")

	// Stream via fasthttp so we're not buffering potentially hundreds
	// of thousands of rows in memory.
	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{
			"id", "monitor_id", "monitor_name", "title", "state",
			"is_manual", "started_at", "resolved_at", "cause",
		})
		for _, r := range rows {
			title := ""
			if r.Title != nil {
				title = *r.Title
			}
			resolved := ""
			if r.ResolvedAt != nil {
				resolved = r.ResolvedAt.UTC().Format(time.RFC3339)
			}
			_ = cw.Write([]string{
				strconv.FormatInt(r.ID, 10),
				r.MonitorID.String(),
				r.MonitorName,
				title,
				string(r.State),
				strconv.FormatBool(r.IsManual),
				r.StartedAt.UTC().Format(time.RFC3339),
				resolved,
				r.Cause,
			})
		}
		cw.Flush()
	}))
	return nil
}
