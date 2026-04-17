# B1 Security & Correctness Bugs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship seven targeted security/correctness fixes — TLS hardening on the HTTP checker, removal of a `template.JS` XSS placeholder, a silently-swallowed error, a dead test helper, a Login timing side-channel, an `ErrUserExists` sentinel with enumeration-safe handling, and a generic HTTP error classifier applied to five leaking handlers.

**Architecture:** Each item is localised and independent. Tasks are written TDD-first where the behaviour is testable, expedited (diff-then-verify) where TDD is overkill (dead-code delete, template-text delete). Everything lands in one `fix: B1 — security & correctness bugs` commit at the end; intermediate state is never pushed to `main`.

**Tech Stack:** Go 1.26, `net/smtp`, `crypto/tls`, `golang.org/x/crypto/bcrypt`, `github.com/jackc/pgx/v5/pgconn`, `github.com/gofiber/fiber/v2`, `slog`, `testify/mock`, existing `testcontainers` integration suite.

**Source spec:** `docs/superpowers/specs/2026-04-17-B1-security-correctness-bugs-design.md`

---

## File Structure

Touched files:

- `internal/adapter/checker/http.go` — Task 1
- `internal/adapter/http/pages.go` — Tasks 2, 3
- `internal/web/templates/monitor_detail.html` — Task 2
- `internal/adapter/http/handler_test.go` — Task 4
- `internal/app/auth.go` — Tasks 5, 6
- `internal/domain/errors.go` — Task 6
- `internal/adapter/postgres/errors.go` (new) — Task 6 (extracts `pgUniqueViolation` from `incident_repo.go`)
- `internal/adapter/postgres/incident_repo.go` — Task 6 (imports the extracted constant)
- `internal/adapter/postgres/user_repo.go` — Task 6
- `internal/adapter/http/server.go` — Tasks 6, 7
- `internal/adapter/httperr/response.go` (new) — Task 7
- Tests — per task.

## Preconditions

- `pwd` is `/Users/kirillinakin/GolandProjects/pingcast`.
- `git status --short` shows only `?? docs/articles/`.
- `git log --oneline -1` is `49cd2e9 docs: add B1 security/correctness bugs design spec`.
- Docker is running (for integration tests in later tasks).
- Branch `b1-security-correctness` does NOT yet exist.

---

## Task 1: Setup feature branch + baseline

**Goal:** Create the working branch and confirm the tree is green before any changes.

- [ ] **Step 1.1: Create branch**

```bash
git checkout -b b1-security-correctness
git branch --show-current
```

Expected: `b1-security-correctness`

- [ ] **Step 1.2: Capture baseline lint set**

Run full lint, save to file for end-of-plan delta comparison:

```bash
golangci-lint run > /tmp/b1-lint-before.txt 2>&1 || true
wc -l /tmp/b1-lint-before.txt
grep -cE '^\S+\.go:[0-9]+' /tmp/b1-lint-before.txt
```

Expected: about 72 lines total output; around 64 lines match the finding pattern. Record both numbers — we'll compare after Task 8.

- [ ] **Step 1.3: Verify build/vet/test/race**

```bash
go build ./... && go vet ./... && go test -count=1 -short ./... && go test -race -count=1 -short ./...
echo "Exit: $?"
```

