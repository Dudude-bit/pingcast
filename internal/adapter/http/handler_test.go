package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	smtpadapter "github.com/kirillinakin/pingcast/internal/adapter/smtp"
	"github.com/kirillinakin/pingcast/internal/adapter/sysclock"
	"github.com/kirillinakin/pingcast/internal/adapter/sysrand"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/mocks"
	"github.com/kirillinakin/pingcast/internal/port"
)

// ---------------------------------------------------------------------------
// Test fixture builder
// ---------------------------------------------------------------------------

type testEnv struct {
	app         *fiber.App
	userRepo    *mocks.MockUserRepo
	sessionRepo *mocks.MockSessionRepo
	rateLimiter *mocks.MockRateLimiter
	apiKeyRepo  *mocks.MockAPIKeyRepo
}

func setupTestApp(t *testing.T) *testEnv {
	t.Helper()

	userRepo := mocks.NewMockUserRepo(t)
	sessionRepo := mocks.NewMockSessionRepo(t)
	rateLimiter := mocks.NewMockRateLimiter(t)
	apiKeyRepo := mocks.NewMockAPIKeyRepo(t)
	// All five scoped limiters share the same mock — existing tests
	// only care that limiter.Allow returns true; the scoping is exercised
	// by the integration suite, not this unit fixture.
	rls := &port.RateLimiters{
		Register: rateLimiter,
		Login:    rateLimiter,
		Status:   rateLimiter,
		Write:    rateLimiter,
		Read:     rateLimiter,
	}
	monitorRepo := mocks.NewMockMonitorRepo(t)
	channelRepo := mocks.NewMockChannelRepo(t)
	checkResultRepo := mocks.NewMockCheckResultRepo(t)
	incidentRepo := mocks.NewMockIncidentRepo(t)
	uptimeRepo := mocks.NewMockUptimeRepo(t)
	checkerRegistry := mocks.NewMockCheckerRegistry(t)
	channelRegistry := mocks.NewMockChannelRegistry(t)
	failedAlertRepo := mocks.NewMockFailedAlertRepo(t)
	txManager := mocks.NewMockTxManager(t)
	alertPub := mocks.NewMockAlertEventPublisher(t)
	metrics := mocks.NewMockMetrics(t)
	eventPub := mocks.NewMockMonitorEventPublisher(t)

	authService := app.NewAuthService(userRepo, sessionRepo, sysclock.New(), sysrand.New())
	incidentUpdateRepo := mocks.NewMockIncidentUpdateRepo(t)
	maintenanceRepo := mocks.NewMockMaintenanceWindowRepo(t)
	monitorGroupRepo := mocks.NewMockMonitorGroupRepo(t)
	monitoringService := app.NewMonitoringService(
		monitorRepo, channelRepo, checkResultRepo, incidentRepo, incidentUpdateRepo, maintenanceRepo, monitorGroupRepo,
		userRepo, uptimeRepo, txManager, alertPub, eventPub, checkerRegistry, metrics,
		sysclock.New(),
	)
	alertService := app.NewAlertService(channelRepo, monitorRepo, channelRegistry, failedAlertRepo, metrics)

	statsRepo := mocks.NewMockStatsRepo(t)
	billingService := app.NewBillingService(userRepo, 100)
	atlassianImporter := app.NewAtlassianImporter(monitorRepo, incidentRepo, incidentUpdateRepo, txManager, sysclock.New())
	statusSubRepo := mocks.NewMockStatusSubscriberRepo(t)
	blogSubRepo := mocks.NewMockBlogSubscriberRepo(t)
	mailerStub := smtpadapter.NewMailer("", 0, "", "", "")
	subscriptionsService := app.NewSubscriptionService(statusSubRepo, mailerStub, "http://test")
	blogSubscriptionsService := app.NewBlogSubscriptionService(blogSubRepo, mailerStub, "http://test")
	customDomainRepo := mocks.NewMockCustomDomainRepo(t)
	customDomainsService := app.NewCustomDomainService(customDomainRepo, app.NoopCertProvisioner{}, "http://test")
	server := NewServer(authService, monitoringService, alertService, billingService, atlassianImporter, subscriptionsService, blogSubscriptionsService, customDomainsService, rls, apiKeyRepo, statsRepo)
	webhookHandler := NewWebhookHandler(authService, alertService, billingService, "test-secret", "")

	healthChecker := &HealthChecker{}
	fiberApp := SetupApp(authService, server, webhookHandler, apiKeyRepo, healthChecker, rls)

	return &testEnv{
		app:         fiberApp,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		rateLimiter: rateLimiter,
		apiKeyRepo:  apiKeyRepo,
	}
}

// ---------------------------------------------------------------------------
// 1. TestHealthz_AllHealthy
// ---------------------------------------------------------------------------

type healthDep struct {
	pgErr     error
	redisErr  error
	natsAlive bool
}

