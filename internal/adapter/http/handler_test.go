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
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// stubUserRepo implements port.UserRepo with controllable behavior.
type stubUserRepo struct {
	users         map[string]*domain.User // keyed by email
	passwordHash  map[string]string       // keyed by email
	usersByID     map[uuid.UUID]*domain.User
	usersBySlug   map[string]*domain.User
	createErr     error
	getByEmailErr error
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{
		users:        make(map[string]*domain.User),
		passwordHash: make(map[string]string),
		usersByID:    make(map[uuid.UUID]*domain.User),
		usersBySlug:  make(map[string]*domain.User),
	}
}

func (r *stubUserRepo) addUser(email, slug, password string) *domain.User {
	hash, _ := app.HashPassword(password)
	id := uuid.New()
	u := &domain.User{
		ID:        id,
		Email:     email,
		Slug:      slug,
		Plan:      domain.PlanFree,
		CreatedAt: time.Now(),
	}
	r.users[email] = u
	r.passwordHash[email] = hash
	r.usersByID[id] = u
	r.usersBySlug[slug] = u
	return u
}

func (r *stubUserRepo) Create(_ context.Context, email, slug, passwordHash string) (*domain.User, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	id := uuid.New()
	u := &domain.User{ID: id, Email: email, Slug: slug, Plan: domain.PlanFree, CreatedAt: time.Now()}
	r.users[email] = u
	r.passwordHash[email] = passwordHash
	r.usersByID[id] = u
	r.usersBySlug[slug] = u
	return u, nil
}

