package httpadapter

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
)

// GetStatusFeed serves an RSS 2.0 feed of incidents for a public
// status page at /status/:slug/feed.xml. Public — same safety profile
// as the status page itself. Cached 5 min at the client; our job is to
// render correct XML, not to compute freshness.
//
// Some of our customers have customers on the other side of the table
// (internal teams, QA contractors, third-party integrators) who won't
// sign up for the email list but will add an RSS reader. This endpoint
// is their channel.
func (s *Server) GetStatusFeed(c *fiber.Ctx) error {
	slug := c.Params("slug")

	data, err := s.monitoring.GetStatusPage(c.UserContext(), slug)
	if err != nil {
		return httperr.WriteNotFound(c, "status page")
	}

	origin := baseOrigin(c)
	channel := rssChannel{
		Title:       fmt.Sprintf("%s — status page", data.Slug),
		Link:        fmt.Sprintf("%s/status/%s", origin, data.Slug),
		Description: fmt.Sprintf("Incident feed for %s. Subscribe with your RSS reader.", data.Slug),
		Language:    "en",
		LastBuild:   time.Now().UTC().Format(time.RFC1123Z),
		Items:       make([]rssItem, 0, len(data.Incidents)),
	}

	for _, inc := range data.Incidents {
		title := inc.Cause
		if inc.Title != nil && *inc.Title != "" {
			title = *inc.Title
		}
		if title == "" {
			title = "Incident"
		}
		description := fmt.Sprintf(
			"State: %s. Started %s.",
			inc.State,
			inc.StartedAt.UTC().Format(time.RFC1123Z),
		)
		if inc.ResolvedAt != nil {
			description += fmt.Sprintf(" Resolved %s.",
				inc.ResolvedAt.UTC().Format(time.RFC1123Z))
		}
		guid := fmt.Sprintf("pingcast-incident-%d", inc.ID)

		channel.Items = append(channel.Items, rssItem{
			Title:       title,
			Link:        fmt.Sprintf("%s/status/%s#incident-%d", origin, data.Slug, inc.ID),
			GUID:        rssGUID{Value: guid, IsPermaLink: "false"},
			PubDate:     inc.StartedAt.UTC().Format(time.RFC1123Z),
			Description: description,
		})
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: channel,
	}

	body, xErr := xml.MarshalIndent(feed, "", "  ")
	if xErr != nil {
		return httperr.Write(c, fmt.Errorf("marshal rss: %w", xErr))
	}

	c.Set(fiber.HeaderContentType, "application/rss+xml; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.Send(append([]byte(xml.Header), body...))
}

// baseOrigin derives the public origin from the incoming request so
// the RSS links work on both pingcast.io and customer custom domains.
// Falls back to the Host header if X-Forwarded-Proto isn't populated.
func baseOrigin(c *fiber.Ctx) string {
	scheme := string(c.Request().URI().Scheme())
	if fwd := c.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	if scheme == "" {
		scheme = "https"
	}
	host := c.Hostname()
	if host == "" {
		host = "pingcast.io"
	}
	return strings.TrimRight(fmt.Sprintf("%s://%s", scheme, host), "/")
}

// --- RSS 2.0 struct shapes ---

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language"`
	LastBuild   string    `xml:"lastBuildDate"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
	Description string  `xml:"description"`
}

type rssGUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}
