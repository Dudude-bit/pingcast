//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §4 (Health): /health, /healthz, /readyz — all public.

func TestHealth_Returns200(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/health")
	harness.AssertStatus(t, resp, 200)
}

func TestHealthz_Returns200(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/healthz")
	harness.AssertStatus(t, resp, 200)
}

func TestReadyz_WithAllDeps_Returns200(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/readyz")
	harness.AssertStatus(t, resp, 200)
}
