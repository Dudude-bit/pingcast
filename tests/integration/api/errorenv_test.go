//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §1: every non-2xx response uses the envelope
// {"error":{"code":"...","message":"..."}}.

func TestErrorEnvelope_Unauthorized_CanonicalShape(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.GET(t, "/api/monitors")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestErrorEnvelope_MalformedJSON_Returns400WithCode(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.DoRaw(t, "POST", "/api/auth/register", "application/json", []byte("{not valid json"))
	harness.AssertError(t, resp, 400, "MALFORMED_JSON")
}

func TestErrorEnvelope_BusinessValidation_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	// empty password is a business-validation failure, not malformed JSON
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "ok@test.local",
		"slug":     "ok-1",
		"password": "",
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestErrorEnvelope_NotFound_Returns404(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitors/00000000-0000-0000-0000-000000000000")
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}
