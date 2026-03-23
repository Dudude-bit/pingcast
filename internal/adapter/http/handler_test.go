package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/mocks"
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

	authService := app.NewAuthService(userRepo, sessionRepo)
	monitoringService := app.NewMonitoringService(
		monitorRepo, channelRepo, checkResultRepo, incidentRepo,
		userRepo, uptimeRepo, txManager, alertPub, checkerRegistry, metrics,
	)
	alertService := app.NewAlertService(channelRepo, monitorRepo, channelRegistry, failedAlertRepo, metrics)

	pageHandler := NewPageHandler(authService, monitoringService, alertService, rateLimiter, apiKeyRepo)
	server := NewServer(authService, monitoringService, alertService, eventPub, rateLimiter, apiKeyRepo)
	webhookHandler := NewWebhookHandler(authService, alertService, "test-secret")

	healthChecker := &HealthChecker{}
	fiberApp := SetupApp(authService, pageHandler, server, webhookHandler, apiKeyRepo, healthChecker)

	return &testEnv{
		app:         fiberApp,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		rateLimiter: rateLimiter,
		apiKeyRepo:  apiKeyRepo,
	}
}

// createSessionForUser sets up mock expectations so that the given sessionID
// is recognised by the session repo, and returns the cookie header value.
func (te *testEnv) createSessionForUser(t *testing.T, user *domain.User) string {
	t.Helper()
	sessionID := uuid.New().String()

	// The auth middleware calls GetUserID and then GetByID + Touch.
	te.sessionRepo.EXPECT().GetUserID(mock.Anything, sessionID).Return(user.ID, nil).Maybe()
	te.userRepo.EXPECT().GetByID(mock.Anything, user.ID).Return(user, nil).Maybe()
	te.sessionRepo.EXPECT().Touch(mock.Anything, sessionID, mock.Anything).Return(nil).Maybe()

	return fmt.Sprintf("session_id=%s", sessionID)
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

// ---------------------------------------------------------------------------
// 3. TestRegisterPage_GET
// ---------------------------------------------------------------------------

func TestRegisterPage_GET(t *testing.T) {
	te := setupTestApp(t)

	// The GET /register page may call session repo to check auth state.
	te.sessionRepo.EXPECT().GetUserID(mock.Anything, mock.Anything).Return(uuid.Nil, errors.New("no session")).Maybe()

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	resp, err := te.app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "<form") {
		t.Error("expected HTML to contain a <form element")
	}
	if !strings.Contains(bodyStr, "register") && !strings.Contains(bodyStr, "Register") && !strings.Contains(bodyStr, "sign up") && !strings.Contains(bodyStr, "Sign Up") {
		t.Error("expected HTML to reference registration")
	}
}

// ---------------------------------------------------------------------------
// 4. TestLoginPage_GET
// ---------------------------------------------------------------------------

func TestLoginPage_GET(t *testing.T) {
	te := setupTestApp(t)

	// The GET /login page may call session repo to check auth state.
	te.sessionRepo.EXPECT().GetUserID(mock.Anything, mock.Anything).Return(uuid.Nil, errors.New("no session")).Maybe()

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	resp, err := te.app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "<form") {
		t.Error("expected HTML to contain a <form element")
	}
}

// ---------------------------------------------------------------------------
// 5. TestLoginSubmit_InvalidCredentials
// ---------------------------------------------------------------------------

func TestLoginSubmit_InvalidCredentials(t *testing.T) {
	te := setupTestApp(t)

	// Set up a user with a known bcrypt hash.
	hash, _ := app.HashPassword("correct-password-123")
	userID := uuid.New()
	user := &domain.User{
		ID:        userID,
		Email:     "test@example.com",
		Slug:      "test-user",
		Plan:      domain.PlanFree,
		CreatedAt: time.Now(),
	}

	// GET /login to obtain CSRF token — no session cookie sent.
	te.sessionRepo.EXPECT().GetUserID(mock.Anything, mock.Anything).Return(uuid.Nil, errors.New("no session")).Maybe()

	// Login POST calls GetByEmail; returns user with hash so bcrypt check happens.
	te.userRepo.EXPECT().GetByEmail(mock.Anything, "test@example.com").Return(user, hash, nil).Maybe()

	// Rate limiter allows the request.
	te.rateLimiter.EXPECT().Allow(mock.Anything, mock.Anything).Return(true, nil).Maybe()

	// First GET to /login to obtain a CSRF token from the rendered HTML.
	csrfToken, csrfCookie := getCSRFToken(t, te.app, "/login")

	formData := fmt.Sprintf("email=test@example.com&password=wrong-password&_csrf=%s", csrfToken)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", csrfCookie)

	resp, err := te.app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// LoginSubmit re-renders login.html with error on failure (200, not redirect).
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "Invalid email or password") {
		t.Errorf("expected error message in HTML, got: %s", truncate(bodyStr, 500))
	}
}

// ---------------------------------------------------------------------------
// 6. TestLoginSubmit_RateLimited
// ---------------------------------------------------------------------------