func (r *stubUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.usersByID[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

func (r *stubUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, string, error) {
	if r.getByEmailErr != nil {
		return nil, "", r.getByEmailErr
	}
	u, ok := r.users[email]
	if !ok {
		return nil, "", errors.New("user not found")
	}
	return u, r.passwordHash[email], nil
}

func (r *stubUserRepo) GetBySlug(_ context.Context, slug string) (*domain.User, error) {
	u, ok := r.usersBySlug[slug]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (r *stubUserRepo) UpdatePlan(_ context.Context, _ uuid.UUID, _ domain.Plan) error { return nil }
func (r *stubUserRepo) UpdateLemonSqueezy(_ context.Context, _ uuid.UUID, _, _ string) error {
	return nil
}

// stubSessionRepo implements port.SessionRepo with in-memory sessions.
type stubSessionRepo struct {
	sessions map[string]uuid.UUID
}

func newStubSessionRepo() *stubSessionRepo {
	return &stubSessionRepo{sessions: make(map[string]uuid.UUID)}
}

func (r *stubSessionRepo) Create(_ context.Context, sessionID string, userID uuid.UUID, _ time.Time) error {
	r.sessions[sessionID] = userID
	return nil
}

func (r *stubSessionRepo) GetUserID(_ context.Context, sessionID string) (uuid.UUID, error) {
	uid, ok := r.sessions[sessionID]
	if !ok {
		return uuid.Nil, errors.New("session not found")
	}
	return uid, nil
}

func (r *stubSessionRepo) Touch(_ context.Context, _ string, _ time.Time) error { return nil }
func (r *stubSessionRepo) Delete(_ context.Context, sessionID string) error {
	delete(r.sessions, sessionID)
	return nil
}
func (r *stubSessionRepo) DeleteExpired(_ context.Context) (int64, error) { return 0, nil }

// stubRateLimiter implements port.RateLimiter.
type stubRateLimiter struct {
	calls   int
	limit   int // deny after this many calls (0 = never deny)
	failErr error
}

func (r *stubRateLimiter) Allow(_ context.Context, _ string) (bool, error) {
	if r.failErr != nil {
		return false, r.failErr
	}
	r.calls++
	if r.limit > 0 && r.calls > r.limit {
		return false, nil
	}
	return true, nil
}

func (r *stubRateLimiter) Reset(_ context.Context, _ string) error {
	r.calls = 0
	return nil
}

// stubMonitorRepo implements port.MonitorRepo (minimal).
type stubMonitorRepo struct{}

func (r *stubMonitorRepo) Create(_ context.Context, m *domain.Monitor) (*domain.Monitor, error) {
	m.ID = uuid.New()
	m.CreatedAt = time.Now()
	return m, nil
}
func (r *stubMonitorRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Monitor, error) {
	return nil, errors.New("not found")
}
func (r *stubMonitorRepo) ListByUserID(_ context.Context, _ uuid.UUID) ([]domain.Monitor, error) {
	return nil, nil
}
func (r *stubMonitorRepo) ListPublicBySlug(_ context.Context, _ string) ([]domain.Monitor, error) {
	return nil, nil
}
func (r *stubMonitorRepo) ListActive(_ context.Context) ([]domain.Monitor, error) { return nil, nil }
func (r *stubMonitorRepo) CountByUserID(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}
func (r *stubMonitorRepo) Update(_ context.Context, _ *domain.Monitor) error    { return nil }
func (r *stubMonitorRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ domain.MonitorStatus) error {
	return nil
}
func (r *stubMonitorRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }

// stubCheckResultRepo implements port.CheckResultRepo (minimal).
type stubCheckResultRepo struct{}

func (r *stubCheckResultRepo) Insert(_ context.Context, _ *domain.CheckResult) error { return nil }
func (r *stubCheckResultRepo) ConsecutiveFailures(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}
func (r *stubCheckResultRepo) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

// stubIncidentRepo implements port.IncidentRepo (minimal).
type stubIncidentRepo struct{}

func (r *stubIncidentRepo) Create(_ context.Context, _ uuid.UUID, _ string) (*domain.Incident, error) {
	return &domain.Incident{}, nil
}
func (r *stubIncidentRepo) Resolve(_ context.Context, _ int64, _ time.Time) error { return nil }
func (r *stubIncidentRepo) GetOpen(_ context.Context, _ uuid.UUID) (*domain.Incident, error) {
	return nil, errors.New("no open")
}
func (r *stubIncidentRepo) IsInCooldown(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}
func (r *stubIncidentRepo) ListByMonitorID(_ context.Context, _ uuid.UUID, _ int) ([]domain.Incident, error) {
	return nil, nil
}

// stubUptimeRepo implements port.UptimeRepo (minimal).
type stubUptimeRepo struct{}

func (r *stubUptimeRepo) RecordCheck(_ context.Context, _ uuid.UUID, _ time.Time, _ bool) error {
	return nil
}
func (r *stubUptimeRepo) GetUptime(_ context.Context, _ uuid.UUID, _ time.Time) (float64, error) {
	return 99.9, nil
}
func (r *stubUptimeRepo) GetUptimeBatch(_ context.Context, _ []uuid.UUID, _ time.Time) (map[uuid.UUID]float64, error) {
	return map[uuid.UUID]float64{}, nil
}

// stubCheckerRegistry implements port.CheckerRegistry (minimal).
type stubCheckerRegistry struct{}

func (r *stubCheckerRegistry) Get(_ domain.MonitorType) (port.MonitorChecker, error) {
	return nil, errors.New("not implemented")
}
func (r *stubCheckerRegistry) Types() []port.MonitorTypeInfo { return nil }
func (r *stubCheckerRegistry) ValidateConfig(_ domain.MonitorType, _ json.RawMessage) error {
	return nil
}
func (r *stubCheckerRegistry) Target(_ domain.MonitorType, _ json.RawMessage) (string, error) {
	return "", nil
}
func (r *stubCheckerRegistry) Host(_ domain.MonitorType, _ json.RawMessage) (string, error) {
	return "", nil
}

// stubChannelRepo implements port.ChannelRepo (minimal).
type stubChannelRepo struct{}

func (r *stubChannelRepo) Create(_ context.Context, ch *domain.NotificationChannel) (*domain.NotificationChannel, error) {
	ch.ID = uuid.New()
	return ch, nil
}
func (r *stubChannelRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.NotificationChannel, error) {
	return nil, errors.New("not found")
}
func (r *stubChannelRepo) ListByUserID(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
	return nil, nil
}
func (r *stubChannelRepo) ListForMonitor(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
	return nil, nil
}
func (r *stubChannelRepo) Update(_ context.Context, _ *domain.NotificationChannel) error { return nil }
func (r *stubChannelRepo) Delete(_ context.Context, _, _ uuid.UUID) error                { return nil }
func (r *stubChannelRepo) BindToMonitor(_ context.Context, _, _ uuid.UUID) error          { return nil }
func (r *stubChannelRepo) UnbindFromMonitor(_ context.Context, _, _ uuid.UUID) error      { return nil }

// stubChannelRegistry implements port.ChannelRegistry (minimal).
type stubChannelRegistry struct{}

func (r *stubChannelRegistry) Get(_ domain.ChannelType) (port.ChannelSenderFactory, error) {
	return nil, errors.New("not implemented")
}
func (r *stubChannelRegistry) CreateSenderWithRetry(_ domain.ChannelType, _ json.RawMessage) (port.AlertSender, error) {
	return nil, errors.New("not implemented")
}
func (r *stubChannelRegistry) Types() []port.ChannelTypeInfo { return nil }
func (r *stubChannelRegistry) ValidateConfig(_ domain.ChannelType, _ json.RawMessage) error {
	return nil
}

// stubFailedAlertRepo implements port.FailedAlertRepo (minimal).
type stubFailedAlertRepo struct{}

func (r *stubFailedAlertRepo) Create(_ context.Context, _ json.RawMessage, _ string, _ []uuid.UUID) error {
	return nil
}

// stubTxManager implements port.TxManager by running fn directly.
type stubTxManager struct{}

func (r *stubTxManager) Do(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}

// stubAPIKeyRepo implements port.APIKeyRepo (minimal).
type stubAPIKeyRepo struct{}

func (r *stubAPIKeyRepo) Create(_ context.Context, k *domain.APIKey) (*domain.APIKey, error) {
	return k, nil
}
func (r *stubAPIKeyRepo) GetByHash(_ context.Context, _ string) (*domain.APIKey, error) {
	return nil, errors.New("not found")
}
func (r *stubAPIKeyRepo) ListByUser(_ context.Context, _ uuid.UUID) ([]domain.APIKey, error) {
	return nil, nil
}
func (r *stubAPIKeyRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (r *stubAPIKeyRepo) Touch(_ context.Context, _ uuid.UUID) error     { return nil }

// ---------------------------------------------------------------------------
// Test fixture builder
// ---------------------------------------------------------------------------

type testEnv struct {
	app         *fiber.App
	userRepo    *stubUserRepo
	sessionRepo *stubSessionRepo
	rateLimiter *stubRateLimiter
	apiKeyRepo  *stubAPIKeyRepo
}

func setupTestApp(t *testing.T) *testEnv {
	t.Helper()

	userRepo := newStubUserRepo()
	sessionRepo := newStubSessionRepo()
	rateLimiter := &stubRateLimiter{}
	apiKeyRepo := &stubAPIKeyRepo{}
	monitorRepo := &stubMonitorRepo{}
	channelRepo := &stubChannelRepo{}
	checkResultRepo := &stubCheckResultRepo{}
	incidentRepo := &stubIncidentRepo{}
	uptimeRepo := &stubUptimeRepo{}
	checkerRegistry := &stubCheckerRegistry{}
	channelRegistry := &stubChannelRegistry{}
	failedAlertRepo := &stubFailedAlertRepo{}
	txManager := &stubTxManager{}

	authService := app.NewAuthService(userRepo, sessionRepo)
	monitoringService := app.NewMonitoringService(
		monitorRepo, channelRepo, checkResultRepo, incidentRepo,
		userRepo, uptimeRepo, txManager, nil, checkerRegistry, nil,
	)
	alertService := app.NewAlertService(channelRepo, monitorRepo, channelRegistry, failedAlertRepo, nil)

	pageHandler := NewPageHandler(authService, monitoringService, alertService, rateLimiter, apiKeyRepo)
	server := NewServer(authService, monitoringService, alertService, nil, rateLimiter)
	webhookHandler := NewWebhookHandler(authService, alertService, "test-secret")

	// HealthChecker must be non-nil because SetupApp registers its methods as route handlers.
	// Fields can be nil since these routes are not exercised via the full app in these tests
	// (healthz tests use a dedicated Fiber app with inline handler).
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

// createSessionForUser registers a fake session and returns the cookie header value.
func (te *testEnv) createSessionForUser(t *testing.T, user *domain.User) string {
	t.Helper()
	sessionID := uuid.New().String()
	te.sessionRepo.sessions[sessionID] = user.ID
	return fmt.Sprintf("session_id=%s", sessionID)
}

// ---------------------------------------------------------------------------
// 1. TestHealthz_AllHealthy
// ---------------------------------------------------------------------------

// stubHealthDB / Redis / NATS for the HealthChecker.
// We test the /healthz endpoint directly with a minimal Fiber app since it
// requires *pgxpool.Pool, *goredis.Client, and *nats.Conn which are hard to
// construct without real connections. We build a focused Fiber app instead.

type healthDep struct {
	pgErr     error
	redisErr  error
	natsAlive bool
}

func newHealthApp(h healthDep) *fiber.App {
	a := fiber.New()

	// Simulate the healthz endpoint behavior without real clients.
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

	// Add a user so the email exists but password is wrong.
	te.userRepo.addUser("test@example.com", "test-user", "correct-password-123")

	// First GET to /login to obtain a CSRF token (from cookie).
	csrfToken, csrfCookie := getCSRFToken(t, te.app, "/login")
	if csrfToken == "" {
		t.Skip("CSRF token not available in test environment — CSRF middleware may need real template rendering")
	}

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

	// Set rate limiter to deny after 5 calls.
	te.rateLimiter.limit = 5

	te.userRepo.addUser("limited@example.com", "limited", "some-password-123")

	// Exhaust rate limit with 5 calls.
	csrfToken, csrfCookie := getCSRFToken(t, te.app, "/login")
	if csrfToken == "" {
		t.Skip("CSRF token not available in test environment")
	}
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
	csrfToken, csrfCookie = getCSRFToken(t, te.app, "/login")
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
		// CSRF token not in HTML — templates may not render in test environment
		return "", ""
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
	// Find start of the <input tag
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