func newHealthApp(h healthDep) *fiber.App {
	a := fiber.New()

	a.Get("/healthz", func(c *fiber.Ctx) error {
		checks := map[string]string{}
		healthy := true

		if h.pgErr != nil {
			checks["postgres"] = "unhealthy: " + h.pgErr.Error()
			healthy = false
		} else {
			checks["postgres"] = "ok"
		}
		if h.redisErr != nil {
			checks["redis"] = "unhealthy: " + h.redisErr.Error()
			healthy = false
		} else {
			checks["redis"] = "ok"
		}
		if !h.natsAlive {
			checks["nats"] = "unhealthy: disconnected"
			healthy = false
		} else {
			checks["nats"] = "ok"
		}

		status := fiber.StatusOK
		if !healthy {
			status = fiber.StatusServiceUnavailable
		}
		return c.Status(status).JSON(fiber.Map{
			"status": map[bool]string{true: "healthy", false: "unhealthy"}[healthy],
			"checks": checks,
		})
	})
	return a
}

func TestHealthz_AllHealthy(t *testing.T) {
	a := newHealthApp(healthDep{pgErr: nil, redisErr: nil, natsAlive: true})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("expected status=healthy, got %v", result["status"])
	}

	checks, _ := result["checks"].(map[string]any)
	for dep, val := range checks {
		if val != "ok" {
			t.Errorf("expected %s=ok, got %v", dep, val)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. TestHealthz_PostgresDown
// ---------------------------------------------------------------------------

func TestHealthz_PostgresDown(t *testing.T) {
	a := newHealthApp(healthDep{pgErr: errors.New("connection refused"), redisErr: nil, natsAlive: true})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 503 {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result["status"] != "unhealthy" {
		t.Errorf("expected status=unhealthy, got %v", result["status"])
	}

	checks, _ := result["checks"].(map[string]any)
	pgStatus, _ := checks["postgres"].(string)
	if !strings.Contains(pgStatus, "unhealthy") {
		t.Errorf("expected postgres to be unhealthy, got %q", pgStatus)
	}
}

// Tests 3–6 (register/login page & submit HTML flows) were removed in C1
// when these routes migrated to the Next.js frontend. The JSON API
// endpoints /api/auth/register and /api/auth/login are covered by the
// Playwright E2E suite in frontend/tests/auth.spec.ts.

// Test 7 (Dashboard unauth redirect) removed in C2 — /dashboard is now
// served by Next.js; auth redirect logic lives in frontend/proxy.ts.

// TestCSRF_MissingToken was removed alongside the CSRF middleware:
// all browser routes migrated to Next.js + JSON API, so there are no
// form-POST targets to guard (spec §8.6).

// ---------------------------------------------------------------------------
// 9. TestAPIAuth_InvalidAPIKey
// ---------------------------------------------------------------------------

func TestAPIAuth_InvalidAPIKey(t *testing.T) {
	te := setupTestApp(t)

	// No session.
	te.sessionRepo.EXPECT().GetUserID(mock.Anything, mock.Anything).Return(uuid.Nil, errors.New("no session")).Maybe()

	// API key lookup returns not found.
	te.apiKeyRepo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(nil, errors.New("not found")).Maybe()

	req := httptest.NewRequest(http.MethodGet, "/api/monitors", nil)
	req.Header.Set("Authorization", "Bearer pc_live_invalidkey1234567890abcdef1234567890abcdef1234567890abcdef")

	resp, err := te.app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "UNAUTHORIZED") {
		t.Errorf("expected canonical UNAUTHORIZED envelope, got: %s", truncate(bodyStr, 300))
	}
}

// ---------------------------------------------------------------------------
// 10. TestErrorHandler_DomainError
// ---------------------------------------------------------------------------

func TestErrorHandler_DomainError(t *testing.T) {
	// Build a minimal Fiber app with the same error handler from SetupApp
	// and a route that returns a domain.ErrNotFound.
	a := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			var domErr *domain.DomainError
			if errors.As(err, &domErr) {
				code := fiber.StatusInternalServerError
				switch {
				case errors.Is(domErr, domain.ErrNotFound):
					code = fiber.StatusNotFound
				case errors.Is(domErr, domain.ErrForbidden):
					code = fiber.StatusForbidden
				case errors.Is(domErr, domain.ErrValidation):
					code = fiber.StatusUnprocessableEntity
				case errors.Is(domErr, domain.ErrConflict):
					code = fiber.StatusConflict
				}
				return c.Status(code).JSON(fiber.Map{
					"error": fiber.Map{
						"code":    domErr.Code,
						"message": domErr.Message,
					},
				})
			}
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "INTERNAL_ERROR",
					"message": err.Error(),
				},
			})
		},
	})

	a.Get("/trigger-not-found", func(c *fiber.Ctx) error {
		return domain.NewNotFoundError("MONITOR_NOT_FOUND", "monitor not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/trigger-not-found", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content type, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in response, got: %s", string(body))
	}
	if errObj["code"] != "MONITOR_NOT_FOUND" {
		t.Errorf("expected code=MONITOR_NOT_FOUND, got %v", errObj["code"])
	}
	if errObj["message"] != "monitor not found" {
		t.Errorf("expected message='monitor not found', got %v", errObj["message"])
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// truncate returns up to n characters of s, for readable error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