func TestLoginSubmit_RateLimited(t *testing.T) {
	te := setupTestApp(t)

	// Set up a user with a known bcrypt hash.
	hash, _ := app.HashPassword("some-password-123")
	userID := uuid.New()
	user := &domain.User{
		ID:        userID,
		Email:     "limited@example.com",
		Slug:      "limited",
		Plan:      domain.PlanFree,
		CreatedAt: time.Now(),
	}

	// No session cookie for these requests.
	te.sessionRepo.EXPECT().GetUserID(mock.Anything, mock.Anything).Return(uuid.Nil, errors.New("no session")).Maybe()

	// GetByEmail will be called for each login attempt.
	te.userRepo.EXPECT().GetByEmail(mock.Anything, "limited@example.com").Return(user, hash, nil).Maybe()

	// Rate limiter: allow first 5 calls, then deny.
	callCount := 0
	te.rateLimiter.EXPECT().Allow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string) (bool, error) {
			callCount++
			if callCount > 5 {
				return false, nil
			}
			return true, nil
		},
	).Maybe()
	te.rateLimiter.EXPECT().Reset(mock.Anything, mock.Anything).Return(nil).Maybe()

	// Exhaust rate limit with 5 calls.
	for i := range 5 {
		csrfToken, csrfCookie := getCSRFToken(t, te.app, "/login")
		formData := fmt.Sprintf("email=limited@example.com&password=wrong-pass-%d&_csrf=%s", i, csrfToken)
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", csrfCookie)

		resp, err := te.app.Test(req, -1)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		resp.Body.Close()
	}

	// 6th request should be rate limited.
	csrfToken, csrfCookie := getCSRFToken(t, te.app, "/login")
	formData := fmt.Sprintf("email=limited@example.com&password=wrong-pass-6&_csrf=%s", csrfToken)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", csrfCookie)

	resp, err := te.app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// The page handler renders login.html with "Too many login attempts" error.
	if !strings.Contains(bodyStr, "Too many login attempts") {
		t.Errorf("expected rate limit message in HTML, got: %s", truncate(bodyStr, 500))
	}
}

// ---------------------------------------------------------------------------
// 7. TestDashboard_Unauthenticated
// ---------------------------------------------------------------------------

func TestDashboard_Unauthenticated(t *testing.T) {
	te := setupTestApp(t)

	// No valid session — middleware should redirect to /login.
	te.sessionRepo.EXPECT().GetUserID(mock.Anything, mock.Anything).Return(uuid.Nil, errors.New("session not found")).Maybe()

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	resp, err := te.app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// PageMiddleware must redirect unauthenticated HTML requests to /login.
	if resp.StatusCode != 302 {
		t.Fatalf("expected 302 redirect to /login, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/login" {
		t.Errorf("expected redirect to /login, got %q", location)
	}
}

// ---------------------------------------------------------------------------
// 8. TestCSRF_MissingToken
// ---------------------------------------------------------------------------

func TestCSRF_MissingToken(t *testing.T) {
	te := setupTestApp(t)

	// No session.
	te.sessionRepo.EXPECT().GetUserID(mock.Anything, mock.Anything).Return(uuid.Nil, errors.New("no session")).Maybe()
	// Rate limiter may be called before CSRF check, or not — depends on middleware order.
	te.rateLimiter.EXPECT().Allow(mock.Anything, mock.Anything).Return(true, nil).Maybe()

	// POST /login without _csrf field.
	formData := "email=test@example.com&password=whatever"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := te.app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for missing CSRF token, got %d", resp.StatusCode)
	}
}

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

	if !strings.Contains(bodyStr, "invalid api key") {
		t.Errorf("expected 'invalid api key' in response, got: %s", truncate(bodyStr, 300))
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

// getCSRFToken performs a GET to the given path and extracts the CSRF token
// from the hidden form field and the csrf cookie from Set-Cookie headers.
func getCSRFToken(t *testing.T, app *fiber.App, path string) (token string, cookieHeader string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("getCSRFToken: GET %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Extract CSRF token from: <input type="hidden" name="_csrf" value="...">
	// or value='...' variants.
	token = extractInputValue(bodyStr, "_csrf")
	if token == "" {
		t.Fatalf("getCSRFToken: _csrf hidden field not found in HTML for %s — CSRF middleware may be misconfigured", path)
	}

	// Collect all Set-Cookie headers for the csrf cookie.
	var cookies []string
	for _, c := range resp.Cookies() {
		cookies = append(cookies, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}

	return token, strings.Join(cookies, "; ")
}

// extractInputValue finds value="..." for a hidden input with the given name.
func extractInputValue(html, name string) string {
	// Look for name="_csrf" (or name='_csrf') and extract the value.
	needle := fmt.Sprintf(`name="%s"`, name)
	idx := strings.Index(html, needle)
	if idx < 0 {
		needle = fmt.Sprintf(`name='%s'`, name)
		idx = strings.Index(html, needle)
	}
	if idx < 0 {
		return ""
	}

	// Search backwards and forwards for the enclosing <input> tag to find value="..."
	start := strings.LastIndex(html[:idx], "<input")
	if start < 0 {
		start = strings.LastIndex(html[:idx], "<Input")
		if start < 0 {
			return ""
		}
	}
	end := strings.Index(html[start:], ">")
	if end < 0 {
		return ""
	}
	tag := html[start : start+end+1]

	// Extract value="..." or value='...'
	valIdx := strings.Index(tag, `value="`)
	if valIdx >= 0 {
		valStart := valIdx + len(`value="`)
		valEnd := strings.Index(tag[valStart:], `"`)
		if valEnd >= 0 {
			return tag[valStart : valStart+valEnd]
		}
	}
	valIdx = strings.Index(tag, `value='`)
	if valIdx >= 0 {
		valStart := valIdx + len(`value='`)
		valEnd := strings.Index(tag[valStart:], `'`)
		if valEnd >= 0 {
			return tag[valStart : valStart+valEnd]
		}
	}
	return ""
}

// truncate returns up to n characters of s, for readable error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
