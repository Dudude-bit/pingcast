//go:build integration

package api

import (
	"fmt"
	"testing"

	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Each rate-limit test spins up its own isolated App with a tight
// bucket so a burst-of-N exercise completes in milliseconds. The
// shared containers' Redis FLUSHDB in harness.Reset gives each test
// a clean slate.

func TestRateLimit_Register_N_AllowedThen429(t *testing.T) {
	_ = harness.New(t) // init containers + reset state
	app := harness.NewAppWithRateLimits(t, &port.RateLimitConfig{
		RegisterPerHour: 3,
		LoginPer15Min:   100, WritePerMin: 100, ReadPerMin: 100, StatusPerMin: 100,
	})

	s := app.NewSession()
	for i := 0; i < 3; i++ {
		resp := s.POST(t, "/api/auth/register", map[string]any{
			"email":    fmt.Sprintf("burst-%d@test.local", i),
			"slug":     fmt.Sprintf("burst-%d-slug", i),
			"password": "password123",
		})
		if resp.Status != 201 {
			t.Fatalf("register %d: status=%d body=%s", i, resp.Status, resp.Body)
		}
	}

	// N+1 should 429
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "burst-over@test.local",
		"slug":     "burst-over",
		"password": "password123",
	})
	body := harness.AssertError(t, resp, 429, "RATE_LIMITED")
	if body.Error.Code != "RATE_LIMITED" {
		t.Errorf("want RATE_LIMITED, got %q", body.Error.Code)
	}
	if resp.Headers.Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}
}

func TestRateLimit_Login_FailedAttempts_429(t *testing.T) {
	_ = harness.New(t)
	app := harness.NewAppWithRateLimits(t, &port.RateLimitConfig{
		RegisterPerHour: 100, LoginPer15Min: 3,
		WritePerMin: 100, ReadPerMin: 100, StatusPerMin: 100,
	})

	reg := app.NewSession()
	rr := reg.POST(t, "/api/auth/register", map[string]any{
		"email":    "loginbucket@test.local",
		"slug":     "loginbucket-1",
		"password": "password123",
	})
	harness.AssertStatus(t, rr, 201)

	// 3 failed attempts on the same email
	for i := 0; i < 3; i++ {
		s := app.NewSession()
		resp := s.POST(t, "/api/auth/login", map[string]any{
			"email": "loginbucket@test.local", "password": "wrong",
		})
		harness.AssertStatus(t, resp, 401)
	}
	// 4th → 429
	s := app.NewSession()
	resp := s.POST(t, "/api/auth/login", map[string]any{
		"email": "loginbucket@test.local", "password": "wrong",
	})
	harness.AssertError(t, resp, 429, "RATE_LIMITED")
}

func TestRateLimit_Login_ResetOnSuccess(t *testing.T) {
	_ = harness.New(t)
	app := harness.NewAppWithRateLimits(t, &port.RateLimitConfig{
		RegisterPerHour: 100, LoginPer15Min: 3,
		WritePerMin: 100, ReadPerMin: 100, StatusPerMin: 100,
	})

	reg := app.NewSession()
	rr := reg.POST(t, "/api/auth/register", map[string]any{
		"email":    "resetlogin@test.local",
		"slug":     "resetlogin-1",
		"password": "password123",
	})
	harness.AssertStatus(t, rr, 201)

	// 2 failed → bucket at 2/3
	for i := 0; i < 2; i++ {
		s := app.NewSession()
		resp := s.POST(t, "/api/auth/login", map[string]any{
			"email": "resetlogin@test.local", "password": "wrong",
		})
		harness.AssertStatus(t, resp, 401)
	}

	// 1 success → bucket reset
	ok := app.NewSession()
	resp := ok.POST(t, "/api/auth/login", map[string]any{
		"email": "resetlogin@test.local", "password": "password123",
	})
	harness.AssertStatus(t, resp, 200)

	// 3 more failed → should all be 401, not 429 (counter was reset)
	for i := 0; i < 3; i++ {
		s := app.NewSession()
		resp := s.POST(t, "/api/auth/login", map[string]any{
			"email": "resetlogin@test.local", "password": "wrong",
		})
		if resp.Status == 429 {
			t.Fatalf("bucket should have reset on success; got 429 at i=%d", i)
		}
	}
}

func TestRateLimit_Status_N_AllowedThen429(t *testing.T) {
	_ = harness.New(t)
	app := harness.NewAppWithRateLimits(t, &port.RateLimitConfig{
		RegisterPerHour: 100, LoginPer15Min: 100,
		StatusPerMin: 3, WritePerMin: 100, ReadPerMin: 100,
	})

	// Register owner to get a slug
	reg := app.NewSession()
	rr := reg.POST(t, "/api/auth/register", map[string]any{
		"email":    "statusrl@test.local",
		"slug":     "statusrl",
		"password": "password123",
	})
	harness.AssertStatus(t, rr, 201)

	pub := app.NewSession()
	for i := 0; i < 3; i++ {
		resp := pub.GET(t, "/api/status/statusrl")
		if resp.Status != 200 {
			t.Fatalf("status #%d: %d body=%s", i, resp.Status, resp.Body)
		}
	}
	resp := pub.GET(t, "/api/status/statusrl")
	harness.AssertError(t, resp, 429, "RATE_LIMITED")
}

func TestRateLimit_Write_N_AllowedThen429(t *testing.T) {
	_ = harness.New(t)
	app := harness.NewAppWithRateLimits(t, &port.RateLimitConfig{
		RegisterPerHour: 100, LoginPer15Min: 100, StatusPerMin: 100,
		WritePerMin: 3, ReadPerMin: 100,
	})

	s := app.NewSession()
	r := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "writerl@test.local",
		"slug":     "writerl",
		"password": "password123",
	})
	harness.AssertStatus(t, r, 201)

	// 3 writes allowed
	for i := 0; i < 3; i++ {
		resp := s.POST(t, "/api/monitors", map[string]any{
			"name":             fmt.Sprintf("m-%d", i),
			"type":             "http",
			"check_config":     map[string]any{"url": "https://example.com/x", "method": "GET"},
			"interval_seconds": 300,
		})
		harness.AssertStatus(t, resp, 201)
	}

	// 4th write → 429
	resp := s.POST(t, "/api/monitors", map[string]any{
		"name":             "over",
		"type":             "http",
		"check_config":     map[string]any{"url": "https://example.com/x", "method": "GET"},
		"interval_seconds": 300,
	})
	harness.AssertError(t, resp, 429, "RATE_LIMITED")
}

func TestRateLimit_Read_N_AllowedThen429(t *testing.T) {
	_ = harness.New(t)
	app := harness.NewAppWithRateLimits(t, &port.RateLimitConfig{
		RegisterPerHour: 100, LoginPer15Min: 100, StatusPerMin: 100,
		WritePerMin: 100, ReadPerMin: 3,
	})

	s := app.NewSession()
	r := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "readrl@test.local",
		"slug":     "readrl",
		"password": "password123",
	})
	harness.AssertStatus(t, r, 201)

	for i := 0; i < 3; i++ {
		resp := s.GET(t, "/api/monitors")
		if resp.Status != 200 {
			t.Fatalf("read #%d: %d", i, resp.Status)
		}
	}
	resp := s.GET(t, "/api/monitors")
	harness.AssertError(t, resp, 429, "RATE_LIMITED")
}
