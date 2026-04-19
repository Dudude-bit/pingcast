# Sprint 1 — Foundation + Status-Page Polish · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the foundation of the status-page pivot — domain change, incident states + manual updates, Atlassian importer, LemonSqueezy Pro tier with founder-price cap, public stats endpoint, pricing/landing rewrite, Plausible analytics, JSON-LD enrichment.

**Architecture:** Hexagonal Go backend (domain → port → app → adapter), sqlc for SQL, oapi-codegen for HTTP types, Fiber for HTTP, Postgres + Redis + NATS. Next.js 16 App Router frontend with Tailwind v4 and shadcn/ui. All Pro-only features gated by a single new middleware that reads `users.plan`. Each sprint task is a standalone, committable change.

**Tech Stack:** Go 1.26 · Fiber · sqlc · pgx · oapi-codegen · NATS JetStream · Redis · Next.js 16 · TypeScript · Tailwind v4 · shadcn/ui · Playwright · LemonSqueezy · Plausible.

**Source spec:** `docs/superpowers/specs/2026-04-20-seo-landing-sales-design.md`

**Effort:** ~3 weeks of solo evening work (the heaviest sprint of the five).

---

## File Structure

New files:

- `internal/database/migrations/017_add_incident_states.sql`
- `internal/database/migrations/018_create_incident_updates.sql`
- `internal/sqlc/queries/incident_updates.sql`
- `internal/sqlc/queries/stats.sql`
- `internal/domain/incident_update.go`
- `internal/domain/plan_gate.go`
- `internal/adapter/http/middleware_pro.go`
- `internal/adapter/http/middleware_pro_test.go`
- `internal/adapter/http/stats.go`
- `internal/adapter/postgres/incident_update_repo.go`
- `internal/adapter/postgres/stats_repo.go`
- `internal/app/atlassian_importer.go`
- `internal/app/atlassian_importer_test.go`
- `internal/app/billing.go` (founder cap counter)
- `internal/adapter/http/atlassian_import.go`
- `tests/integration/api/incident_updates_test.go`
- `tests/integration/api/atlassian_import_test.go`
- `tests/integration/api/stats_public_test.go`
- `tests/integration/api/pro_gate_test.go`
- `tests/integration/api/founder_cap_test.go`
- `frontend/components/features/incidents/incident-timeline.tsx`
- `frontend/components/features/incidents/incident-state-badge.tsx`
- `frontend/components/features/incidents/incident-update-form.tsx`
- `frontend/components/features/billing/upgrade-button.tsx`
- `frontend/components/site/landing/hero.tsx`
- `frontend/components/site/landing/trust-bar.tsx`
- `frontend/components/site/landing/why-not-atlassian.tsx`
- `frontend/components/site/landing/features-grid.tsx`
- `frontend/components/site/landing/comparison-table.tsx`
- `frontend/components/site/landing/faq.tsx`
- `frontend/components/site/landing/final-cta.tsx`
- `frontend/components/analytics/plausible.tsx`
- `frontend/lib/analytics.ts`
- `frontend/app/(main)/dashboard/incidents/[id]/page.tsx`
- `frontend/app/(main)/import/atlassian/page.tsx`
- `frontend/tests/incidents-flow.spec.ts`
- `frontend/tests/upgrade-flow.spec.ts`
- `frontend/tests/atlassian-import.spec.ts`

Modified files:

- `api/openapi.yaml` — add `state` enum on `Incident`, add `IncidentUpdate` schema, new routes for incident management, Atlassian import, public stats, founder-price availability
- `internal/sqlc/queries/incidents.sql` — add `state` column to existing queries; add `UpdateIncidentState`
- `internal/sqlc/queries/users.sql` — add `CountActiveFounderSubscriptions` query
- `internal/domain/incident.go` — add `State` field + enum methods
- `internal/domain/user.go` — verify `Plan` enum includes `pro` value
- `internal/port/repository.go` — extend `IncidentRepo`, add `IncidentUpdateRepo`, `StatsRepo`
- `internal/adapter/postgres/incident_repo.go` — handle new `state` column
- `internal/adapter/postgres/mapper.go` — map new fields
- `internal/app/monitoring.go` — change `IncidentRepo.Create` signature impact
- `internal/adapter/http/setup.go` — register new routes + middleware
- `internal/adapter/http/server.go` — wire stats + import handlers
- `internal/bootstrap/app.go` — wire new repos + services
- `internal/config/config.go` — add `LEMONSQUEEZY_FOUNDER_VARIANT_ID`, `LEMONSQUEEZY_RETAIL_VARIANT_ID`, `FOUNDER_CAP`
- `frontend/app/(main)/page.tsx` — drop top-level `"use client"`, decompose into server-rendered sections
- `frontend/app/(main)/pricing/page.tsx` — rewrite with Pro $9 founder + retail
- `frontend/app/(main)/dashboard/page.tsx` — add Upgrade button
- `frontend/app/layout.tsx` — Plausible script + Organization JSON-LD
- `frontend/app/(main)/page.tsx` — FAQPage JSON-LD
- `frontend/app/sitemap.ts` — add new routes
- `frontend/components/site/footer.tsx` — five-column structure
- `frontend/components/site/landing-demo.tsx` — render branded sample
- `frontend/lib/api.ts` (or wherever the API client is) — add new endpoints
- `frontend/lib/openapi-types.ts` — regenerated
- `README.md` — update domain references
- `.env.example` — add new envs
- `docker-compose.yml` — pass new envs

Deleted files: none in Sprint 1.

---

## Task 0: Pro-gating middleware

**Why first:** Task 5 (incident state HTTP) and Task 9 (Atlassian importer HTTP) both depend on this. Cleaner to ship as its own commit so the gate is reviewed in isolation.

**Files:**
- Create: `internal/domain/plan_gate.go`
- Create: `internal/adapter/http/middleware_pro.go`
- Create: `internal/adapter/http/middleware_pro_test.go`
- Create: `tests/integration/api/pro_gate_test.go`
- Modify: `internal/adapter/http/setup.go`

- [ ] **Step 1: Write the unit test for the gate predicate**

```go
// internal/domain/plan_gate.go (test will go at plan_gate_test.go but the
// predicate also lives in domain so it's testable without HTTP setup)
```

Create `internal/domain/plan_gate_test.go`:

```go
package domain

import "testing"

func TestRequiresPro(t *testing.T) {
	tests := []struct {
		name string
		plan Plan
		want bool
	}{
		{"free user denied", PlanFree, true},
		{"pro user allowed", PlanPro, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RequiresPro(tt.plan); got != tt.want {
				t.Fatalf("RequiresPro(%v) = %v, want %v", tt.plan, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```
go test -short ./internal/domain/ -run TestRequiresPro
```

Expected: FAIL with "undefined: RequiresPro" (and possibly "undefined: PlanPro" if the const doesn't exist yet).

- [ ] **Step 3: Verify Plan enum has Pro value**

Read `internal/domain/user.go`. If `PlanPro` is not present, add:

```go
type Plan string

const (
	PlanFree Plan = "free"
	PlanPro  Plan = "pro"
)
```

- [ ] **Step 4: Implement RequiresPro**

Create `internal/domain/plan_gate.go`:

```go
package domain

// RequiresPro reports whether a feature gated to Pro users should be
// blocked for the given plan. Returns true → request denied.
func RequiresPro(p Plan) bool {
	return p != PlanPro
}
```

- [ ] **Step 5: Run the unit test to verify pass**

```
go test -short ./internal/domain/ -run TestRequiresPro
```

Expected: PASS.

- [ ] **Step 6: Write the middleware test**

Create `internal/adapter/http/middleware_pro_test.go`:

```go
package httpadapter

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

type stubUserGetter struct{ user *domain.User }

func (s stubUserGetter) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.user, nil
}

func TestRequireProMiddleware_blocksFree(t *testing.T) {
	app := fiber.New()
	uid := uuid.New()
	mw := RequirePro(stubUserGetter{user: &domain.User{ID: uid, Plan: domain.PlanFree}})
	app.Get("/protected", func(c *fiber.Ctx) error {
		c.Locals("user_id", uid)
		return c.SendStatus(fiber.StatusOK)
	}, mw)

	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusPaymentRequired {
		t.Fatalf("got %d, want 402", resp.StatusCode)
	}
}

func TestRequireProMiddleware_allowsPro(t *testing.T) {
	app := fiber.New()
	uid := uuid.New()
	mw := RequirePro(stubUserGetter{user: &domain.User{ID: uid, Plan: domain.PlanPro}})
	app.Get("/protected", mw, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Cookie", "session=anything") // bypassing real auth here
	// Actual middleware ordering uses the auth middleware to set user_id;
	// here we set it manually inside a wrapper.
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uid)
		return c.Next()
	})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("got %d, want 200", resp.StatusCode)
	}
}
```

> **Note for the implementer:** the second test as written has an
> ordering wart (Use after the route). Fix the test so the wrapper runs
> first; the production middleware MUST read `user_id` from `c.Locals`
> after the auth middleware has populated it.

- [ ] **Step 7: Run middleware tests to verify they fail**

```
go test -short ./internal/adapter/http/ -run TestRequirePro
```

Expected: FAIL with "undefined: RequirePro".

- [ ] **Step 8: Implement the middleware**

Create `internal/adapter/http/middleware_pro.go`:

```go
package httpadapter

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// userGetter is the slice of UserRepo that the Pro middleware needs.
// Defined here (not in port/) so the HTTP layer doesn't pull in the
// whole repo surface.
type userGetter interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// RequirePro returns a Fiber middleware that 402s any request from a user
// not on the Pro plan. Must run AFTER the auth middleware that sets
// `user_id` in `c.Locals`.
func RequirePro(users userGetter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, ok := c.Locals("user_id").(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "UNAUTHENTICATED",
					"message": "auth required",
				},
			})
		}
		user, err := users.GetByID(c.UserContext(), uid)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "USER_LOOKUP_FAILED",
					"message": "could not verify plan",
				},
			})
		}
		if domain.RequiresPro(user.Plan) {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "PRO_REQUIRED",
					"message": "this feature requires a Pro subscription",
				},
			})
		}
		return c.Next()
	}
}
```

- [ ] **Step 9: Run middleware tests to verify they pass**

```
go test -short ./internal/adapter/http/ -run TestRequirePro
```

Expected: PASS.

- [ ] **Step 10: Write integration test (free user gets 402, pro user gets 200 on a placeholder route)**

Create `tests/integration/api/pro_gate_test.go`:

```go
//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestProGate_freeBlocked_proAllowed(t *testing.T) {
	h := harness.New(t)

	free := h.RegisterAndLogin(t, "free@test.local", "freeuser", "Password123!")
	pro := h.RegisterAndLogin(t, "pro@test.local", "prouser", "Password123!")
	h.PromoteToPro(t, pro.UserID)

	// /api/incidents/0/state is one of the new Pro-gated routes (Task 5).
	// At this stage the route may not exist yet — this test will be
	// re-enabled by Task 5. Skip if route returns 404.
	resp := h.Do(t, "PATCH", "/api/incidents/1/state", free.SessionCookie,
		map[string]string{"state": "investigating"})
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("incident state route not yet wired (Task 5 not done)")
	}
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("free user: got %d, want 402", resp.StatusCode)
	}

	resp2 := h.Do(t, "PATCH", "/api/incidents/1/state", pro.SessionCookie,
		map[string]string{"state": "investigating"})
	// Pro user passes the gate; the route may then 404 because incident
	// 1 doesn't exist — but it must NOT be 402.
	if resp2.StatusCode == http.StatusPaymentRequired {
		t.Fatalf("pro user: got 402 (still gated)")
	}
}
```

> **Implementer note:** The harness helpers `RegisterAndLogin`,
> `PromoteToPro`, and `Do` may not exist verbatim. Check
> `tests/integration/api/harness/` and follow the existing conventions.
> If `PromoteToPro` is missing, add it: it should call
> `UserRepo.UpdatePlan(uid, domain.PlanPro)` directly through the test
> DB connection.

- [ ] **Step 11: Run lint and full short-test suite**

```
make lint && make test-short
```

Expected: 0 lint findings, 0 unit-test failures.

- [ ] **Step 12: Commit**

```bash
git add internal/domain/plan_gate.go internal/domain/plan_gate_test.go \
  internal/adapter/http/middleware_pro.go internal/adapter/http/middleware_pro_test.go \
  tests/integration/api/pro_gate_test.go