Expected: `Exit: 0`. (`-short` skips integration tests for speed; they'll run in Task 8.)

---

## Task 2: TLS MinVersion on HTTP monitor checker (spec §1)

**Files:** `internal/adapter/checker/http.go`

**Goal:** `HTTPChecker` rejects servers that only offer TLS ≤ 1.1. Unit test fails today, passes after the one-line fix.

- [ ] **Step 2.1: Write the failing test**

Create a new test file `internal/adapter/checker/http_tls_test.go`:

```go
package checker

import (
	"crypto/tls"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func TestHTTPChecker_RejectsLegacyTLS(t *testing.T) {
	srv := httptest.NewUnstartedServer(nil)
	srv.TLS = &tls.Config{MaxVersion: tls.VersionTLS11}
	srv.StartTLS()
	defer srv.Close()

	cfg, _ := json.Marshal(HTTPCheckConfig{URL: srv.URL, Method: MethodGET, ExpectedStatus: 200})
	monitor := &domain.Monitor{
		ID:          uuid.New(),
		Type:        domain.MonitorHTTP,
		CheckConfig: cfg,
	}

	chk := NewHTTPChecker()
	result := chk.Check(t.Context(), monitor)

	if result.Status != domain.StatusDown {
		t.Fatalf("expected StatusDown, got %s", result.Status)
	}
	if result.ErrorMessage == nil {
		t.Fatal("expected ErrorMessage to be populated")
	}
}
```

- [ ] **Step 2.2: Run and confirm failure**

```bash
go test -run TestHTTPChecker_RejectsLegacyTLS ./internal/adapter/checker/ -v
```

Expected: FAIL — the check passes because TLS 1.1 handshake succeeds with the default `tls.Config{}`.

- [ ] **Step 2.3: Apply the fix**

Edit `internal/adapter/checker/http.go`, locate the Transport block around line 62:

```go
// Before
Transport: &http.Transport{
    TLSClientConfig: &tls.Config{},
},
```

Replace with:

```go
// After
Transport: &http.Transport{
    TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
},
```

- [ ] **Step 2.4: Run and confirm pass**

```bash
go test -run TestHTTPChecker_RejectsLegacyTLS ./internal/adapter/checker/ -v
```

Expected: PASS. Result's `ErrorMessage` mentions TLS handshake failure.

- [ ] **Step 2.5: Verify no regressions**

```bash
go test -count=1 -short ./internal/adapter/checker/...
```

Expected: all tests PASS (the existing HTTP check tests do not rely on legacy TLS).

- [ ] **Step 2.6: Verify G402 cleared in lint**

```bash
golangci-lint run ./internal/adapter/checker/... 2>&1 | grep -c G402
```

Expected: `0`

---

## Task 3: Remove `template.JS` XSS placeholder (spec §2)

**Files:** `internal/adapter/http/pages.go`, `internal/web/templates/monitor_detail.html`

**Goal:** Delete the dead `ChartData` / `template.JS` path so the XSS foot-gun can't accidentally be loaded.

- [ ] **Step 3.1: Locate the template usage**

```bash
grep -n "ChartData" internal/web/templates/monitor_detail.html internal/adapter/http/pages.go
```

Record line numbers. Typical output:

```
internal/web/templates/monitor_detail.html:N: <script>const chartData = {{.ChartData}};</script>
internal/adapter/http/pages.go:197: chartJSON, _ := json.Marshal([]struct{}{}) // empty for now
internal/adapter/http/pages.go:212: "ChartData": template.JS(chartJSON),
```

- [ ] **Step 3.2: Remove ChartData from the handler**

Edit `internal/adapter/http/pages.go`. Delete the two lines identified:

```go
// Delete this line:
chartJSON, _ := json.Marshal([]struct{}{}) // empty for now
// ...
// Delete this entry inside the render map:
"ChartData": template.JS(chartJSON),
```

Also remove the `"html/template"` import if `template.JS` was its only user. Run `goimports` if available:

```bash
goimports -w internal/adapter/http/pages.go
```

(If goimports is not installed, the build will tell you what to trim.)

- [ ] **Step 3.3: Remove ChartData from the template**

Edit `internal/web/templates/monitor_detail.html` and delete the `{{.ChartData}}` reference (including the surrounding `<script>` block if it exists only to hold the placeholder).

- [ ] **Step 3.4: Build + run existing tests**

```bash
go build ./... && go test -count=1 -short ./internal/adapter/http/...
echo "Exit: $?"
```

Expected: `Exit: 0`. If the build fails because `json.Marshal` or `template` is now an unused import, trim it and re-run.

- [ ] **Step 3.5: Verify G203 cleared in lint**

```bash
golangci-lint run ./internal/adapter/http/... 2>&1 | grep -c G203
```

Expected: `0`

---

## Task 4: Log the silent rate-limiter reset error (spec §3)

**Files:** `internal/adapter/http/pages.go`

**Goal:** Replace the empty-body branch with a warning log.

- [ ] **Step 4.1: Write the failing test**

Edit `internal/adapter/http/handler_test.go` (or create a new file `pages_login_test.go` in the same package if the existing file is crowded). Add a test that asserts the warning is emitted when Reset fails.

```go
func TestLoginSubmit_RateLimiterResetFailureIsLogged(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	// ... set up mocks so Login succeeds and rateLimiter.Reset returns an error ...
	// Exact mock setup follows the existing pattern used by other handler tests in
	// handler_test.go (testEnv + rateLimiter mock EXPECT() calls). If no such test
	// exists yet for the login flow, create one using the testEnv helper.

	// Send request through the handler, then:
	if !strings.Contains(buf.String(), "rate limiter reset failed") {
		t.Fatalf("expected warning log to mention rate limiter reset failure; got:\n%s", buf.String())
	}
}
```

**Note:** If wiring the full login flow via the test app is too heavy, an
equivalent unit test that invokes the same `slog.Warn` code path through a
minimal extracted helper is acceptable. Keep the assertion: after the
branch executes, the configured `slog` sink contains the warning message.

- [ ] **Step 4.2: Run and confirm failure**

```bash
go test -run TestLoginSubmit_RateLimiterResetFailureIsLogged ./internal/adapter/http/... -v
```

Expected: FAIL — the log message is absent.

- [ ] **Step 4.3: Apply the fix**

Edit `internal/adapter/http/pages.go` around line 110:

```go
// Before
if err := h.rateLimiter.Reset(c.UserContext(), email); err != nil {
    // Non-blocking: login succeeded, just log
}
```

Replace with:

```go
// After
if err := h.rateLimiter.Reset(c.UserContext(), email); err != nil {
    slog.Warn("rate limiter reset failed after successful login", "email", email, "error", err)
}
```

- [ ] **Step 4.4: Run and confirm pass**

```bash
go test -run TestLoginSubmit_RateLimiterResetFailureIsLogged ./internal/adapter/http/... -v
```

Expected: PASS.

- [ ] **Step 4.5: Verify SA9003 cleared in lint**

```bash
golangci-lint run ./internal/adapter/http/... 2>&1 | grep -c SA9003
```

Expected: `0`

---

## Task 5: Delete unused test helper (spec §4)

**Files:** `internal/adapter/http/handler_test.go`

**Goal:** Remove `(*testEnv).createSessionForUser`.

- [ ] **Step 5.1: Confirm still unused**

```bash
grep -n "createSessionForUser" internal/adapter/http/
```

Expected: only the definition at `handler_test.go:81` (no callers).

- [ ] **Step 5.2: Delete the method**

Edit `internal/adapter/http/handler_test.go`, delete the entire `func (te *testEnv) createSessionForUser(t *testing.T, user *domain.User) string { ... }` block (the definition plus its leading comment).

- [ ] **Step 5.3: Build + unit tests**

```bash
go build ./... && go test -count=1 -short ./internal/adapter/http/...
echo "Exit: $?"
```

Expected: `Exit: 0`.

- [ ] **Step 5.4: Verify `unused` linter warning is gone**

```bash
golangci-lint run ./internal/adapter/http/... 2>&1 | grep -c createSessionForUser
```

Expected: `0`.

---

## Task 6: Login timing-attack equalisation (spec §5)

**Files:** `internal/app/auth.go`, `internal/app/auth_test.go` (or new `internal/app/auth_timing_test.go`)

**Goal:** Equalise response time so missing-user and wrong-password paths cannot be distinguished by latency.

- [ ] **Step 6.1: Write the failing benchmark**

Create `internal/app/auth_timing_test.go`:

```go
package app

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/mocks"
)

// BenchmarkLogin_MissingUser measures Login when GetByEmail returns an error.
func BenchmarkLogin_MissingUser(b *testing.B) {
	users := mocks.NewMockUserRepo(b)
	sessions := mocks.NewMockSessionRepo(b)
	users.EXPECT().
		GetByEmail(mock.Anything, mock.Anything).
		Return(nil, "", errors.New("not found")).
		Maybe()

	svc := NewAuthService(users, sessions)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = svc.Login(context.Background(), "absent@example.com", "password")
	}
}

// BenchmarkLogin_WrongPassword measures Login when user exists but bcrypt fails.
func BenchmarkLogin_WrongPassword(b *testing.B) {
	users := mocks.NewMockUserRepo(b)
	sessions := mocks.NewMockSessionRepo(b)

	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		b.Fatal(err)
	}
	existingUser := &domain.User{ID: uuid.New(), Email: "present@example.com"}
	users.EXPECT().
		GetByEmail(mock.Anything, "present@example.com").
		Return(existingUser, hash, nil).
		Maybe()

	svc := NewAuthService(users, sessions)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = svc.Login(context.Background(), "present@example.com", "wrong-password")
	}
}
```

Also add a non-benchmark assertion test — runs quickly, catches accidental regressions in CI:

```go
func TestLogin_TimingParityUnderShort(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test runs full bcrypt cost")
	}

	users := mocks.NewMockUserRepo(t)
	sessions := mocks.NewMockSessionRepo(t)
	hash, err := HashPassword("correct")
	if err != nil {
		t.Fatal(err)
	}
	existingUser := &domain.User{ID: uuid.New(), Email: "present@example.com"}

	users.EXPECT().GetByEmail(mock.Anything, "present@example.com").
		Return(existingUser, hash, nil).Maybe()
	users.EXPECT().GetByEmail(mock.Anything, "absent@example.com").
		Return(nil, "", errors.New("not found")).Maybe()

	svc := NewAuthService(users, sessions)

	const samples = 5
	var missingTotal, wrongTotal time.Duration

	for i := 0; i < samples; i++ {
		start := time.Now()
		_, _, _ = svc.Login(context.Background(), "absent@example.com", "password")
		missingTotal += time.Since(start)

		start = time.Now()
		_, _, _ = svc.Login(context.Background(), "present@example.com", "wrong-password")
		wrongTotal += time.Since(start)
	}

	missingAvg := missingTotal / samples
	wrongAvg := wrongTotal / samples

	// Assert the two paths are within 3× of each other. A fast missing-user
	// path would be >>3× smaller than the bcrypt path; equalised paths are ≈1×.
	ratio := float64(wrongAvg) / float64(missingAvg)
	if ratio > 3.0 || ratio < 0.33 {
		t.Fatalf("timing parity broken: missing=%s wrong=%s ratio=%.2fx (expected within 3x)",
			missingAvg, wrongAvg, ratio)
	}
}
```

Add the extra imports at the top of the file if not already present:

```go
import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/mock"

    "github.com/kirillinakin/pingcast/internal/domain"
    "github.com/kirillinakin/pingcast/internal/mocks"
)
```

- [ ] **Step 6.2: Run the parity test and confirm failure**

```bash
go test -run TestLogin_TimingParityUnderShort ./internal/app/... -v
```

Expected: FAIL — ratio is large (bcrypt path ~100 ms vs missing-user path ~1 ms ⇒ ratio ≫ 3).

- [ ] **Step 6.3: Apply the fix**

Edit `internal/app/auth.go`. Add a package-level precomputed dummy hash and
update `Login`:

```go
// Near the top of the file, after imports and existing package-level vars:

// dummyHash is precomputed once at startup and used to equalise Login response
// time when the email does not exist. Comparison always fails; result is discarded.
// Rationale: prevents user-enumeration via response-latency side channel.
var dummyHash = mustHashPassword("pingcast-dummy-timing-never-matches")

func mustHashPassword(p string) string {
	h, err := HashPassword(p)
	if err != nil {
		panic(fmt.Errorf("precompute dummy hash: %w", err))
	}
	return h
}
```

Replace the body of `Login`:

```go
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	user, hash, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// Equalise timing so response latency does not reveal whether the
		// email is registered. Result is discarded.
		_ = CheckPassword(dummyHash, password)
		return nil, "", fmt.Errorf("invalid email or password")
	}

	if !CheckPassword(hash, password) {
		return nil, "", fmt.Errorf("invalid email or password")
	}

	sessionID, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, sessionID, nil
}
```

- [ ] **Step 6.4: Run and confirm pass**

```bash
go test -run TestLogin_TimingParityUnderShort ./internal/app/... -v
```

Expected: PASS, ratio within `[0.33, 3.0]`.

- [ ] **Step 6.5: Optional — observe benchmark numbers**

```bash
go test -bench='^BenchmarkLogin' -benchtime=20x ./internal/app/... -run '^$'
```

Expected: both `ns/op` values are in the same order of magnitude (≈80-120 ms).
This is informational, not a pass/fail gate in CI.

---

## Task 7: `ErrUserExists` sentinel + enumeration-safe Register (spec §6)

**Files:** `internal/domain/errors.go`, `internal/adapter/postgres/errors.go` (new), `internal/adapter/postgres/incident_repo.go`, `internal/adapter/postgres/user_repo.go`, `internal/app/auth.go`, `internal/adapter/http/server.go`, `internal/adapter/http/pages.go`.

**Goal:** Classify duplicate-email registrations server-side without leaking that the email already exists to the client.

### 7a. Shared pg-errors helper

- [ ] **Step 7a.1: Create `postgres/errors.go`**

New file `internal/adapter/postgres/errors.go`:

```go
package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgUniqueViolation is the Postgres SQLSTATE code for a unique constraint violation.
const pgUniqueViolation = "23505"

// isUniqueViolation reports whether err originated from a Postgres unique-constraint
// violation. Returns false for nil, non-pg, or non-unique errors.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
```

- [ ] **Step 7a.2: Migrate `incident_repo.go` to the helper**

Edit `internal/adapter/postgres/incident_repo.go`:

1. Delete the local `const pgUniqueViolation = "23505"` line and its preceding comment.
2. Replace the existing inline check inside `Create`:

```go
// Before
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
    return nil, domain.ErrIncidentExists
}
return nil, err
```

```go
// After
if isUniqueViolation(err) {
    return nil, domain.ErrIncidentExists
}
return nil, err
```

3. Run `goimports -w internal/adapter/postgres/incident_repo.go` — the
   `errors` and `pgconn` imports become unused in that file and must be removed.

- [ ] **Step 7a.3: Build check**

```bash
go build ./...
echo "Exit: $?"
```

Expected: `Exit: 0`.

### 7b. `ErrUserExists` sentinel + repo classification

- [ ] **Step 7b.1: Add the sentinel**

Edit `internal/domain/errors.go`. Extend the sentinel block:

```go
var (
	ErrNotFound       = errors.New("not found")
	ErrForbidden      = errors.New("forbidden")
	ErrValidation     = errors.New("validation error")
	ErrConflict       = errors.New("conflict")
	ErrIncidentExists = errors.New("active incident already exists for this monitor")
	ErrUserExists     = errors.New("user already exists")
)
```

- [ ] **Step 7b.2: Write failing integration test for repo classification**

Append to `tests/integration/repo_test.go` (mirrors existing test style):

```go
func TestUserRepo_Create_DuplicateEmailReturnsErrUserExists(t *testing.T) {
	ctx := context.Background()
	pool, q, cleanup := SetupTestDB(t)
	defer cleanup()

	repo := postgres.NewUserRepo(q)

	// first create succeeds
	_, err := repo.Create(ctx, "dup@example.com", "slug1", "hash1")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	// second create with same email must classify as ErrUserExists
	_, err = repo.Create(ctx, "dup@example.com", "slug2", "hash2")
	if !errors.Is(err, domain.ErrUserExists) {
		t.Fatalf("expected domain.ErrUserExists, got %v", err)
	}
	_ = pool
}
```

- [ ] **Step 7b.3: Run and confirm failure**

```bash
go test -run TestUserRepo_Create_DuplicateEmailReturnsErrUserExists ./tests/integration/... -v
```

Expected: FAIL — current code returns the raw pg error.

- [ ] **Step 7b.4: Update `user_repo.go` Create**

Edit `internal/adapter/postgres/user_repo.go`:

```go
// Add imports:
import (
    // existing imports...
    "github.com/kirillinakin/pingcast/internal/domain"
)

func (r *UserRepo) Create(ctx context.Context, email, slug, passwordHash string) (*domain.User, error) {
	row, err := r.q.CreateUser(ctx, gen.CreateUserParams{
		Email:        email,
		Slug:         slug,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrUserExists
		}
		return nil, err
	}
	return userFromCreateRow(row), nil
}
```

- [ ] **Step 7b.5: Run and confirm pass**

```bash
go test -run TestUserRepo_Create_DuplicateEmailReturnsErrUserExists ./tests/integration/... -v
```

Expected: PASS.

### 7c. Register handler enumeration-safe classification

- [ ] **Step 7c.1: Update `AuthService.Register` to pass the sentinel through**

Edit `internal/app/auth.go`:

```go
import (
    "errors"
    // keep existing imports...
)

// In Register, replace:
user, err := s.users.Create(ctx, email, slug, hash)
if err != nil {
    return nil, "", fmt.Errorf("create user: %w", err)
}

// With:
user, err := s.users.Create(ctx, email, slug, hash)
if err != nil {
    if errors.Is(err, domain.ErrUserExists) {
        return nil, "", err
    }
    return nil, "", fmt.Errorf("create user: %w", err)
}
```

- [ ] **Step 7c.2: Update JSON Register handler log classification**

Edit `internal/adapter/http/server.go`. Locate the `Register` handler (around line 49). Replace the error branch:

```go
// Before
user, sessionID, err := s.auth.Register(c.UserContext(), string(req.Email), req.Slug, req.Password)
if err != nil {
    slog.Warn("registration failed", "error", err)
    return c.Status(400).JSON(apigen.ErrorResponse{Error: new("registration failed")})
}
```

```go
// After
user, sessionID, err := s.auth.Register(c.UserContext(), string(req.Email), req.Slug, req.Password)
if err != nil {
    if errors.Is(err, domain.ErrUserExists) {
        slog.Info("duplicate registration attempt", "email", string(req.Email))
    } else {
        slog.Warn("registration failed", "error", err)
    }
    return c.Status(400).JSON(apigen.ErrorResponse{Error: new("registration failed")})
}
```

Add `"errors"` to the import block if not present.

- [ ] **Step 7c.3: Update HTML Register handler symmetrically**

Edit `internal/adapter/http/pages.go`, locate `RegisterSubmit`. Apply the
same pattern:

```go
// In RegisterSubmit, replace the error branch of s.auth.Register(...) with:
_, sessionID, err := h.auth.Register(c.UserContext(), email, slug, password)
if err != nil {
    if errors.Is(err, domain.ErrUserExists) {
        slog.Info("duplicate registration attempt", "email", email)
    } else {
        slog.Warn("registration failed", "error", err)
    }
    return h.render(c, "register.html", fiber.Map{"Error": "Registration failed"})
}
```

Add `"errors"` to the import block if not present.

- [ ] **Step 7c.4: Integration test: duplicate registration returns safe body**

Append to `tests/integration/repo_test.go` or create
`tests/integration/auth_register_test.go`:

```go
func TestRegister_DuplicateEmail_ResponseIsGeneric(t *testing.T) {
	// This test requires the HTTP stack wired against a real DB. If the
	// project already has an HTTP integration harness, reuse it; otherwise
	// validate at the AuthService level with the real user_repo.

	ctx := context.Background()
	pool, q, cleanup := SetupTestDB(t)
	defer cleanup()
	_ = pool

	users := postgres.NewUserRepo(q)

	// Create sessions mock (SessionRepo interface — we don't care about it here;
	// the duplicate-email path returns BEFORE session creation).
	sessions := mocks.NewMockSessionRepo(t)

	auth := app.NewAuthService(users, sessions)

	_, _, err := auth.Register(ctx, "dupe@example.com", "userone", "password123")
	if err != nil {
		t.Fatalf("first register: %v", err)
	}

	_, _, err = auth.Register(ctx, "dupe@example.com", "usertwo", "password123")
	if !errors.Is(err, domain.ErrUserExists) {
		t.Fatalf("expected domain.ErrUserExists on duplicate email, got %v", err)
	}
}
```

- [ ] **Step 7c.5: Build + run new tests**

```bash
go build ./... && \
go test -count=1 -run 'TestUserRepo_Create_DuplicateEmail|TestRegister_DuplicateEmail' ./tests/integration/... -v
```

Expected: both tests PASS.

---

## Task 8: Generic HTTP error classifier (spec §7)

**Files:** `internal/adapter/httperr/response.go` (new), `internal/adapter/http/server.go`

**Goal:** Replace `new(err.Error())` in five channel handlers with a classifier that returns (status, client-safe-message).

- [ ] **Step 8.1: Write the classifier test**

Create `internal/adapter/httperr/response_test.go`:

```go
package httperr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kirillinakin/pingcast/internal/domain"
)

func TestClassifyHTTPError_Table(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"not-found", domain.ErrNotFound, 404, "not found"},
		{"validation", domain.ErrValidation, 400, "invalid request"},
		{"forbidden", domain.ErrForbidden, 403, "forbidden"},
		{"conflict", domain.ErrConflict, 409, "conflict"},
		{"user-exists", domain.ErrUserExists, 400, "registration failed"},
		{"wrapped-not-found", fmt.Errorf("wrap: %w", domain.ErrNotFound), 404, "not found"},
		{"unclassified", errors.New("boom"), 500, "internal error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotStatus, gotMsg := ClassifyHTTPError(c.err)
			if gotStatus != c.wantStatus || gotMsg != c.wantMsg {
				t.Fatalf("got (%d, %q), want (%d, %q)",
					gotStatus, gotMsg, c.wantStatus, c.wantMsg)
			}
		})
	}
}
```

- [ ] **Step 8.2: Run and confirm failure**

```bash
go test ./internal/adapter/httperr/... -v
```

Expected: compile error — `ClassifyHTTPError` does not exist yet.

- [ ] **Step 8.3: Create the classifier**

Create `internal/adapter/httperr/response.go`:

```go
package httperr

import (
	"errors"

	"github.com/kirillinakin/pingcast/internal/domain"
)

// ClassifyHTTPError maps a domain error to an HTTP status code and a
// client-safe message. Unclassified errors collapse to 500 / "internal
// error"; the raw error text is the caller's responsibility to log.
//
// ErrUserExists intentionally maps to a generic "registration failed"
// message to prevent user-enumeration via API response body.
func ClassifyHTTPError(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return 404, "not found"
	case errors.Is(err, domain.ErrValidation):
		return 400, "invalid request"
	case errors.Is(err, domain.ErrForbidden):
		return 403, "forbidden"
	case errors.Is(err, domain.ErrConflict):
		return 409, "conflict"
	case errors.Is(err, domain.ErrUserExists):
		return 400, "registration failed"
	default:
		return 500, "internal error"
	}
}
```

- [ ] **Step 8.4: Run and confirm pass**

```bash
go test ./internal/adapter/httperr/... -v
```

Expected: all cases PASS.

- [ ] **Step 8.5: Apply classifier to channel handlers**

Edit `internal/adapter/http/server.go`. Add `"github.com/kirillinakin/pingcast/internal/adapter/httperr"` to imports (if not present from unrelated code).

For each of the five handlers below, replace `return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})` with the classifier call:

1. `CreateChannel` (around line 507)
2. `UpdateChannel` (around line 539)
3. `DeleteChannel` (around line 550)
4. `BindChannel` (around line 567)
5. `UnbindChannel` (around line 578)

Replacement pattern (apply to all five):

```go
// Before (example from CreateChannel)
if err != nil {
    return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
}
```

```go
// After
if err != nil {
    slog.Warn("channel handler error", "path", c.Path(), "error", err)
    status, msg := httperr.ClassifyHTTPError(err)
    return c.Status(status).JSON(apigen.ErrorResponse{Error: new(msg)})
}
```

- [ ] **Step 8.6: Build + verify no new lint regressions**

```bash
go build ./... && golangci-lint run ./internal/adapter/httperr/... ./internal/adapter/http/... 2>&1 | head -20
echo "---"
grep -cE '\(errcheck\)|\(gosec\)|\(govet\)' /tmp/b1-lint-before.txt
```

Expected: build succeeds; the two lint warnings targeted in this task
(XSS G203, empty branch SA9003) are gone. Other pre-existing warnings
untouched.

---

## Task 9: Final verification + commit

**Files:** none (gate + commit)

- [ ] **Step 9.1: Full green gate**

```bash
{ echo "=== build ==="; go build ./...; echo "=== vet ==="; go vet ./...; echo "=== test ==="; go test -count=1 ./...; echo "=== race ==="; go test -race -count=1 ./...; } 2>&1 | tee /tmp/b1-final.log; echo "exit=${PIPESTATUS[0]}"
```

Expected: all four pass. (Integration tests run here — Docker must be up.)

- [ ] **Step 9.2: Lint delta report**

```bash
golangci-lint run > /tmp/b1-lint-after.txt 2>&1 || true
echo "Before: $(grep -cE '^\S+\.go:[0-9]+' /tmp/b1-lint-before.txt) findings"
echo "After:  $(grep -cE '^\S+\.go:[0-9]+' /tmp/b1-lint-after.txt) findings"
echo "--- cleared ---"
diff <(grep -oE '^\S+\.go:[0-9]+' /tmp/b1-lint-before.txt | sort -u) \
     <(grep -oE '^\S+\.go:[0-9]+' /tmp/b1-lint-after.txt  | sort -u) \
  | grep '^<' | sort
echo "--- introduced (should be empty) ---"
diff <(grep -oE '^\S+\.go:[0-9]+' /tmp/b1-lint-before.txt | sort -u) \
     <(grep -oE '^\S+\.go:[0-9]+' /tmp/b1-lint-after.txt  | sort -u) \
  | grep '^>' | sort
```

Expected: "cleared" list includes G402 (http.go), G203 (pages.go:212), SA9003
(pages.go:110), unused `createSessionForUser`, and any `ctx_unused` /
shadow lines that moved due to edits. "Introduced" list should be empty or
contain only line-number shifts of same-class pre-existing warnings.

- [ ] **Step 9.3: Stage all changes**

```bash
git add -A
git reset docs/articles/  # untracked, out of scope
git status --short
```

Expected: A mix of `M` and `A` entries for the files listed under "File Structure". No `docs/articles/`.

- [ ] **Step 9.4: Commit**

```bash
git commit -m "$(cat <<'EOF'
fix: B1 — security & correctness bugs

Seven targeted fixes per docs/superpowers/specs/2026-04-17-B1-security-
correctness-bugs-design.md:

- checker/http.go: TLS MinVersion=1.2 (G402)
- http/pages.go + monitor_detail.html: remove dead ChartData/template.JS
  placeholder (G203)
- http/pages.go: log rate-limiter-reset failures instead of silently
  swallowing them (SA9003)
- http/handler_test.go: delete unused createSessionForUser helper
- app/auth.go: equalise Login response time for missing-user vs
  wrong-password paths via dummy bcrypt compare (timing side-channel,
  user enumeration)
- domain/errors.go + postgres/user_repo.go + app/auth.go + http/server.go
  + http/pages.go: classify duplicate-email registrations server-side
  via ErrUserExists sentinel; response body remains generic
  "Registration failed" to prevent enumeration. Deviates from parent spec
  §4.4 ("email already registered") for security — documented in B1 spec.
- postgres/errors.go (new): extract pgUniqueViolation + isUniqueViolation
  helper; migrate incident_repo.go to use it.
- adapter/httperr/response.go (new): ClassifyHTTPError maps domain
  sentinels to (status, safe message); applied to 5 channel handlers
  in server.go that previously leaked raw err.Error() to clients.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git log --oneline -1
```

- [ ] **Step 9.5: Fast-forward merge to main**

```bash
git checkout main
git merge --ff-only b1-security-correctness
git branch -d b1-security-correctness
git log --oneline -5
```

Expected: main advances by one commit; branch deleted cleanly.

- [ ] **Step 9.6: Do NOT push**

The user has repeatedly confirmed "делай все сам" for local ops but
pushing is a shared-state action — defer to explicit user request.

---

## Self-Review

**1. Spec coverage:**
- §1 TLS → Task 2 ✅
- §2 template.JS → Task 3 ✅
- §3 empty branch → Task 4 ✅
- §4 unused func → Task 5 ✅
- §5 Login timing → Task 6 ✅
- §6 ErrUserExists + Register classification → Task 7 ✅ (three subsections)
- §7 HTTP response classifier → Task 8 ✅
- Spec "Files changed" summary → covered by §File Structure at top of plan ✅
- Spec "Testing strategy" → covered by per-task TDD steps + Task 9 race/integration ✅
- Spec "Success criteria" 1-6 → Task 9 ✅

**2. Placeholder scan:** no TBDs, TODOs, "appropriate error handling", or
vague references. Every code block has concrete code; every verification
step has a concrete command and expected output.

**3. Type consistency:**
- `ClassifyHTTPError` — signature `(err error) (int, string)` — consistent
  in Tasks 8.1 (test), 8.3 (impl), 8.5 (usage).
- `ErrUserExists` — added once in domain/errors.go (Task 7b.1), used in
  user_repo.go (7b.4), auth.go (7c.1), server.go (7c.2), pages.go (7c.3),
  httperr/response.go (8.3), and tests (7b.2, 7c.4, 8.1). Consistent.
- `isUniqueViolation` — defined in 7a.1, used in 7a.2 (incident_repo.go)
  and 7b.4 (user_repo.go). Consistent.
- `dummyHash` / `mustHashPassword` — defined and used in 6.3. Consistent.
