//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §4 (Public status page): unauthenticated, by slug.

func TestStatusPage_UnknownSlug_Returns404(t *testing.T) {
	h := harness.New(t)
	pub := h.App.NewSession()
	resp := pub.GET(t, "/api/status/no-such-slug")
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}

func TestStatusPage_ValidSlug_Returns200(t *testing.T) {
	h := harness.New(t)
	h.RegisterAndLogin(t, "slug-owner@test.local", "password123")

	// Read the slug from the DB — allowed as a post-setup inspection,
	// not as a shortcut to register without hitting the API.
	var slug string
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT slug FROM users WHERE email=$1`, "slug-owner@test.local").
		Scan(&slug)
	if err != nil {
		t.Fatalf("select slug: %v", err)
	}

	pub := h.App.NewSession()
	resp := pub.GET(t, "/api/status/"+slug)
	harness.AssertStatus(t, resp, 200)
}