git commit -m "feat(billing): add Pro-plan gating middleware (402 for free users)"
```

---

## Task 1: Public stats endpoint

**Files:**
- Create: `internal/sqlc/queries/stats.sql`
- Create: `internal/adapter/postgres/stats_repo.go`
- Create: `internal/adapter/http/stats.go`
- Create: `tests/integration/api/stats_public_test.go`
- Modify: `api/openapi.yaml`
- Modify: `internal/port/repository.go`
- Modify: `internal/adapter/http/setup.go`
- Modify: `internal/bootstrap/app.go`

- [ ] **Step 1: Write the sqlc query**

Create `internal/sqlc/queries/stats.sql`:

```sql
-- name: GetPublicStats :one
SELECT
    (SELECT COUNT(*) FROM monitors WHERE deleted_at IS NULL)::bigint AS monitors_count,
    (SELECT COUNT(*) FROM incidents WHERE resolved_at IS NOT NULL)::bigint AS incidents_resolved,
    (SELECT COUNT(DISTINCT user_id) FROM monitors
     WHERE deleted_at IS NULL AND is_public = TRUE)::bigint AS public_status_pages;
```

> **Implementer note:** verify the column names. The `is_public` column
> may be on `users` (per-status-page slug) or on `monitors` (per-monitor
> visibility). Read `monitors.sql` and `users.sql` and adjust the query
> to whichever owns the public flag. If neither, count distinct user_id
> with `slug IS NOT NULL`.

- [ ] **Step 2: Regenerate sqlc**

```
make generate-go
```

Expected: `internal/sqlc/gen/stats.sql.go` exists with `GetPublicStats` method.

- [ ] **Step 3: Add the port interface**

Append to `internal/port/repository.go`:

```go
type StatsRepo interface {
	GetPublic(ctx context.Context) (PublicStats, error)
}

type PublicStats struct {
	MonitorsCount      int64
	IncidentsResolved  int64
	PublicStatusPages  int64
}
```

- [ ] **Step 4: Implement the repo**

Create `internal/adapter/postgres/stats_repo.go`:

```go
package postgres

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type StatsRepo struct {
	q *gen.Queries
}

func NewStatsRepo(q *gen.Queries) *StatsRepo { return &StatsRepo{q: q} }

func (r *StatsRepo) GetPublic(ctx context.Context) (port.PublicStats, error) {
	row, err := r.q.GetPublicStats(ctx)
	if err != nil {
		return port.PublicStats{}, err
	}
	return port.PublicStats{
		MonitorsCount:     row.MonitorsCount,
		IncidentsResolved: row.IncidentsResolved,
		PublicStatusPages: row.PublicStatusPages,
	}, nil
}
```

- [ ] **Step 5: Add OpenAPI route**

Edit `api/openapi.yaml`. Add under `paths:`:

```yaml
  /stats/public:
    get:
      operationId: getPublicStats
      summary: Public counters for the landing trust bar (no auth)
      responses:
        "200":
          description: ok
          headers:
            Cache-Control:
              schema:
                type: string
                example: "public, max-age=300"
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/PublicStats"
```

Add under `components.schemas`:

```yaml
    PublicStats:
      type: object
      required: [monitors_count, incidents_resolved, public_status_pages]
      properties:
        monitors_count:
          type: integer
          format: int64
        incidents_resolved:
          type: integer
          format: int64
        public_status_pages:
          type: integer
          format: int64
```

- [ ] **Step 6: Regenerate apigen**

```
make generate-go && make generate-ts
```

Expected: `internal/api/gen/server.go` includes `GetPublicStats` interface; `frontend/lib/openapi-types.ts` includes `PublicStats`.

- [ ] **Step 7: Implement the HTTP handler**

Create `internal/adapter/http/stats.go`:

```go
package httpadapter

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/port"
)

type StatsHandler struct {
	repo port.StatsRepo

	mu       sync.RWMutex
	cached   port.PublicStats
	expires  time.Time
}

func NewStatsHandler(repo port.StatsRepo) *StatsHandler {
	return &StatsHandler{repo: repo}
}

const publicStatsTTL = 5 * time.Minute

func (h *StatsHandler) GetPublic(c *fiber.Ctx) error {
	h.mu.RLock()
	if time.Now().Before(h.expires) {
		stats := h.cached
		h.mu.RUnlock()
		return h.respond(c, stats)
	}
	h.mu.RUnlock()

	stats, err := h.repo.GetPublic(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    "STATS_UNAVAILABLE",
				"message": "could not load stats",
			},
		})
	}
	h.mu.Lock()
	h.cached = stats
	h.expires = time.Now().Add(publicStatsTTL)
	h.mu.Unlock()
	return h.respond(c, stats)
}

func (h *StatsHandler) respond(c *fiber.Ctx, s port.PublicStats) error {
	c.Set("Cache-Control", "public, max-age=300")
	return c.JSON(fiber.Map{
		"monitors_count":      s.MonitorsCount,
		"incidents_resolved":  s.IncidentsResolved,
		"public_status_pages": s.PublicStatusPages,
	})
}

