//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Spec §4 (Auth): register/login/logout behaviour. All tests assert
// against the canonical envelope for error cases.

func TestRegister_Happy_Returns201WithUser(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "happy@test.local",
		"slug":     "happy-1",
		"password": "password123",
	})
	harness.AssertStatus(t, resp, 201)

	var body struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Slug  string `json:"slug"`
			Plan  string `json:"plan"`
		} `json:"user"`
	}
	resp.JSON(t, &body)

	if body.User.Email != "happy@test.local" {
		t.Errorf("email: got %q", body.User.Email)
	}
	if body.User.Slug != "happy-1" {
		t.Errorf("slug: got %q", body.User.Slug)
	}
	if body.User.Plan != "free" {
		t.Errorf("plan: got %q, want free", body.User.Plan)
	}
	if body.User.ID == "" {
		t.Error("user.id empty")
	}
}

func TestRegister_DuplicateEmail_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()

	body := map[string]any{
		"email":    "dup@test.local",
		"slug":     "dup-1",
		"password": "password123",
	}
	first := s.POST(t, "/api/auth/register", body)
	harness.AssertStatus(t, first, 201)

	body["slug"] = "dup-2" // different slug, same email
	resp := s.POST(t, "/api/auth/register", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestRegister_MalformedJSON_Returns400(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.DoRaw(t, "POST", "/api/auth/register", "application/json", []byte("{"))
	harness.AssertError(t, resp, 400, "MALFORMED_JSON")
}

func TestRegister_ShortPassword_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "short@test.local",
		"slug":     "short-1",
		"password": "abc",
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestRegister_InvalidEmail_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "not-an-email",
		"slug":     "bad-email-1",
		"password": "password123",
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestRegister_InvalidSlug_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	// slug with uppercase — violates [a-z0-9-]{3,30}
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "slug@test.local",
		"slug":     "Slug-Uppercase",
		"password": "password123",
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestRegister_ReservedSlug_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "reserved@test.local",
		"slug":     "admin",
		"password": "password123",
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestLogin_Happy_ReturnsUserAndSetsCookie(t *testing.T) {
	h := harness.New(t)

	reg := h.App.NewSession()
	reg.POST(t, "/api/auth/register", map[string]any{
		"email":    "login@test.local",
		"slug":     "login-1",
		"password": "password123",
	})

	client := h.App.NewSession()
	resp := client.POST(t, "/api/auth/login", map[string]any{
		"email":    "login@test.local",
		"password": "password123",
	})
	harness.AssertStatus(t, resp, 200)

	// Cookie should now authenticate subsequent calls
	me := client.GET(t, "/api/monitors")
	if me.Status == 401 {
		t.Fatalf("login cookie did not persist: %d %s", me.Status, me.Body)
	}
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	h := harness.New(t)
	reg := h.App.NewSession()
	reg.POST(t, "/api/auth/register", map[string]any{
		"email":    "bad@test.local",
		"slug":     "bad-1",
		"password": "password123",
	})

	client := h.App.NewSession()
	resp := client.POST(t, "/api/auth/login", map[string]any{
		"email":    "bad@test.local",
		"password": "wrongpass",
	})
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestLogin_NonexistentEmail_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/login", map[string]any{
		"email":    "nobody@test.local",
		"password": "whatever-123",
	})
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestLogout_Happy_ClearsSession(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	resp := s.POST(t, "/api/auth/logout", nil)
	harness.AssertStatus(t, resp, 204)

	// Cookie cleared by server; subsequent call must be 401
	after := s.GET(t, "/api/monitors")
	harness.AssertError(t, after, 401, "UNAUTHORIZED")
}

func TestLogout_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/logout", nil)
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

// TestGetMe_LiveSession_ReturnsUser is the contract the SSR navbar
// depends on to distinguish a stale cookie from a live session.
func TestGetMe_LiveSession_ReturnsUser(t *testing.T) {
	h := harness.New(t)
	s, user := h.RegisterAndLoginUser(t)

	resp := s.GET(t, "/api/auth/me")
	harness.AssertStatus(t, resp, 200)

	var body struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Slug  string `json:"slug"`
		Plan  string `json:"plan"`
	}
	resp.JSON(t, &body)
	if body.ID != user.ID.String() {
		t.Errorf("id: got %q want %q", body.ID, user.ID)
	}
	if body.Email != user.Email {
		t.Errorf("email: got %q want %q", body.Email, user.Email)
	}
	if body.Plan != "free" {
		t.Errorf("plan: got %q want 'free'", body.Plan)
	}
}

// TestGetMe_NoCookie_Returns401 — visitor with no session_id cookie
// at all must get 401, not a half-authenticated response.
func TestGetMe_NoCookie_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()

	resp := s.GET(t, "/api/auth/me")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

// TestGetMe_StaleCookie_Returns401 — the bug this endpoint exists to
// fix: a session_id from a logged-out / TTL-expired session must
// return 401 so the SSR navbar can stop showing logged-in chrome.
func TestGetMe_StaleCookie_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	s.SetHeader("Cookie", "session_id=this-token-was-never-real")

	resp := s.GET(t, "/api/auth/me")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}