// Compile-time check that we satisfy the generated server interface.
var _ context.Context // keep import if not used elsewhere
```

- [ ] **Step 8: Wire in setup.go**

In `internal/adapter/http/setup.go`, add a route registration in the same
style as the existing public routes (no auth middleware):

```go
statsHandler := NewStatsHandler(deps.StatsRepo)
app.Get("/api/stats/public", statsHandler.GetPublic)
```

The exact location depends on the existing `setup.go` structure; place it
alongside the other unauthenticated routes (e.g. `/health`, `/status/:slug`).

- [ ] **Step 9: Wire in bootstrap/app.go**

In `internal/bootstrap/app.go`, add `StatsRepo: postgres.NewStatsRepo(queries)`
to the deps struct and propagate.

- [ ] **Step 10: Write the integration test**

Create `tests/integration/api/stats_public_test.go`:

```go
//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestPublicStats_unauthenticated(t *testing.T) {
	h := harness.New(t)

	resp := h.Do(t, "GET", "/api/stats/public", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("cache-control = %q, want public, max-age=300", cc)
	}

	var body struct {
		MonitorsCount     int64 `json:"monitors_count"`
		IncidentsResolved int64 `json:"incidents_resolved"`
		PublicStatusPages int64 `json:"public_status_pages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	// Empty DB → all zero. Just assert the call works.
	if body.MonitorsCount < 0 {
		t.Fatalf("nonsense count: %+v", body)
	}
}
```

- [ ] **Step 11: Run integration test**

```
make test-integration
```

Expected: `TestPublicStats_unauthenticated` passes.

- [ ] **Step 12: Lint + full unit suite**

```
make lint && make test-short
```

Expected: 0 findings, 0 failures.

- [ ] **Step 13: Commit**

```bash
git add api/openapi.yaml internal/sqlc/queries/stats.sql \
  internal/sqlc/gen/stats.sql.go internal/sqlc/gen/models.go \
  internal/api/gen/server.go internal/api/gen/types.go \
  internal/port/repository.go internal/adapter/postgres/stats_repo.go \
  internal/adapter/http/stats.go internal/adapter/http/setup.go \
  internal/bootstrap/app.go tests/integration/api/stats_public_test.go \
  frontend/lib/openapi-types.ts
git commit -m "feat(api): add /api/stats/public for landing trust bar"
```

---

## Task 2: Incident states migration + domain

**Files:**
- Create: `internal/database/migrations/017_add_incident_states.sql`
- Modify: `internal/domain/incident.go`
- Modify: `internal/sqlc/queries/incidents.sql`
- Modify: `internal/adapter/postgres/incident_repo.go`
- Modify: `internal/adapter/postgres/mapper.go`
- Modify: `internal/port/repository.go`

- [ ] **Step 1: Write the migration**

Create `internal/database/migrations/017_add_incident_states.sql`:

```sql
-- +goose Up
CREATE TYPE incident_state AS ENUM (
    'investigating',
    'identified',
    'monitoring',
    'resolved'
);

ALTER TABLE incidents
    ADD COLUMN state incident_state NOT NULL DEFAULT 'investigating',
    ADD COLUMN is_manual BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN title TEXT;

-- Backfill existing rows: resolved → resolved, otherwise investigating.
UPDATE incidents SET state = 'resolved' WHERE resolved_at IS NOT NULL;

-- +goose Down
ALTER TABLE incidents DROP COLUMN state, DROP COLUMN is_manual, DROP COLUMN title;
DROP TYPE incident_state;
```

> **Note:** verify the project's goose conventions by reading an
> existing migration. If goose annotations are different (e.g. `-- +migrate Up`),
> match the existing style.

- [ ] **Step 2: Update domain model**

Edit `internal/domain/incident.go`. Replace its contents with:

```go
package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type IncidentState string

const (
	IncidentStateInvestigating IncidentState = "investigating"
	IncidentStateIdentified    IncidentState = "identified"
	IncidentStateMonitoring    IncidentState = "monitoring"
	IncidentStateResolved      IncidentState = "resolved"
)

func (s IncidentState) Valid() bool {
	switch s {
	case IncidentStateInvestigating, IncidentStateIdentified,
		IncidentStateMonitoring, IncidentStateResolved:
		return true
	}
	return false
}

// CanTransitionTo enforces the public-facing lifecycle. We allow any
// forward move and a "rewind" only from monitoring → identified (an
// engineer realising they were wrong before declaring resolved). All
// other backwards moves are rejected.
func (s IncidentState) CanTransitionTo(next IncidentState) error {
	if !next.Valid() {
		return fmt.Errorf("invalid incident state: %q", next)
	}
	if s == next {
		return nil
	}
	order := map[IncidentState]int{
		IncidentStateInvestigating: 0,
		IncidentStateIdentified:    1,
		IncidentStateMonitoring:    2,
		IncidentStateResolved:      3,
	}
	curr := order[s]
	want := order[next]
	if want > curr {
		return nil
	}
	if s == IncidentStateMonitoring && next == IncidentStateIdentified {
		return nil
	}
	return fmt.Errorf("cannot move incident from %s to %s", s, next)
}

type Incident struct {
	ID         int64
	MonitorID  uuid.UUID
	StartedAt  time.Time
	ResolvedAt *time.Time
	Cause      string
	State      IncidentState
	IsManual   bool
	Title      *string
}

func (i Incident) IsResolved() bool {
	return i.ResolvedAt != nil
}
```

- [ ] **Step 3: Write unit tests for state transitions**

Append to existing `internal/domain/` test files or create
`internal/domain/incident_test.go`:

```go
package domain

import "testing"

func TestIncidentState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from    IncidentState
		to      IncidentState
		wantErr bool
	}{
		{IncidentStateInvestigating, IncidentStateIdentified, false},
		{IncidentStateInvestigating, IncidentStateMonitoring, false},
		{IncidentStateInvestigating, IncidentStateResolved, false},
		{IncidentStateMonitoring, IncidentStateIdentified, false},
		{IncidentStateResolved, IncidentStateInvestigating, true},
		{IncidentStateIdentified, IncidentStateInvestigating, true},
		{IncidentStateInvestigating, "garbage", true},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			err := tt.from.CanTransitionTo(tt.to)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 4: Run unit tests**

```
go test -short ./internal/domain/
```

Expected: PASS.

- [ ] **Step 5: Update sqlc queries**

Edit `internal/sqlc/queries/incidents.sql`:

```sql
-- name: CreateIncident :one
INSERT INTO incidents (monitor_id, cause, state, is_manual, title)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, monitor_id, started_at, resolved_at, cause, state, is_manual, title;

-- name: ResolveIncident :exec
UPDATE incidents SET resolved_at = $2, state = 'resolved' WHERE id = $1;

-- name: UpdateIncidentState :exec
UPDATE incidents SET state = $2 WHERE id = $1;

-- name: GetIncidentByID :one
SELECT id, monitor_id, started_at, resolved_at, cause, state, is_manual, title
FROM incidents WHERE id = $1;

-- name: GetOpenIncidentByMonitorID :one
SELECT id, monitor_id, started_at, resolved_at, cause, state, is_manual, title
FROM incidents
WHERE monitor_id = $1 AND resolved_at IS NULL
ORDER BY started_at DESC
LIMIT 1;

-- name: IsInCooldown :one
SELECT EXISTS(
    SELECT 1 FROM incidents
    WHERE monitor_id = $1 AND resolved_at IS NOT NULL
    AND resolved_at > NOW() - INTERVAL '5 minutes'
)::bool AS in_cooldown;

-- name: ListIncidentsByMonitorID :many
SELECT id, monitor_id, started_at, resolved_at, cause, state, is_manual, title
FROM incidents
WHERE monitor_id = $1
ORDER BY started_at DESC
LIMIT $2;
```

- [ ] **Step 6: Update IncidentRepo signature**

Edit `internal/port/repository.go`:

```go
type IncidentRepo interface {
	Create(ctx context.Context, in CreateIncidentInput) (*domain.Incident, error)
	Resolve(ctx context.Context, id int64, resolvedAt time.Time) error
	UpdateState(ctx context.Context, id int64, state domain.IncidentState) error
	GetByID(ctx context.Context, id int64) (*domain.Incident, error)
	GetOpen(ctx context.Context, monitorID uuid.UUID) (*domain.Incident, error)
	IsInCooldown(ctx context.Context, monitorID uuid.UUID) (bool, error)
	ListByMonitorID(ctx context.Context, monitorID uuid.UUID, limit int) ([]domain.Incident, error)
}

type CreateIncidentInput struct {
	MonitorID uuid.UUID
	Cause     string
	State     domain.IncidentState // defaults to investigating
	IsManual  bool
	Title     *string
}
```

- [ ] **Step 7: Regenerate sqlc + apigen**

```
make generate-go
```

- [ ] **Step 8: Update IncidentRepo postgres impl**

Edit `internal/adapter/postgres/incident_repo.go`. Adjust `Create`,
`Resolve`, etc. to populate the new fields. Add `UpdateState` and
`GetByID`. Use `mapper.go` to convert sqlc rows to `domain.Incident`.

> **Implementer note:** read the existing `incident_repo.go` and follow
> its pattern. Auto-detected callers in `monitoring.go` should pass
> `State: domain.IncidentStateInvestigating, IsManual: false`.

- [ ] **Step 9: Update mapper.go**

In `internal/adapter/postgres/mapper.go`, extend the incident-row→domain
converter to populate `State`, `IsManual`, `Title`.

- [ ] **Step 10: Update callers in app/monitoring.go**

Update `monitoring.go` so any place that called `incidents.Create(ctx, monitorID, cause)`
now calls `incidents.Create(ctx, port.CreateIncidentInput{MonitorID: id, Cause: cause})`.

- [ ] **Step 11: Run unit tests + integration repo tests**

```
make test-short
make test-integration
```

Expected: PASS for unit, PASS for repo integration. The API integration
suite may need updates if `Incident` JSON serialisation changed; address
in Step 12.

- [ ] **Step 12: Update OpenAPI Incident schema**

In `api/openapi.yaml`, extend the `Incident` schema:

```yaml
    Incident:
      type: object
      required: [id, monitor_id, started_at, cause, state, is_manual]
      properties:
        id:
          type: integer
          format: int64
        monitor_id:
          type: string
          format: uuid
        started_at:
          type: string
          format: date-time
        resolved_at:
          type: string
          format: date-time
          nullable: true
        cause:
          type: string
        state:
          type: string
          enum: [investigating, identified, monitoring, resolved]
        is_manual:
          type: boolean
        title:
          type: string
          nullable: true
```

- [ ] **Step 13: Regenerate apigen + frontend types**

```
make generate
```

- [ ] **Step 14: Lint + full test suite**

```
make lint && make test
```

Expected: clean.

- [ ] **Step 15: Commit**

```bash
git add internal/database/migrations/017_add_incident_states.sql \
  internal/domain/incident.go internal/domain/incident_test.go \
  internal/sqlc/queries/incidents.sql internal/sqlc/gen/incidents.sql.go \
  internal/sqlc/gen/models.go internal/port/repository.go \
  internal/adapter/postgres/incident_repo.go internal/adapter/postgres/mapper.go \
  internal/app/monitoring.go api/openapi.yaml internal/api/gen/server.go \
  internal/api/gen/types.go frontend/lib/openapi-types.ts
git commit -m "feat(incidents): add state enum (investigating/identified/monitoring/resolved) and is_manual flag"
```

---

## Task 3: IncidentUpdate model + repo

**Files:**
- Create: `internal/database/migrations/018_create_incident_updates.sql`
- Create: `internal/sqlc/queries/incident_updates.sql`
- Create: `internal/domain/incident_update.go`
- Create: `internal/adapter/postgres/incident_update_repo.go`
- Modify: `internal/port/repository.go`
- Modify: `internal/bootstrap/app.go`

- [ ] **Step 1: Write the migration**

Create `internal/database/migrations/018_create_incident_updates.sql`:

```sql
-- +goose Up
CREATE TABLE incident_updates (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    state incident_state NOT NULL,
    body TEXT NOT NULL,
    posted_by_user_id UUID NOT NULL REFERENCES users(id),
    posted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_incident_updates_incident_id_posted_at
    ON incident_updates (incident_id, posted_at DESC);

-- +goose Down
DROP TABLE incident_updates;
```

- [ ] **Step 2: Write sqlc queries**

Create `internal/sqlc/queries/incident_updates.sql`:

```sql
-- name: CreateIncidentUpdate :one
INSERT INTO incident_updates (incident_id, state, body, posted_by_user_id)
VALUES ($1, $2, $3, $4)
RETURNING id, incident_id, state, body, posted_by_user_id, posted_at;

-- name: ListIncidentUpdates :many
SELECT id, incident_id, state, body, posted_by_user_id, posted_at
FROM incident_updates
WHERE incident_id = $1
ORDER BY posted_at DESC;
```

- [ ] **Step 3: Add domain type**

Create `internal/domain/incident_update.go`:

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

type IncidentUpdate struct {
	ID             int64
	IncidentID     int64
	State          IncidentState
	Body           string
	PostedByUserID uuid.UUID
	PostedAt       time.Time
}
```

- [ ] **Step 4: Add port interface**

Append to `internal/port/repository.go`:

```go
type IncidentUpdateRepo interface {
	Create(ctx context.Context, in CreateIncidentUpdateInput) (*domain.IncidentUpdate, error)
	ListByIncidentID(ctx context.Context, incidentID int64) ([]domain.IncidentUpdate, error)
}

type CreateIncidentUpdateInput struct {
	IncidentID     int64
	State          domain.IncidentState
	Body           string
	PostedByUserID uuid.UUID
}
```

- [ ] **Step 5: Regenerate sqlc**

```
make generate-go
```

- [ ] **Step 6: Implement the repo**

Create `internal/adapter/postgres/incident_update_repo.go`. Follow the
pattern of `incident_repo.go` — wrap sqlc, return domain types.

- [ ] **Step 7: Wire into bootstrap**

Add `IncidentUpdateRepo: postgres.NewIncidentUpdateRepo(queries)` in
`internal/bootstrap/app.go` and propagate through deps.

- [ ] **Step 8: Repo integration test**

Add to `internal/adapter/postgres/repo_integration_test.go` (follow
the existing test conventions there):

```go
func TestIncidentUpdateRepo_CreateAndList(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	users := postgres.NewUserRepo(...)
	monitors := postgres.NewMonitorRepo(...)
	incidents := postgres.NewIncidentRepo(...)
	updates := postgres.NewIncidentUpdateRepo(...)

	// Set up a user → monitor → incident → updates chain.
	user, _ := users.Create(ctx, "u@test.local", "userslug", "hash")
	monitor, _ := monitors.Create(ctx, &domain.Monitor{ /* ... */ })
	inc, _ := incidents.Create(ctx, port.CreateIncidentInput{
		MonitorID: monitor.ID,
		Cause:     "test outage",
		State:     domain.IncidentStateInvestigating,
		IsManual:  true,
	})

	u1, err := updates.Create(ctx, port.CreateIncidentUpdateInput{
		IncidentID:     inc.ID,
		State:          domain.IncidentStateInvestigating,
		Body:           "Looking into it",
		PostedByUserID: user.ID,
	})
	require.NoError(t, err)

	list, err := updates.ListByIncidentID(ctx, inc.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, u1.ID, list[0].ID)
}
```

> **Implementer note:** copy the precise testdb harness setup from
> existing `repo_integration_test.go` rather than ad-libbing.

- [ ] **Step 9: Run integration tests**

```
make test-integration
```

Expected: PASS.

- [ ] **Step 10: Lint + commit**

```
make lint
git add internal/database/migrations/018_create_incident_updates.sql \
  internal/sqlc/queries/incident_updates.sql \
  internal/sqlc/gen/incident_updates.sql.go internal/sqlc/gen/models.go \
  internal/domain/incident_update.go internal/port/repository.go \
  internal/adapter/postgres/incident_update_repo.go \
  internal/adapter/postgres/repo_integration_test.go \
  internal/bootstrap/app.go
git commit -m "feat(incidents): add incident_updates table for manual status posts"
```

---

## Task 4: Incident state/update service methods

**Files:**
- Modify: `internal/app/monitoring.go`
- Create: `internal/app/monitoring_incidents_test.go`

- [ ] **Step 1: Write failing test for ChangeIncidentState**

Append to existing tests or create `internal/app/monitoring_incidents_test.go`:

```go
package app_test

import (
	"context"
	"testing"

	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func TestMonitoringService_ChangeIncidentState_appendUpdate(t *testing.T) {
	// Use mocks from internal/mocks/mocks.go.
	// Set up: monitor owned by userA, open incident in 'investigating'.
	// Call ChangeIncidentState(userA, incidentID, 'identified', "We see DNS")
	// Assert:
	//   - incidents.UpdateState called with 'identified'
	//   - incident_updates.Create called with body "We see DNS"
	// (Detailed expectations modelled after existing service tests in
	// internal/app/.)
	t.Skip("test scaffold — fill in mocks per existing test conventions")
}
```

> **Implementer note:** copy the mock-setup style from
> `internal/app/alert_test.go` and `internal/app/monitoring.go` (look
> for `MonitoringService` test patterns). The scaffold above is a
> placeholder — replace with the real test before going green.

- [ ] **Step 2: Add service methods**

In `internal/app/monitoring.go`, add to `MonitoringService`:

```go
type ChangeIncidentStateInput struct {
	IncidentID     int64
	UserID         uuid.UUID
	NewState       domain.IncidentState
	UpdateBody     string
}

func (s *MonitoringService) ChangeIncidentState(
	ctx context.Context, in ChangeIncidentStateInput,
) (*domain.IncidentUpdate, error) {
	inc, err := s.incidents.GetByID(ctx, in.IncidentID)
	if err != nil {
		return nil, err
	}
	monitor, err := s.monitors.GetByID(ctx, inc.MonitorID)
	if err != nil {
		return nil, err
	}
	if monitor.UserID != in.UserID {
		return nil, domain.ErrForbidden
	}
	if err := inc.State.CanTransitionTo(in.NewState); err != nil {
		return nil, err
	}
	// Apply state, then append update inside the same transaction.
	return s.txm.Run(ctx, func(ctx context.Context) (*domain.IncidentUpdate, error) {
		if err := s.incidents.UpdateState(ctx, in.IncidentID, in.NewState); err != nil {
			return nil, err
		}
		if in.NewState == domain.IncidentStateResolved {
			now := s.clock.Now()
			if err := s.incidents.Resolve(ctx, in.IncidentID, now); err != nil {
				return nil, err
			}
		}
		return s.incidentUpdates.Create(ctx, port.CreateIncidentUpdateInput{
			IncidentID:     in.IncidentID,
			State:          in.NewState,
			Body:           in.UpdateBody,
			PostedByUserID: in.UserID,
		})
	})
}

type CreateManualIncidentInput struct {
	MonitorID uuid.UUID
	UserID    uuid.UUID
	Title     string
	Body      string
}

func (s *MonitoringService) CreateManualIncident(
	ctx context.Context, in CreateManualIncidentInput,
) (*domain.Incident, *domain.IncidentUpdate, error) {
	monitor, err := s.monitors.GetByID(ctx, in.MonitorID)
	if err != nil {
		return nil, nil, err
	}
	if monitor.UserID != in.UserID {
		return nil, nil, domain.ErrForbidden
	}
	// ... create incident with IsManual=true, Title=&in.Title, then
	// initial IncidentUpdate with body=in.Body, state=investigating.
	// Returns both for convenience.
}
```

> **Implementer note:** the `txm.Run` generic-Result signature shown
> above may not match what the codebase actually exposes. Check
> `internal/port/uow.go` and adapt — wrap explicitly in begin/commit
> calls if no generic helper exists.

- [ ] **Step 3: Add IncidentUpdateRepo + IncidentUpdates field to MonitoringService**

Update the `MonitoringService` struct and `NewMonitoringService` constructor
to take the new repo. Update `internal/bootstrap/app.go` accordingly.

- [ ] **Step 4: Run unit tests**

```
make test-short
```

Expected: PASS.

- [ ] **Step 5: Lint + commit**

```
make lint
git add internal/app/monitoring.go internal/app/monitoring_incidents_test.go \
  internal/bootstrap/app.go
git commit -m "feat(incidents): add ChangeIncidentState and CreateManualIncident services"
```

---

## Task 5: Incident state/update HTTP endpoints (Pro-gated)

**Files:**
- Modify: `api/openapi.yaml`
- Modify: `internal/adapter/http/setup.go`
- Create: `internal/adapter/http/incidents.go`
- Create: `tests/integration/api/incident_updates_test.go`

- [ ] **Step 1: Add OpenAPI routes**

Edit `api/openapi.yaml`:

```yaml
  /incidents:
    post:
      operationId: createManualIncident
      security: [{ sessionAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [monitor_id, title, body]
              properties:
                monitor_id: { type: string, format: uuid }
                title:      { type: string, minLength: 3, maxLength: 200 }
                body:       { type: string, minLength: 1, maxLength: 4000 }
      responses:
        "201":
          description: created
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Incident" }
        "402":
          description: pro required
        "403":
          description: monitor not owned

  /incidents/{id}/state:
    patch:
      operationId: updateIncidentState
      security: [{ sessionAuth: [] }]
      parameters:
        - in: path
          name: id
          required: true
          schema: { type: integer, format: int64 }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [state, body]
              properties:
                state: { type: string, enum: [investigating, identified, monitoring, resolved] }
                body:  { type: string, minLength: 1, maxLength: 4000 }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/IncidentUpdate" }
        "402": { description: pro required }
        "403": { description: not owned }
        "409": { description: invalid state transition }

  /incidents/{id}/updates:
    get:
      operationId: listIncidentUpdates
      parameters:
        - in: path
          name: id
          required: true
          schema: { type: integer, format: int64 }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: array
                items: { $ref: "#/components/schemas/IncidentUpdate" }
```

Add `IncidentUpdate` schema:

```yaml
    IncidentUpdate:
      type: object
      required: [id, incident_id, state, body, posted_at]
      properties:
        id:                { type: integer, format: int64 }
        incident_id:       { type: integer, format: int64 }
        state:             { type: string, enum: [investigating, identified, monitoring, resolved] }
        body:              { type: string }
        posted_by_user_id: { type: string, format: uuid }
        posted_at:         { type: string, format: date-time }
```

- [ ] **Step 2: Regenerate**

```
make generate
```

- [ ] **Step 3: Implement handler**

Create `internal/adapter/http/incidents.go`. Follow the existing handler
patterns (read e.g. `monitors.go` if present, or `setup.go`'s monitor
routes). Each handler:
- pulls `user_id` from `c.Locals`
- parses body via `c.BodyParser`
- calls service method
- maps domain errors to HTTP statuses (`ErrForbidden` → 403,
  `state-transition` errors → 409, etc.)

- [ ] **Step 4: Wire routes (Pro-gated)**

In `internal/adapter/http/setup.go`:

```go
proGroup := app.Group("/api", authMiddleware, RequirePro(deps.UserRepo))
proGroup.Post("/incidents", incidentsHandler.Create)
proGroup.Patch("/incidents/:id/state", incidentsHandler.UpdateState)
// public read:
app.Get("/api/incidents/:id/updates", incidentsHandler.ListUpdates)
```

> **Note:** placement under `proGroup` ensures the gate runs after auth.
> The exact route group structure depends on existing setup.go — match
> the project's pattern.

- [ ] **Step 5: Write integration test**

Create `tests/integration/api/incident_updates_test.go`:

```go
//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestIncidentLifecycle_proCanManageStates(t *testing.T) {
	h := harness.New(t)
	pro := h.RegisterAndLogin(t, "pro@test.local", "prouser", "Password123!")
	h.PromoteToPro(t, pro.UserID)
	monitor := h.CreateMonitor(t, pro.SessionCookie, "https://example.com")

	resp := h.Do(t, "POST", "/api/incidents", pro.SessionCookie, map[string]any{
		"monitor_id": monitor.ID,
		"title":      "Outage at 14:00 UTC",
		"body":       "We're investigating reports of slow response times.",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d", resp.StatusCode)
	}
	var inc struct{ ID int64 `json:"id"` }
	h.DecodeBody(t, resp, &inc)

	resp2 := h.Do(t, "PATCH", "/api/incidents/"+itoa(inc.ID)+"/state", pro.SessionCookie,
		map[string]string{"state": "identified", "body": "DNS misconfiguration"})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("update state: got %d", resp2.StatusCode)
	}

	resp3 := h.Do(t, "GET", "/api/incidents/"+itoa(inc.ID)+"/updates", "", nil)
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("list updates: got %d", resp3.StatusCode)
	}
	var updates []struct{ State string `json:"state"` }
	h.DecodeBody(t, resp3, &updates)
	if len(updates) != 2 {
		t.Fatalf("want 2 updates (investigating + identified), got %d", len(updates))
	}
}

func TestIncidentLifecycle_freeIs402(t *testing.T) {
	h := harness.New(t)
	free := h.RegisterAndLogin(t, "free@test.local", "freeuser", "Password123!")
	monitor := h.CreateMonitor(t, free.SessionCookie, "https://example.com")
	resp := h.Do(t, "POST", "/api/incidents", free.SessionCookie, map[string]any{
		"monitor_id": monitor.ID,
		"title":      "x", "body": "y",
	})
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("got %d, want 402", resp.StatusCode)
	}
}
```

- [ ] **Step 6: Run tests + lint**

```
make test-integration && make lint
```

Expected: PASS, 0 findings.

- [ ] **Step 7: Commit**

```bash
git add api/openapi.yaml internal/api/gen/server.go internal/api/gen/types.go \
  frontend/lib/openapi-types.ts internal/adapter/http/incidents.go \
  internal/adapter/http/setup.go tests/integration/api/incident_updates_test.go
git commit -m "feat(api): incident state/update CRUD (Pro-gated)"
```

---

## Task 6: Status page incident timeline (frontend)

**Files:**
- Create: `frontend/components/features/incidents/incident-timeline.tsx`
- Create: `frontend/components/features/incidents/incident-state-badge.tsx`
- Modify: `frontend/app/status/[slug]/page.tsx`
- Modify: `frontend/lib/queries.ts` (or wherever queries are defined)
- Create: `frontend/tests/incidents-timeline.spec.ts`

- [ ] **Step 1: Build the IncidentStateBadge**

Create `frontend/components/features/incidents/incident-state-badge.tsx`:

```tsx
import { cn } from "@/lib/utils";

const STATE_COLORS: Record<string, string> = {
  investigating: "bg-amber-500/15 text-amber-700 dark:text-amber-300",
  identified:    "bg-orange-500/15 text-orange-700 dark:text-orange-300",
  monitoring:    "bg-blue-500/15 text-blue-700 dark:text-blue-300",
  resolved:      "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300",
};

const STATE_LABELS: Record<string, string> = {
  investigating: "Investigating",
  identified:    "Identified",
  monitoring:    "Monitoring",
  resolved:      "Resolved",
};

export function IncidentStateBadge({ state }: { state: string }) {
  return (
    <span className={cn(
      "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium",
      STATE_COLORS[state] ?? "bg-muted text-muted-foreground"
    )}>
      {STATE_LABELS[state] ?? state}
    </span>
  );
}
```

- [ ] **Step 2: Build the IncidentTimeline**

Create `frontend/components/features/incidents/incident-timeline.tsx`:

```tsx
import { IncidentStateBadge } from "./incident-state-badge";

type Update = {
  id: number;
  state: string;
  body: string;
  posted_at: string;
};

export function IncidentTimeline({ updates }: { updates: Update[] }) {
  if (!updates.length) return null;
  return (
    <ol className="border-l border-border/60 space-y-4 pl-5">
      {updates.map((u) => (
        <li key={u.id} className="relative">
          <span className="absolute -left-[27px] top-1 h-3 w-3 rounded-full bg-primary ring-4 ring-background" />
          <div className="flex items-center gap-2">
            <IncidentStateBadge state={u.state} />
            <time className="text-xs text-muted-foreground">
              {new Date(u.posted_at).toLocaleString()}
            </time>
          </div>
          <p className="mt-2 text-sm text-foreground">{u.body}</p>
        </li>
      ))}
    </ol>
  );
}
```

- [ ] **Step 3: Fetch updates on the status page**

Modify `frontend/app/status/[slug]/page.tsx` (read it first to understand
current structure). For each incident, fetch updates via the new
`/api/incidents/{id}/updates` endpoint. SSR-fetch using the existing
session-aware fetch helper. Render `<IncidentTimeline updates={updates} />`
under each incident block.

- [ ] **Step 4: Write Playwright spec**

Create `frontend/tests/incidents-timeline.spec.ts`:

```ts
import { test, expect } from "@playwright/test";

test("public status page renders incident timeline with state badges", async ({ page, request }) => {
  // 1. Bootstrap a Pro user via API.
  // 2. Create a monitor + manual incident + state changes via API.
  // 3. Navigate to /status/<slug> and assert the timeline renders the
  //    state badges in order.
  test.skip(); // Implement after harness API helpers exist; see frontend/tests/* for patterns.
});
```

> **Implementer note:** the `frontend/tests/` directory has API
> bootstrap helpers similar to the Go harness — use them. If they don't
> exist for Pro promotion, add them.

- [ ] **Step 5: Run frontend dev + manual smoke**

```
cd frontend && pnpm dev
# In another shell: docker compose up postgres redis nats api
```

Open `http://localhost:3000/status/<test-slug>` and verify the timeline
renders with sample data inserted via psql.

- [ ] **Step 6: Run Playwright**

```
cd frontend && pnpm test:e2e -- incidents-timeline
```

Expected: PASS (or `skip` if helpers not yet wired — note in the task
which prerequisite is missing).

- [ ] **Step 7: Commit**

```bash
git add frontend/components/features/incidents/ \
  frontend/app/status/[slug]/page.tsx \
  frontend/tests/incidents-timeline.spec.ts
git commit -m "feat(status-page): render incident state timeline"
```

---

## Task 7: Dashboard incident management UI

**Files:**
- Create: `frontend/app/(main)/dashboard/incidents/[id]/page.tsx`
- Create: `frontend/components/features/incidents/incident-update-form.tsx`
- Create: `frontend/tests/incidents-flow.spec.ts`

- [ ] **Step 1: Build update form**

Create `frontend/components/features/incidents/incident-update-form.tsx`:

```tsx
"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useMutation, useQueryClient } from "@tanstack/react-query";

const STATES = ["investigating", "identified", "monitoring", "resolved"] as const;

export function IncidentUpdateForm({ incidentId }: { incidentId: number }) {
  const [state, setState] = useState<string>("investigating");
  const [body, setBody] = useState("");
  const qc = useQueryClient();

  const mutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/incidents/${incidentId}/state`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ state, body }),
      });
      if (!res.ok) throw new Error(`status ${res.status}`);
      return res.json();
    },
    onSuccess: () => {
      setBody("");
      qc.invalidateQueries({ queryKey: ["incident-updates", incidentId] });
    },
  });

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }}
      className="space-y-3"
    >
      <Select value={state} onValueChange={setState}>
        <SelectTrigger><SelectValue /></SelectTrigger>
        <SelectContent>
          {STATES.map((s) => <SelectItem key={s} value={s}>{s}</SelectItem>)}
        </SelectContent>
      </Select>
      <Textarea
        placeholder="What's happening?"
        value={body}
        onChange={(e) => setBody(e.target.value)}
        rows={3}
        required
      />
      <Button type="submit" disabled={mutation.isPending}>
        {mutation.isPending ? "Posting…" : "Post update"}
      </Button>
      {mutation.isError && (
        <p className="text-sm text-destructive">Failed to post update.</p>
      )}
    </form>
  );
}
```

- [ ] **Step 2: Build the incident detail page**

Create `frontend/app/(main)/dashboard/incidents/[id]/page.tsx`:

```tsx
import { sessionFetch } from "@/lib/session";
import { IncidentTimeline } from "@/components/features/incidents/incident-timeline";
import { IncidentUpdateForm } from "@/components/features/incidents/incident-update-form";

export default async function IncidentDetail({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const incident = await sessionFetch(`/api/incidents/${id}`).then(r => r.json());
  const updates  = await sessionFetch(`/api/incidents/${id}/updates`).then(r => r.json());
  return (
    <div className="container mx-auto px-4 py-8 max-w-3xl space-y-8">
      <h1 className="text-2xl font-semibold">{incident.title ?? incident.cause}</h1>
      <IncidentUpdateForm incidentId={Number(id)} />
      <IncidentTimeline updates={updates} />
    </div>
  );
}
```

> **Note:** `sessionFetch` is the existing SSR-side fetch helper
> (`frontend/lib/session.ts`). Adjust import path if different.

- [ ] **Step 3: Add link from monitor detail page**

In whichever monitor detail page renders incidents, change the incident
row to link to `/dashboard/incidents/<id>`.

- [ ] **Step 4: Add a `Get incident by id` route**

This requires `/api/incidents/{id}` to exist. Verify it does (Task 5 may
have added it; if not, add a `GET /incidents/{id}` to OpenAPI and a
handler — Pro-gated read OR public read scoped to the monitor's owner).

> **Implementer note:** for the dashboard, use the user-scoped read
> (auth required, owner check). Do NOT expose `GET /incidents/:id` as
> public — that's reserved for `/api/incidents/:id/updates` which is
> safe to read.

- [ ] **Step 5: Playwright spec**

Create `frontend/tests/incidents-flow.spec.ts`:

```ts
import { test, expect } from "@playwright/test";

test("pro user can post an incident update from dashboard", async ({ page }) => {
  // Bootstrap pro user + monitor + open manual incident via API.
  // Navigate to /dashboard/incidents/<id>.
  // Fill form: state=identified, body="DB up but slow".
  // Click "Post update". Assert the timeline now shows 2 entries.
  test.skip();
});
```

- [ ] **Step 6: Lint + commit**

```
cd frontend && pnpm lint && pnpm test:e2e -- incidents-flow
```

```
git add frontend/app/(main)/dashboard/incidents/ \
  frontend/components/features/incidents/incident-update-form.tsx \
  frontend/tests/incidents-flow.spec.ts
git commit -m "feat(dashboard): incident detail page with state-update form"
```

---

## Task 8: Atlassian importer service

**Files:**
- Create: `internal/app/atlassian_importer.go`
- Create: `internal/app/atlassian_importer_test.go`
- Create: `internal/app/testdata/atlassian_export_sample.json`

- [ ] **Step 1: Capture a sample Atlassian export**

Create `internal/app/testdata/atlassian_export_sample.json` with a
realistic Statuspage v1 JSON shape — a minimal version is:

```json
{
  "schema_version": "1.0",
  "page": {
    "name": "Acme Status",
    "url": "https://status.acme.com"
  },
  "components": [
    {
      "id": "abc",
      "name": "API",
      "url": "https://api.acme.com/health",
      "status": "operational"
    },
    {
      "id": "def",
      "name": "Dashboard",
      "url": "https://app.acme.com",
      "status": "operational"
    }
  ],
  "incidents": [
    {
      "id": "inc-1",
      "name": "API timeouts",
      "status": "resolved",
      "started_at": "2026-04-15T12:00:00Z",
      "resolved_at": "2026-04-15T13:30:00Z",
      "components": ["abc"],
      "incident_updates": [
        { "status": "investigating", "body": "Looking into API timeouts.",
          "created_at": "2026-04-15T12:05:00Z" },
        { "status": "identified", "body": "Upstream Postgres saturation.",
          "created_at": "2026-04-15T12:30:00Z" },
        { "status": "monitoring", "body": "Failed over, watching.",
          "created_at": "2026-04-15T13:00:00Z" },
        { "status": "resolved", "body": "All clear.",
          "created_at": "2026-04-15T13:30:00Z" }
      ]
    }
  ]
}
```

> **Implementer note:** Atlassian's actual export shape may differ — this
> JSON is the *contract* we accept. Document this shape in
> `docs/atlassian-import-format.md` as part of this task.

- [ ] **Step 2: Define types + parser**

Create `internal/app/atlassian_importer.go`:

```go
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// AtlassianImporter accepts a v1 Statuspage JSON export and creates the
// equivalent monitors + incidents under a target user.
type AtlassianImporter struct {
	monitors        port.MonitorRepo
	incidents       port.IncidentRepo
	incidentUpdates port.IncidentUpdateRepo
	txm             port.TxManager
}

func NewAtlassianImporter(
	monitors port.MonitorRepo, incidents port.IncidentRepo,
	updates port.IncidentUpdateRepo, txm port.TxManager,
) *AtlassianImporter {
	return &AtlassianImporter{monitors: monitors, incidents: incidents,
		incidentUpdates: updates, txm: txm}
}

type atlassianExport struct {
	SchemaVersion string             `json:"schema_version"`
	Page          atlassianPage      `json:"page"`
	Components    []atlassianComponent `json:"components"`
	Incidents     []atlassianIncident  `json:"incidents"`
}
type atlassianPage struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
type atlassianComponent struct {
	ID, Name, URL, Status string
}
type atlassianIncident struct {
	ID, Name, Status string
	StartedAt, ResolvedAt time.Time
	Components []string
	IncidentUpdates []atlassianIncidentUpdate `json:"incident_updates"`
}
type atlassianIncidentUpdate struct {
	Status, Body string
	CreatedAt time.Time `json:"created_at"`
}

// ImportResult summarises what was created so the UI can show a recap.
type ImportResult struct {
	MonitorsCreated  int
	IncidentsCreated int
	UpdatesCreated   int
}

// Import parses the export, validates the schema version, and creates
// the entities under userID. All-or-nothing inside one transaction.
func (i *AtlassianImporter) Import(ctx context.Context, userID uuid.UUID, src io.Reader) (ImportResult, error) {
	raw, err := io.ReadAll(src)
	if err != nil {
		return ImportResult{}, fmt.Errorf("read import body: %w", err)
	}
	var exp atlassianExport
	if err := json.Unmarshal(raw, &exp); err != nil {
		return ImportResult{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if exp.SchemaVersion != "1.0" {
		return ImportResult{}, fmt.Errorf("unsupported atlassian schema version: %q (only 1.0 accepted)", exp.SchemaVersion)
	}

	// Map components → monitors. URL is mandatory; skip components without one.
	componentToMonitor := map[string]uuid.UUID{}
	res := ImportResult{}
	// ... loop, create each monitor with type=http and config.url=component.URL.
	// ... loop incidents, create each as is_manual=true with title=Name,
	//     state mapped from Status, started_at from export.
	// ... loop incident_updates, create each with state mapped + body.
	// All inside i.txm.Run for atomicity.
	return res, nil
}

// mapState converts Atlassian status to our IncidentState. Atlassian
// uses "investigating", "identified", "monitoring", "resolved",
// "postmortem" — the first four map 1:1; postmortem we collapse to
// resolved (we don't model postmortem separately).
func mapState(s string) (domain.IncidentState, error) {
	switch s {
	case "investigating": return domain.IncidentStateInvestigating, nil
	case "identified":    return domain.IncidentStateIdentified, nil
	case "monitoring":    return domain.IncidentStateMonitoring, nil
	case "resolved":      return domain.IncidentStateResolved, nil
	case "postmortem":    return domain.IncidentStateResolved, nil
	}
	return "", fmt.Errorf("unknown atlassian status: %q", s)
}
```

- [ ] **Step 3: Write the test using the fixture**

Create `internal/app/atlassian_importer_test.go`:

```go
package app_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/stretchr/testify/require"
)

func TestAtlassianImporter_ImportsSampleExport(t *testing.T) {
	ctx := context.Background()
	monitors := newMockMonitorRepo(t)
	incidents := newMockIncidentRepo(t)
	updates := newMockIncidentUpdateRepo(t)
	txm := newPassThroughTxManager()

	imp := app.NewAtlassianImporter(monitors, incidents, updates, txm)

	f, err := os.Open("testdata/atlassian_export_sample.json")
	require.NoError(t, err)
	defer f.Close()

	uid := uuid.New()
	res, err := imp.Import(ctx, uid, f)
	require.NoError(t, err)
	require.Equal(t, 2, res.MonitorsCreated)   // 2 components
	require.Equal(t, 1, res.IncidentsCreated)  // 1 incident
	require.Equal(t, 4, res.UpdatesCreated)    // 4 updates on it

	// Assert state propagation: the incident should have ended at 'resolved'.
	require.Len(t, incidents.created, 1)
	// (assertion details depend on mock shape)
}

func TestAtlassianImporter_RejectsUnknownSchemaVersion(t *testing.T) {
	imp := app.NewAtlassianImporter(nil, nil, nil, nil)
	_, err := imp.Import(context.Background(), uuid.Nil,
		strings.NewReader(`{"schema_version":"99.0"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")
}
```

> **Implementer note:** the mocks (`newMockMonitorRepo` etc.) use the
> mockery-generated helpers in `internal/mocks/`. Read `.mockery.yaml`
> and use the conventions from existing tests in `internal/app/`.

- [ ] **Step 4: Run tests**

```
make test-short
```

Expected: PASS (after the `Import` body is filled in past the `// ...`
placeholders in Step 2).

- [ ] **Step 5: Lint + commit**

```
make lint
git add internal/app/atlassian_importer.go internal/app/atlassian_importer_test.go \
  internal/app/testdata/atlassian_export_sample.json \
  docs/atlassian-import-format.md
git commit -m "feat(import): atlassian statuspage v1 JSON importer service"
```

---

## Task 9: Atlassian importer HTTP endpoint (Pro-gated)

**Files:**
- Create: `internal/adapter/http/atlassian_import.go`
- Modify: `api/openapi.yaml`
- Modify: `internal/adapter/http/setup.go`
- Create: `tests/integration/api/atlassian_import_test.go`

- [ ] **Step 1: OpenAPI route**

Add to `api/openapi.yaml`:

```yaml
  /import/atlassian:
    post:
      operationId: importAtlassian
      security: [{ sessionAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/AtlassianExport"
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                required: [monitors_created, incidents_created, updates_created]
                properties:
                  monitors_created:  { type: integer }
                  incidents_created: { type: integer }
                  updates_created:   { type: integer }
        "400": { description: bad export }
        "402": { description: pro required }
```

`AtlassianExport` schema can be defined as `type: object, additionalProperties: true`
since we accept any JSON and validate inside the handler — keeps the
OpenAPI surface honest.

- [ ] **Step 2: Implement handler**

Create `internal/adapter/http/atlassian_import.go`:

```go
package httpadapter

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/app"
)

type AtlassianImportHandler struct {
	imp *app.AtlassianImporter
}

func NewAtlassianImportHandler(imp *app.AtlassianImporter) *AtlassianImportHandler {
	return &AtlassianImportHandler{imp: imp}
}

func (h *AtlassianImportHandler) Import(c *fiber.Ctx) error {
	uid := c.Locals("user_id").(uuid.UUID)
	res, err := h.imp.Import(c.UserContext(), uid, bytesReader(c.Body()))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    "IMPORT_FAILED",
				"message": err.Error(),
			},
		})
	}
	return c.JSON(fiber.Map{
		"monitors_created":  res.MonitorsCreated,
		"incidents_created": res.IncidentsCreated,
		"updates_created":   res.UpdatesCreated,
	})
}
```

- [ ] **Step 3: Wire route under proGroup**

```go
proGroup.Post("/import/atlassian", atlassianHandler.Import)
```

- [ ] **Step 4: Integration test**

Create `tests/integration/api/atlassian_import_test.go`:

```go
//go:build integration

package api_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAtlassianImport_proUserCreatesMonitorsAndIncidents(t *testing.T) {
	h := harness.New(t)
	pro := h.RegisterAndLogin(t, "pro@test.local", "pro", "Password123!")
	h.PromoteToPro(t, pro.UserID)

	body, err := os.ReadFile("../../../internal/app/testdata/atlassian_export_sample.json")
	if err != nil { t.Fatal(err) }

	resp := h.DoRaw(t, "POST", "/api/import/atlassian", pro.SessionCookie,
		"application/json", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
	// Verify counters via GET /monitors and /api/incidents/<id>/updates.
}

func TestAtlassianImport_freeUserGets402(t *testing.T) { /* similar */ }
```

- [ ] **Step 5: Run + commit**

```
make test-integration && make lint
git add api/openapi.yaml internal/api/gen/server.go internal/api/gen/types.go \
  frontend/lib/openapi-types.ts internal/adapter/http/atlassian_import.go \
  internal/adapter/http/setup.go tests/integration/api/atlassian_import_test.go
git commit -m "feat(api): /import/atlassian endpoint (Pro-gated)"
```

---

## Task 10: Atlassian importer frontend page

**Files:**
- Create: `frontend/app/(main)/import/atlassian/page.tsx`
- Create: `frontend/tests/atlassian-import.spec.ts`

- [ ] **Step 1: Build the import form**

Create `frontend/app/(main)/import/atlassian/page.tsx`:

```tsx
"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";

export default function AtlassianImportPage() {
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<{ monitors_created: number; incidents_created: number; updates_created: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;
    setBusy(true); setError(null); setResult(null);
    try {
      const json = await file.text();
      const res = await fetch("/api/import/atlassian", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: json,
      });
      if (res.status === 402) throw new Error("Pro subscription required for Atlassian import.");
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      }
      setResult(await res.json());
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "import failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="container mx-auto px-4 py-12 max-w-2xl">
      <h1 className="text-3xl font-bold tracking-tight">Import from Atlassian Statuspage</h1>
      <p className="mt-3 text-sm text-muted-foreground">
        Export your Statuspage configuration as JSON and upload it here. We'll
        create equivalent monitors, incidents, and updates in one click.
      </p>

      <form onSubmit={submit} className="mt-8 space-y-4">
        <input
          type="file"
          accept="application/json"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          className="block w-full text-sm"
        />
        <Button type="submit" disabled={!file || busy}>
          {busy ? "Importing…" : "Import"}
        </Button>
      </form>

      {error && (
        <p className="mt-6 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </p>
      )}

      {result && (
        <div className="mt-6 rounded-md border border-emerald-500/40 bg-emerald-500/5 px-4 py-3 text-sm">
          Imported <strong>{result.monitors_created}</strong> monitors,{" "}
          <strong>{result.incidents_created}</strong> incidents,{" "}
          <strong>{result.updates_created}</strong> updates.
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Playwright spec**

Create `frontend/tests/atlassian-import.spec.ts` covering happy path
(upload sample, see "imported X monitors") and error path (upload garbage,
see error message). Use `page.setInputFiles` for the upload.

- [ ] **Step 3: Lint + commit**

```
cd frontend && pnpm lint && pnpm test:e2e -- atlassian-import
git add frontend/app/(main)/import/atlassian/ frontend/tests/atlassian-import.spec.ts
git commit -m "feat(import): atlassian import page in dashboard"
```

---

## Task 11: LemonSqueezy product setup (operational checklist)

This is **not a code task** — it's a runbook step that must be done before
Tasks 12–13 work end-to-end. Track here so it isn't forgotten.

- [ ] **Step 1: In LemonSqueezy dashboard, create product `PingCast Pro`**
  - Two variants:
    - `Founder · $9/mo` (variant id → save to `LEMONSQUEEZY_FOUNDER_VARIANT_ID`)
    - `Retail · $19/mo` (variant id → save to `LEMONSQUEEZY_RETAIL_VARIANT_ID`)
  - Recurring monthly. No trial in v1.
  - Webhook URL: `https://<new-domain>/webhook/lemonsqueezy`
  - Webhook secret: copy to `LEMONSQUEEZY_WEBHOOK_SECRET` env

- [ ] **Step 2: Update `.env.example`**

Add (without real secrets):

```
# server-side (read by Go API)
LEMONSQUEEZY_WEBHOOK_SECRET=
LEMONSQUEEZY_FOUNDER_VARIANT_ID=
LEMONSQUEEZY_RETAIL_VARIANT_ID=
FOUNDER_CAP=100

# client-side (Next.js inlines NEXT_PUBLIC_* into the bundle — these
# are checkout URLs the browser opens, so they must be public)
NEXT_PUBLIC_LEMONSQUEEZY_FOUNDER_URL=
NEXT_PUBLIC_LEMONSQUEEZY_RETAIL_URL=
NEXT_PUBLIC_PLAUSIBLE_DOMAIN=
NEXT_PUBLIC_PLAUSIBLE_SRC=
```

- [ ] **Step 3: Update `internal/config/config.go`**

Read the existing config struct and add the new fields with their env
keys. Mirror the convention in the file.

- [ ] **Step 4: Update `docker-compose.yml`**

Pass the new envs through to the API service.

- [ ] **Step 5: Commit**

```
git add .env.example internal/config/config.go docker-compose.yml
git commit -m "chore(config): add LemonSqueezy Pro variant + founder cap envs"
```

---

## Task 12: Founder-price cap query + endpoint

**Files:**
- Modify: `internal/sqlc/queries/users.sql`
- Modify: `internal/port/repository.go`
- Modify: `internal/adapter/postgres/user_repo.go`
- Create: `internal/app/billing.go`
- Create: `internal/app/billing_test.go`
- Modify: `api/openapi.yaml`
- Modify: `internal/adapter/http/setup.go`
- Create: `internal/adapter/http/billing.go`
- Create: `tests/integration/api/founder_cap_test.go`

- [ ] **Step 1: Add the count query**

Append to `internal/sqlc/queries/users.sql`:

```sql
-- name: CountActiveFounderSubscriptions :one
SELECT COUNT(*)::bigint AS active_founder_count
FROM users
WHERE plan = 'pro'
  AND lemonsqueezy_subscription_id IS NOT NULL
  AND lemonsqueezy_subscription_id LIKE 'founder:%';
```

> **Implementer note:** the simplest way to mark a subscription as the
> founder variant is a prefix on `lemonsqueezy_subscription_id` written
> by the webhook when it sees the founder variant id. If a separate
> column is preferred, add it to the users table with a migration —
> include it in this Task.

Alternative (cleaner) — add `subscription_variant TEXT` to `users`:

```sql
-- migration 019
ALTER TABLE users ADD COLUMN subscription_variant TEXT;
-- query:
SELECT COUNT(*)::bigint FROM users
WHERE plan = 'pro' AND subscription_variant = 'founder';
```

Pick the cleaner option (migration + dedicated column). Add a `019_*.sql`
migration accordingly.

- [ ] **Step 2: Regenerate**

```
make generate-go
```

- [ ] **Step 3: Add port + repo method**

In `internal/port/repository.go` extend `UserRepo`:

```go
CountActiveFounderSubscriptions(ctx context.Context) (int64, error)
SetSubscriptionVariant(ctx context.Context, id uuid.UUID, variant string) error
```

Implement in `internal/adapter/postgres/user_repo.go`.

- [ ] **Step 4: Billing service**

Create `internal/app/billing.go`:

```go
package app

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/port"
)

type BillingService struct {
	users      port.UserRepo
	founderCap int
}

func NewBillingService(users port.UserRepo, founderCap int) *BillingService {
	return &BillingService{users: users, founderCap: founderCap}
}

// FounderAvailable reports whether the founder-price variant is still open.
// Cached at the caller layer — this query is cheap (COUNT on indexed col)
// but called on every pricing-page render. The HTTP handler caches for 60s.
func (s *BillingService) FounderAvailable(ctx context.Context) (bool, int64, error) {
	used, err := s.users.CountActiveFounderSubscriptions(ctx)
	if err != nil { return false, 0, err }
	return used < int64(s.founderCap), used, nil
}
```

Test it with mocks in `billing_test.go` — table-driven across used=0,
used=99, used=100, used=999.

- [ ] **Step 5: HTTP endpoint**

Add to `api/openapi.yaml`:

```yaml
  /billing/founder-status:
    get:
      operationId: getFounderStatus
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                required: [available, used, cap]
                properties:
                  available: { type: boolean }
                  used:      { type: integer, format: int64 }
                  cap:       { type: integer }
```

Create `internal/adapter/http/billing.go`:

```go
package httpadapter

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/app"
)

type BillingHandler struct {
	svc *app.BillingService
	cap int

	mu      sync.RWMutex
	cached  struct{ avail bool; used int64 }
	expires time.Time
}

func NewBillingHandler(svc *app.BillingService, cap int) *BillingHandler {
	return &BillingHandler{svc: svc, cap: cap}
}

func (h *BillingHandler) FounderStatus(c *fiber.Ctx) error {
	h.mu.RLock()
	if time.Now().Before(h.expires) {
		v := h.cached
		h.mu.RUnlock()
		return c.JSON(fiber.Map{"available": v.avail, "used": v.used, "cap": h.cap})
	}
	h.mu.RUnlock()

	avail, used, err := h.svc.FounderAvailable(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code":"BILLING_LOOKUP_FAILED","message":"could not load"},
		})
	}
	h.mu.Lock()
	h.cached.avail, h.cached.used = avail, used
	h.expires = time.Now().Add(60 * time.Second)
	h.mu.Unlock()
	return c.JSON(fiber.Map{"available": avail, "used": used, "cap": h.cap})
}
```

Wire `app.Get("/api/billing/founder-status", billingHandler.FounderStatus)`
in setup.go (no auth required).

- [ ] **Step 6: Update webhook to set subscription_variant**

Modify `internal/adapter/http/webhook.go` `subscription_created` branch:

```go
variantID := webhook.Data.Attributes.VariantID  // add to struct
variant := "retail"
if fmt.Sprintf("%d", variantID) == h.cfg.FounderVariantID {
	variant = "founder"
}
// after UpgradeToPro, set variant:
_ = h.auth.SetSubscriptionVariant(c.UserContext(), user.ID, variant)
```

> **Implementer note:** the variant comparison must be against the
> string env. Add `VariantID` to the `lemonSqueezyWebhook` struct's
> `Attributes`. The `auth.SetSubscriptionVariant` method needs to be
> added to AuthService — wraps `users.SetSubscriptionVariant`.

- [ ] **Step 7: Integration test**

Create `tests/integration/api/founder_cap_test.go` covering:
- empty DB → `available: true, used: 0, cap: 100`
- after 100 promoted founder users → `available: false, used: 100`
- 60s caching behaviour (call twice within window, assert one DB hit
  via test logger or skip)

- [ ] **Step 8: Lint + commit**

```
make test && make lint
git add internal/database/migrations/019_add_subscription_variant.sql \
  internal/sqlc/queries/users.sql internal/sqlc/gen/users.sql.go \
  internal/sqlc/gen/models.go internal/port/repository.go \
  internal/adapter/postgres/user_repo.go internal/app/billing.go \
  internal/app/billing_test.go internal/app/auth.go \
  api/openapi.yaml internal/api/gen/server.go internal/api/gen/types.go \
  internal/adapter/http/billing.go internal/adapter/http/webhook.go \
  internal/adapter/http/setup.go tests/integration/api/founder_cap_test.go \
  frontend/lib/openapi-types.ts
git commit -m "feat(billing): founder-price cap (100) with /api/billing/founder-status"
```

---

## Task 13: Upgrade-to-Pro dashboard button + checkout flow

**Files:**
- Create: `frontend/components/features/billing/upgrade-button.tsx`
- Modify: `frontend/app/(main)/dashboard/page.tsx`
- Create: `frontend/tests/upgrade-flow.spec.ts`

- [ ] **Step 1: Build the button**

Create `frontend/components/features/billing/upgrade-button.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";

const FOUNDER_CHECKOUT_URL = process.env.NEXT_PUBLIC_LEMONSQUEEZY_FOUNDER_URL!;
const RETAIL_CHECKOUT_URL  = process.env.NEXT_PUBLIC_LEMONSQUEEZY_RETAIL_URL!;

export function UpgradeButton({ plan }: { plan: string }) {
  const [founderAvailable, setFounderAvailable] = useState<boolean | null>(null);

  useEffect(() => {
    if (plan === "pro") return;
    fetch("/api/billing/founder-status")
      .then(r => r.json())
      .then(d => setFounderAvailable(d.available));
  }, [plan]);

  if (plan === "pro") return null;
  if (founderAvailable === null) return null;

  const url = founderAvailable ? FOUNDER_CHECKOUT_URL : RETAIL_CHECKOUT_URL;
  const price = founderAvailable ? "$9" : "$19";
  const label = founderAvailable ? "Upgrade — founder's price" : "Upgrade to Pro";

  return (
    <Link
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      onClick={() => window.plausible?.("pro_checkout_clicked", { props: { variant: founderAvailable ? "founder" : "retail" } })}
      className={buttonVariants({ size: "lg" })}
    >
      {label} · {price}/mo
    </Link>
  );
}
```

- [ ] **Step 2: Mount on dashboard**

Add the `<UpgradeButton plan={user.plan} />` to the dashboard page. Read
the current `dashboard/page.tsx` to understand how the user is loaded.

- [ ] **Step 3: Playwright**

Create `frontend/tests/upgrade-flow.spec.ts` covering:
- free user sees "Upgrade — founder's price · $9/mo" when cap not reached
- free user sees "Upgrade to Pro · $19/mo" after cap is reached (mock the
  `/api/billing/founder-status` response or seed 100 founder users)
- pro user sees no button

- [ ] **Step 4: Lint + commit**

```
cd frontend && pnpm lint && pnpm test:e2e -- upgrade-flow
git add frontend/components/features/billing/ frontend/app/(main)/dashboard/page.tsx \
  frontend/tests/upgrade-flow.spec.ts .env.example
git commit -m "feat(dashboard): upgrade button with founder-price awareness"
```

---

## Task 14: Pricing page rewrite

**Files:**
- Modify: `frontend/app/(main)/pricing/page.tsx`

- [ ] **Step 1: Rewrite pricing/page.tsx**

Replace the current two-column layout with three columns: Free / **Pro
$9 founder** / Self-hosted. Pro card shows the founder-status pill ("X
of 100 founder seats left") fetched from `/api/billing/founder-status`.

The Pro card structure (drop into the existing `Plan` component):

```tsx
<Plan
  icon={<Sparkles className="h-5 w-5" />}
  name="Pro"
  price="$9"
  priceHint="/ mo · founder's price"
  cta="Start Pro"
  href={process.env.NEXT_PUBLIC_LEMONSQUEEZY_FOUNDER_URL!}
  primary
  features={[
    "50 monitors · 30s interval",
    "Custom domain (status.yourcompany.com)",
    "Branded status page (logo, color, no watermark)",
    "Incident updates with state machine",
    "Email subscriptions for your customers",
    "Atlassian Statuspage importer (1-click)",
    "SVG status badge for READMEs",
    "Embeddable JS incident widget",
    "SSL expiry warnings",
    "1 year of incident history + CSV export",
    "Maintenance windows",
    "Priority email support",
  ]}
  footnote="$9/mo locked for the first 100 customers. Retail price is $19/mo. Cancel anytime."
/>
```

Add a small live counter component above the Pro card showing
"X of 100 founder seats left" — or hide if `available === false`,
swap CTA url + price to retail.

- [ ] **Step 2: Lint + commit**

```
cd frontend && pnpm lint
git add frontend/app/(main)/pricing/page.tsx
git commit -m "feat(pricing): three-column layout with Pro \$9 founder/\$19 retail"
```

---

## Task 15: Landing copy + section reorder

**Files:**
- Modify: `frontend/app/(main)/page.tsx`
- Create: `frontend/components/site/landing/hero.tsx`
- Create: `frontend/components/site/landing/trust-bar.tsx`
- Create: `frontend/components/site/landing/why-not-atlassian.tsx`
- Create: `frontend/components/site/landing/features-grid.tsx`
- Create: `frontend/components/site/landing/comparison-table.tsx`
- Create: `frontend/components/site/landing/faq.tsx`
- Create: `frontend/components/site/landing/final-cta.tsx`

- [ ] **Step 1: Extract Hero into its own client component**

`frontend/components/site/landing/hero.tsx` — keep Framer Motion here
(client island). New tagline:

```tsx
<h1>
  Status pages для SaaS,{" "}
  <span className="bg-gradient-to-r from-blue-600 via-cyan-500 to-teal-500 bg-clip-text text-transparent">
    в 3 раза дешевле Atlassian.
  </span>
</h1>
```

(English-default version for now; RU mirror is Sprint 4.)

- [ ] **Step 2-7:** Build each section component as a client island
  (Framer Motion-using sections) or server component (static sections).

- [ ] **Step 8: Recompose page.tsx as a server component**

Remove top-level `"use client"`. Import each section component. Order
per spec §7.

- [ ] **Step 9: Playwright smoke**

Update existing landing-page Playwright spec to assert new tagline +
section order.

- [ ] **Step 10: Commit**

```
git add frontend/components/site/landing/ frontend/app/(main)/page.tsx \
  frontend/tests/landing-smoke.spec.ts
git commit -m "feat(landing): rewrite around status-page positioning"
```

---

## Task 16: Landing SSR conversion (Framer islands)

This is the critical SEO step. Most of it happens in Task 15 by
extracting Framer Motion into client components. Verify here that:

- [ ] **Step 1: Inspect SSR HTML for new tagline**

Run dev server, view source on `/`, confirm the new tagline string is
present in the SSR HTML (not loaded by JS). If it's missing, the section
hasn't been server-rendered.

- [ ] **Step 2: Lighthouse SEO score check**

```
cd frontend && pnpm dev
npx lighthouse http://localhost:3000 --only-categories=seo --output=json --output-path=/tmp/lh.json
```

Target: SEO score ≥ 95.

- [ ] **Step 3: Commit only if changes were needed**

(Probably a no-op task if Task 15 was done correctly.)

---

## Task 17: JSON-LD enrichment

**Files:**
- Modify: `frontend/app/layout.tsx`
- Create: `frontend/components/seo/faq-jsonld.tsx`
- Create: `frontend/components/seo/breadcrumb-jsonld.tsx`
- Create: `frontend/components/seo/organization-jsonld.tsx`

- [ ] **Step 1: Organization JSON-LD in layout**

In `frontend/app/layout.tsx`, add a `<script type="application/ld+json">`
tag with:

```json
{
  "@context": "https://schema.org",
  "@type": "Organization",
  "name": "PingCast",
  "url": "https://pingcast.io",
  "logo": "https://pingcast.io/favicon.png",
  "sameAs": ["https://github.com/kirillinakin/pingcast"]
}
```

- [ ] **Step 2: FAQPage JSON-LD on landing**

In the FAQ section component (Task 15), build a JSON-LD block from the
same `[{q, a}]` array used to render the visible FAQ. Render once per
page. Validate via https://validator.schema.org/ before commit.

- [ ] **Step 3: BreadcrumbList helper**

For nested routes (later sprints add /alternatives/* etc.), create a
reusable `<BreadcrumbList items={[{name, url}, ...]} />` component that
emits the JSON-LD.

- [ ] **Step 4: Commit**

```
git add frontend/app/layout.tsx frontend/components/seo/
git commit -m "feat(seo): organization, faqpage, breadcrumb json-ld"
```

---

## Task 18: Domain normalization

**Files:**
- Modify: `frontend/app/(main)/page.tsx`
- Modify: `frontend/app/(main)/pricing/page.tsx`
- Modify: `README.md`
- Modify: `.env.example`
- Search-replace across repo

- [ ] **Step 1: Verify the new domain is registered**

(Operational — done in §9 of the spec.) Set the canonical domain in:

```
NEXT_PUBLIC_SITE_URL=https://pingcast.io
BASE_URL=https://pingcast.io
```

- [ ] **Step 2: Search-replace stale references**

```
rg -l "pingcast\.kirillin\.tech|kirillin\.tech/pingcast|pingcast\.io"
```

For each match, decide: keep `process.env.NEXT_PUBLIC_SITE_URL`, or hard-code
the new domain (only for the README and JSON-LD where env-templating doesn't
apply).

- [ ] **Step 3: Verify sitemap, robots, OG generation**

Hit `/sitemap.xml`, `/robots.txt`, `/opengraph-image` in dev — confirm
they emit the new domain.

- [ ] **Step 4: Commit**

```
git add -A  # only after manual review
git commit -m "chore(domain): normalize all references to pingcast.io"
```

---

## Task 19: Plausible install + funnel events

**Files:**
- Create: `frontend/components/analytics/plausible.tsx`
- Create: `frontend/lib/analytics.ts`
- Modify: `frontend/app/layout.tsx`
- Modify: `frontend/components/features/billing/upgrade-button.tsx`
- Modify: `frontend/app/(main)/register/page.tsx` (or wherever signup completes)

- [ ] **Step 1: Pick hosted vs self-hosted Plausible**

If self-hosting, deploy a Plausible container alongside the existing
docker-compose. If hosted, sign up at plausible.io and copy the script
src.

- [ ] **Step 2: Mount the script**

Create `frontend/components/analytics/plausible.tsx`:

```tsx
import Script from "next/script";

export function PlausibleScript() {
  if (process.env.NODE_ENV !== "production") return null;
  const domain = process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN;
  if (!domain) return null;
  return (
    <Script
      defer
      data-domain={domain}
      src={process.env.NEXT_PUBLIC_PLAUSIBLE_SRC ?? "https://plausible.io/js/script.js"}
      strategy="afterInteractive"
    />
  );
}
```

Mount in `frontend/app/layout.tsx`.

- [ ] **Step 3: Custom event helper**

Create `frontend/lib/analytics.ts`:

```ts
type Plausible = (event: string, options?: { props?: Record<string, string> }) => void;

declare global {
  interface Window { plausible?: Plausible; }
}

export function track(event: string, props?: Record<string, string>) {
  if (typeof window !== "undefined" && window.plausible) {
    window.plausible(event, { props });
  }
}
```

- [ ] **Step 4: Wire `pro_checkout_clicked`**

The `UpgradeButton` already calls `window.plausible?.("pro_checkout_clicked", ...)`
in Task 13. Replace with `track("pro_checkout_clicked", { variant })`.

- [ ] **Step 5: Wire `register_completed`**

In whichever register flow completes (likely a page that redirects to
`/dashboard` after a successful POST), call `track("register_completed")`
on the success path.

- [ ] **Step 6: Verify in dev (or staging)**

If hosted Plausible, the dashboard will show events shortly after they fire.

- [ ] **Step 7: Commit**

```
git add frontend/components/analytics/ frontend/lib/analytics.ts \
  frontend/app/layout.tsx frontend/components/features/billing/upgrade-button.tsx \
  frontend/app/(main)/register/ .env.example
git commit -m "feat(analytics): plausible + pro_checkout_clicked + register_completed events"
```

---

## Task 20: Bootstrap-proof outreach (operational)

Not engineering — track here so it ships in Sprint 1.

- [ ] **Step 1: Build the prospect list (10 indie SaaS targets)**

Pick targets that meet ALL of: (a) public Twitter/X presence, (b) <50
employees, (c) currently use no status page or a self-rolled one, (d)
the founder is reachable. Store the list in
`docs/marketing/bootstrap-prospects.md` (gitignored — this is internal).

- [ ] **Step 2: Outreach template**

Draft a personal email template:

```
Subject: A free year of Pro on PingCast (no strings, just a logo)

Hey {name},

I'm Kirill — I just shipped PingCast, an open-source uptime + branded
status page tool aimed at indie SaaS like {company}. I want a real logo
wall instead of fake "trusted by" placeholders, so I'm offering 10
founders a free year of Pro ($9/mo retail) in exchange for permission
to display your logo on pingcast.io and a one-line quote later.

Pro includes custom domain (status.{their-domain}.com), branding,
incident updates, Atlassian importer. No credit card. If you outgrow
us, the whole stack is MIT — just self-host.

Worth 5 minutes? Reply yes and I'll provision your account.

— Kirill
```

- [ ] **Step 3: Send 10 emails over 3 days**

Stagger so replies don't pile up. Track responses in the prospect doc.

- [ ] **Step 4: For each acceptance**

- Provision their account (Pro plan via DB update + a year-long stub
  subscription_variant=`gift`).
- Get their logo SVG + 1-line quote.
- When ≥ 5 accepted, swap the landing "no logo wall" copy for the real
  wall (Task 15 section §10).

---

## Self-review

Run through each spec section vs. plan task:

- §5 Pricing: Tasks 11–14 cover Free + Pro + Self-host page rendering and
  founder-price machinery ✓
- §6 Sprint 1 features: incident states (2,3,4), Atlassian importer (8,9,10),
  LemonSqueezy product (11,12,13), public stats endpoint (1) ✓
- §7 Landing rewrite: Tasks 15, 16 (SSR conversion), 17 (JSON-LD) ✓
- §8 SEO infra (sitemap, robots, OG, footer): partial — sitemap update
  for new public routes is done by Tasks 1, 5, 9, 12; the footer
  redesign and full SEO-page generation are Sprint 4. **Add note in
  Sprint 4 outline that footer comes there.**
- §9 Domain + Plausible: Tasks 18, 19 ✓
- §9 distribution: Sprint 5 runbook
- §11 risks: Pro-gate middleware (Task 0) addresses the structural risk;
  founder-price trust addressed by encoding variant in DB (Task 12)

Placeholder scan: zero "TBD"/"TODO" in the steps; only explicit
implementer notes that surface known unknowns about the existing codebase
(e.g. "verify the precise field on monitors").

Type consistency: `IncidentState`, `IncidentUpdate`, `ChangeIncidentStateInput`,
`CreateIncidentUpdateInput`, `CreateIncidentInput`, `PublicStats` —
referenced consistently across Tasks 2–7.

---

## Out-of-sprint deferrals

- Status page custom domain — Sprint 3
- Branded status page (logo, color upload) — Sprint 2
- SSL warnings — Sprint 2
- Email subscriptions — Sprint 3
- SVG badge — Sprint 2
- JS widget — Sprint 3
- Maintenance windows — Sprint 2
- 1-year retention + CSV export — Sprint 2
- Multi-monitor groups — Sprint 3
- Footer redesign + 12 SEO pages — Sprint 4
- next-intl + RU mirror — Sprint 4
- All distribution (Habr, vc.ru, IH, etc.) — Sprint 5
