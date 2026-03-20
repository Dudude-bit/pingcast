# PingCast Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build PingCast — an uptime monitoring SaaS with HTTP checks, Telegram/email alerts, dashboards, and public status pages.

**Architecture:** Go monolith. Fiber HTTP framework. API-first: OpenAPI spec → oapi-codegen generates Fiber server interfaces and types. sqlc generates type-safe DB queries from SQL. HTMX frontend calls JSON API, page handlers render HTML templates.

**Tech Stack:** Go, Fiber, sqlc, oapi-codegen, HTMX, PostgreSQL, pgx, bcrypt, slog, Telegram Bot API, Lemon Squeezy, Docker

**Spec:** `docs/superpowers/specs/2026-03-19-pingcast-uptime-monitoring-design.md`

---

## File Structure

```
pingcast/
├── cmd/
│   └── pingcast/
│       └── main.go                    # entry point, wires all components
├── api/
│   └── openapi.yaml                   # OpenAPI 3.0 specification
├── internal/
│   ├── config/
│   │   └── config.go                  # env-based configuration
│   ├── database/
│   │   ├── database.go                # PG connection + migration runner
│   │   └── migrations/
│   │       ├── 001_create_users.sql
│   │       ├── 002_create_monitors.sql
│   │       ├── 003_create_check_results.sql
│   │       ├── 004_create_incidents.sql
│   │       └── 005_create_sessions.sql
│   ├── sqlc/
│   │   ├── sqlc.yaml                  # sqlc configuration
│   │   ├── queries/
│   │   │   ├── users.sql
│   │   │   ├── monitors.sql
│   │   │   ├── check_results.sql
│   │   │   ├── incidents.sql
│   │   │   └── sessions.sql
│   │   └── gen/                       # sqlc-generated code (DO NOT EDIT)
│   │       ├── db.go
│   │       ├── models.go
│   │       ├── users.sql.go
│   │       ├── monitors.sql.go
│   │       ├── check_results.sql.go
│   │       ├── incidents.sql.go
│   │       └── sessions.sql.go
│   ├── api/
│   │   └── gen/                       # oapi-codegen generated code (DO NOT EDIT)
│   │       ├── server.go              # Fiber server interface
│   │       └── types.go               # Request/response types
│   ├── auth/
│   │   ├── service.go                 # register, login, session validation
│   │   ├── service_test.go
│   │   ├── middleware.go              # Fiber middleware for session auth
│   │   └── ratelimit.go              # in-memory login rate limiter
│   ├── checker/
│   │   ├── client.go                  # configured HTTP client
│   │   ├── client_test.go
│   │   ├── scheduler.go              # timer-based scheduler
│   │   ├── scheduler_test.go
│   │   ├── worker.go                  # worker pool
│   │   └── hostlimit.go              # per-host semaphore
│   ├── notifier/
│   │   ├── listener.go               # PG LISTEN/NOTIFY subscriber
│   │   ├── telegram.go               # Telegram bot sender + /start handler
│   │   ├── telegram_test.go
│   │   ├── email.go                   # email sender via SMTP
│   │   └── email_test.go
│   ├── handler/
│   │   ├── server.go                  # implements oapi-codegen StrictServerInterface
│   │   ├── server_test.go
│   │   ├── pages.go                   # HTML page handlers (dashboard, login, etc.)
│   │   ├── webhook.go                 # Lemon Squeezy + Telegram webhooks
│   │   └── setup.go                   # Fiber app setup, routes, middleware
│   └── web/
│       ├── embed.go                   # go:embed for templates + static
│       ├── templates/
│       │   ├── layout.html
│       │   ├── landing.html
│       │   ├── login.html
│       │   ├── register.html
│       │   ├── dashboard.html
│       │   ├── monitor_detail.html
│       │   ├── monitor_form.html
│       │   └── statuspage.html
│       └── static/
│           ├── css/
│           │   └── style.css
│           └── js/
│               └── chart-init.js
├── tools/
│   └── generate.go                    # //go:generate directives
├── Dockerfile
├── docker-compose.yml
├── .gitignore
├── go.mod
└── go.sum
```

---

## Task 1: Project Scaffolding & Config

**Files:**
- Create: `go.mod`
- Create: `cmd/pingcast/main.go`
- Create: `internal/config/config.go`
- Create: `.gitignore`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/kirillinakin/GolandProjects/p-project
go mod init github.com/kirillinakin/pingcast
```

- [ ] **Step 2: Install core dependencies**

```bash
go get github.com/gofiber/fiber/v2
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/google/uuid
go get golang.org/x/crypto/bcrypt
```

- [ ] **Step 3: Create config**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port                       int
	DatabaseURL                string
	TelegramToken              string
	SMTPHost                   string
	SMTPPort                   int
	SMTPUser                   string
	SMTPPass                   string
	SMTPFrom                   string
	LemonSqueezyWebhookSecret string
	BaseURL                    string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &Config{
		Port:                       port,
		DatabaseURL:                dbURL,
		TelegramToken:              os.Getenv("TELEGRAM_BOT_TOKEN"),
		SMTPHost:                   os.Getenv("SMTP_HOST"),
		SMTPPort:                   smtpPort,
		SMTPUser:                   os.Getenv("SMTP_USER"),
		SMTPPass:                   os.Getenv("SMTP_PASS"),
		SMTPFrom:                   getEnv("SMTP_FROM", "noreply@pingcast.io"),
		LemonSqueezyWebhookSecret: os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		BaseURL:                    getEnv("BASE_URL", "http://localhost:8080"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Create minimal main.go**

Create `cmd/pingcast/main.go`:

```go
package main

import (
	"log/slog"
	"os"

	"github.com/kirillinakin/pingcast/internal/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting pingcast", "port", cfg.Port)
}
```

- [ ] **Step 5: Create .gitignore**

Create `.gitignore`:

```
/pingcast
*.exe
.env
.env.*
.idea/
.vscode/
*.swp
.DS_Store
docker-compose.override.yml
```

- [ ] **Step 6: Verify and commit**

```bash
go build ./cmd/pingcast/
git init
git add go.mod cmd/ internal/config/ .gitignore
git commit -m "feat: project scaffolding with Fiber, config, and entry point"
```

---

## Task 2: Database Connection & Migrations

**Files:**
- Create: `internal/database/database.go`
- Create: `internal/database/migrations/001_create_users.sql`
- Create: `internal/database/migrations/002_create_monitors.sql`
- Create: `internal/database/migrations/003_create_check_results.sql`
- Create: `internal/database/migrations/004_create_incidents.sql`
- Create: `internal/database/migrations/005_create_sessions.sql`

- [ ] **Step 1: Create database connection module**

Create `internal/database/database.go`:

```go
package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version := 0
		fmt.Sscanf(entry.Name(), "%d_", &version)
		if version == 0 {
			continue
		}

		var count int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if count > 0 {
			continue
		}

		content, err := fs.ReadFile(migrationsFS, "migrations/"+entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", version, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("execute migration %d: %w", version, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %d: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}

		slog.Info("applied migration", "version", version, "file", entry.Name())
	}

	return nil
}
```

- [ ] **Step 2: Create migration files**

Create `internal/database/migrations/001_create_users.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(30) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    tg_chat_id BIGINT,
    plan VARCHAR(10) NOT NULL DEFAULT 'free',
    lemon_squeezy_customer_id VARCHAR(255),
    lemon_squeezy_subscription_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Create `internal/database/migrations/002_create_monitors.sql`:

```sql
CREATE TABLE monitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(2048) NOT NULL,
    method VARCHAR(10) NOT NULL DEFAULT 'GET',
    interval_seconds INT NOT NULL DEFAULT 300,
    expected_status INT NOT NULL DEFAULT 200,
    keyword VARCHAR(255),
    alert_after_failures INT NOT NULL DEFAULT 3,
    is_paused BOOLEAN NOT NULL DEFAULT FALSE,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    current_status VARCHAR(10) NOT NULL DEFAULT 'unknown',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_monitors_user_id ON monitors (user_id);
```

Create `internal/database/migrations/003_create_check_results.sql`:

```sql
CREATE TABLE check_results (
    id BIGSERIAL PRIMARY KEY,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status VARCHAR(10) NOT NULL,
    status_code INT,
    response_time_ms INT NOT NULL,
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_check_results_monitor_checked ON check_results (monitor_id, checked_at);
```

Create `internal/database/migrations/004_create_incidents.sql`:

```sql
CREATE TABLE incidents (
    id BIGSERIAL PRIMARY KEY,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    cause TEXT NOT NULL
);

CREATE INDEX idx_incidents_monitor_id ON incidents (monitor_id);
CREATE INDEX idx_incidents_monitor_started ON incidents (monitor_id, started_at);
```

Create `internal/database/migrations/005_create_sessions.sql`:

```sql
CREATE TABLE sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
```

- [ ] **Step 3: Wire database into main.go**

Update `cmd/pingcast/main.go`:

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	slog.Info("pingcast started", "port", cfg.Port)
	<-ctx.Done()
	slog.Info("shutting down")
}
```

- [ ] **Step 4: Verify and commit**

```bash
go mod tidy && go build ./cmd/pingcast/
git add internal/database/ cmd/ go.mod go.sum
git commit -m "feat: database connection and SQL migrations"
```

---

## Task 3: sqlc Setup & Query Generation

**Files:**
- Create: `internal/sqlc/sqlc.yaml`
- Create: `internal/sqlc/queries/users.sql`
- Create: `internal/sqlc/queries/monitors.sql`
- Create: `internal/sqlc/queries/check_results.sql`
- Create: `internal/sqlc/queries/incidents.sql`
- Create: `internal/sqlc/queries/sessions.sql`
- Generated: `internal/sqlc/gen/` (all files)
- Create: `tools/generate.go`

- [ ] **Step 1: Install sqlc**

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

- [ ] **Step 2: Create sqlc config**

Create `internal/sqlc/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries"
    schema: "../database/migrations"
    gen:
      go:
        package: "gen"
        out: "gen"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
        overrides:
          - db_type: "uuid"
            go_type:
              import: "github.com/google/uuid"
              type: "UUID"
          - db_type: "timestamptz"
            go_type:
              import: "time"
              type: "Time"
          - db_type: "pg_catalog.varchar"
            nullable: true
            go_type:
              import: ""
              type: "*string"
              pointer: false
          - db_type: "text"
            nullable: true
            go_type:
              import: ""
              type: "*string"
              pointer: false
```

- [ ] **Step 3: Create user queries**

Create `internal/sqlc/queries/users.sql`:

```sql
-- name: CreateUser :one
INSERT INTO users (email, slug, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserBySlug :one
SELECT * FROM users WHERE slug = $1;

-- name: UpdateUserPlan :exec
UPDATE users SET plan = $2 WHERE id = $1;

-- name: UpdateUserTelegramChatID :exec
UPDATE users SET tg_chat_id = $2 WHERE id = $1;

-- name: UpdateUserLemonSqueezy :exec
UPDATE users
SET lemon_squeezy_customer_id = $2, lemon_squeezy_subscription_id = $3
WHERE id = $1;
```

- [ ] **Step 4: Create monitor queries**

Create `internal/sqlc/queries/monitors.sql`:

```sql
-- name: CreateMonitor :one
INSERT INTO monitors (user_id, name, url, method, interval_seconds, expected_status, keyword, alert_after_failures, is_paused, is_public)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetMonitorByID :one
SELECT * FROM monitors WHERE id = $1;

-- name: ListMonitorsByUserID :many
SELECT * FROM monitors WHERE user_id = $1 ORDER BY created_at;

-- name: ListPublicMonitorsByUserSlug :many
SELECT m.* FROM monitors m
JOIN users u ON m.user_id = u.id
WHERE u.slug = $1 AND m.is_public = TRUE
ORDER BY m.name;

-- name: ListActiveMonitors :many
SELECT * FROM monitors WHERE is_paused = FALSE;

-- name: CountMonitorsByUserID :one
SELECT COUNT(*)::int FROM monitors WHERE user_id = $1;

-- name: UpdateMonitor :exec
UPDATE monitors
SET name = $2, url = $3, method = $4, interval_seconds = $5, expected_status = $6,
    keyword = $7, alert_after_failures = $8, is_paused = $9, is_public = $10
WHERE id = $1 AND user_id = $11;

-- name: UpdateMonitorStatus :exec
UPDATE monitors SET current_status = $2 WHERE id = $1;

-- name: DeleteMonitor :exec
DELETE FROM monitors WHERE id = $1 AND user_id = $2;
```

- [ ] **Step 5: Create check result queries**

Create `internal/sqlc/queries/check_results.sql`:

```sql
-- name: InsertCheckResult :one
INSERT INTO check_results (monitor_id, status, status_code, response_time_ms, error_message, checked_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetLatestCheckResults :many
SELECT * FROM check_results
WHERE monitor_id = $1
ORDER BY checked_at DESC
LIMIT $2;

-- name: ConsecutiveFailures :one
WITH ordered AS (
    SELECT status, ROW_NUMBER() OVER (ORDER BY checked_at DESC) AS rn
    FROM check_results WHERE monitor_id = $1
),
first_up AS (
    SELECT MIN(rn) AS rn FROM ordered WHERE status = 'up'
)
SELECT COUNT(*)::int FROM ordered
WHERE status = 'down'
AND rn < COALESCE((SELECT rn FROM first_up), (SELECT COUNT(*) + 1 FROM ordered));

-- name: GetAggregatedCheckResults :many
SELECT
    date_trunc('hour', checked_at) + (EXTRACT(minute FROM checked_at)::INT / sqlc.arg(interval_minutes)::INT) * INTERVAL '1 minute' * sqlc.arg(interval_minutes)::INT AS bucket,
    AVG(response_time_ms)::FLOAT AS avg_response_ms,
    COUNT(*)::int AS check_count
FROM check_results
WHERE monitor_id = $1 AND checked_at >= $2
GROUP BY bucket
ORDER BY bucket;

-- name: GetUptimePercent :one
SELECT
    CASE WHEN COUNT(*) = 0 THEN 100.0
    ELSE (COUNT(*) FILTER (WHERE status = 'up'))::FLOAT / COUNT(*)::FLOAT * 100
    END AS uptime_percent
FROM check_results
WHERE monitor_id = $1 AND checked_at >= $2;

-- name: DeleteCheckResultsOlderThan :execrows
DELETE FROM check_results WHERE checked_at < $1;
```

- [ ] **Step 6: Create incident queries**

Create `internal/sqlc/queries/incidents.sql`:

```sql
-- name: CreateIncident :one
INSERT INTO incidents (monitor_id, cause)
VALUES ($1, $2)
RETURNING *;

-- name: ResolveIncident :exec
UPDATE incidents SET resolved_at = $2 WHERE id = $1;

-- name: GetOpenIncidentByMonitorID :one
SELECT * FROM incidents
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
SELECT * FROM incidents
WHERE monitor_id = $1
ORDER BY started_at DESC
LIMIT $2;
```

- [ ] **Step 7: Create session queries**

Create `internal/sqlc/queries/sessions.sql`:

```sql
-- name: CreateSession :one
INSERT INTO sessions (id, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = $1 AND expires_at > NOW();

-- name: TouchSession :exec
UPDATE sessions SET expires_at = $2 WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < NOW();
```

- [ ] **Step 8: Create generate.go**

Create `tools/generate.go`:

```go
package tools

//go:generate sqlc generate -f ../internal/sqlc/sqlc.yaml
//go:generate oapi-codegen -config ../internal/api/oapi-config.yaml ../api/openapi.yaml
```

- [ ] **Step 9: Generate sqlc code**

```bash
cd internal/sqlc && sqlc generate && cd ../..
```

Verify that `internal/sqlc/gen/` contains generated files.

- [ ] **Step 10: Commit**

```bash
git add internal/sqlc/ tools/
git commit -m "feat: sqlc setup with all queries for users, monitors, checks, incidents, sessions"
```

---

## Task 4: OpenAPI Spec & Code Generation

**Files:**
- Create: `api/openapi.yaml`
- Create: `internal/api/oapi-config.yaml`
- Generated: `internal/api/gen/server.go`, `internal/api/gen/types.go`

- [ ] **Step 1: Install oapi-codegen**

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```

- [ ] **Step 2: Create OpenAPI spec**

Create `api/openapi.yaml`:

```yaml
openapi: "3.0.3"
info:
  title: PingCast API
  version: "1.0.0"
  description: Uptime monitoring API

paths:
  /api/auth/register:
    post:
      operationId: register
      tags: [auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/RegisterRequest"
      responses:
        "200":
          description: Registered successfully
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/AuthResponse"
        "400":
          description: Validation error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ErrorResponse"

  /api/auth/login:
    post:
      operationId: login
      tags: [auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/LoginRequest"
      responses:
        "200":
          description: Logged in
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/AuthResponse"
        "401":
          description: Invalid credentials
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ErrorResponse"

  /api/auth/logout:
    post:
      operationId: logout
      tags: [auth]
      security:
        - sessionAuth: []
      responses:
        "200":
          description: Logged out

  /api/monitors:
    get:
      operationId: listMonitors
      tags: [monitors]
      security:
        - sessionAuth: []
      responses:
        "200":
          description: List of monitors
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/MonitorWithUptime"
    post:
      operationId: createMonitor
      tags: [monitors]
      security:
        - sessionAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateMonitorRequest"
      responses:
        "201":
          description: Monitor created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Monitor"
        "400":
          description: Validation error or limit reached
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ErrorResponse"

  /api/monitors/{id}:
    get:
      operationId: getMonitor
      tags: [monitors]
      security:
        - sessionAuth: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Monitor details
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/MonitorDetail"
        "404":
          description: Not found
    put:
      operationId: updateMonitor
      tags: [monitors]
      security:
        - sessionAuth: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UpdateMonitorRequest"
      responses:
        "200":
          description: Updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Monitor"
    delete:
      operationId: deleteMonitor
      tags: [monitors]
      security:
        - sessionAuth: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /api/monitors/{id}/pause:
    post:
      operationId: toggleMonitorPause
      tags: [monitors]
      security:
        - sessionAuth: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Toggled
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Monitor"

  /api/status/{slug}:
    get:
      operationId: getStatusPage
      tags: [status]
      parameters:
        - name: slug
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Public status page data
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/StatusPageResponse"
        "404":
          description: User not found

  /health:
    get:
      operationId: healthCheck
      tags: [system]
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/HealthResponse"

components:
  securitySchemes:
    sessionAuth:
      type: apiKey
      in: cookie
      name: session_id

  schemas:
    RegisterRequest:
      type: object
      required: [email, slug, password]
      properties:
        email:
          type: string
          format: email
        slug:
          type: string
          pattern: "^[a-z0-9-]{3,30}$"
        password:
          type: string
          minLength: 8

    LoginRequest:
      type: object
      required: [email, password]
      properties:
        email:
          type: string
          format: email
        password:
          type: string

    AuthResponse:
      type: object
      properties:
        user:
          $ref: "#/components/schemas/User"
        session_id:
          type: string

    User:
      type: object
      properties:
        id:
          type: string
          format: uuid
        email:
          type: string
        slug:
          type: string
        plan:
          type: string
          enum: [free, pro]
        tg_linked:
          type: boolean
        created_at:
          type: string
          format: date-time

    Monitor:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        url:
          type: string
        method:
          type: string
          enum: [GET, POST]
        interval_seconds:
          type: integer
        expected_status:
          type: integer
        keyword:
          type: string
          nullable: true
        alert_after_failures:
          type: integer
        is_paused:
          type: boolean
        is_public:
          type: boolean
        current_status:
          type: string
          enum: [up, down, unknown]
        created_at:
          type: string
          format: date-time

    MonitorWithUptime:
      allOf:
        - $ref: "#/components/schemas/Monitor"
        - type: object
          properties:
            uptime_24h:
              type: number
              format: float

    MonitorDetail:
      allOf:
        - $ref: "#/components/schemas/Monitor"
        - type: object
          properties:
            uptime_24h:
              type: number
            uptime_7d:
              type: number
            uptime_30d:
              type: number
            chart_data:
              type: array
              items:
                $ref: "#/components/schemas/ChartPoint"
            incidents:
              type: array
              items:
                $ref: "#/components/schemas/Incident"

    CreateMonitorRequest:
      type: object
      required: [name, url]
      properties:
        name:
          type: string
        url:
          type: string
        method:
          type: string
          enum: [GET, POST]
          default: GET
        interval_seconds:
          type: integer
          enum: [30, 60, 300]
          default: 300
        expected_status:
          type: integer
          default: 200
        keyword:
          type: string
          nullable: true
        alert_after_failures:
          type: integer
          default: 3
        is_public:
          type: boolean
          default: false

    UpdateMonitorRequest:
      type: object
      properties:
        name:
          type: string
        url:
          type: string
        method:
          type: string
          enum: [GET, POST]
        interval_seconds:
          type: integer
          enum: [30, 60, 300]
        expected_status:
          type: integer
        keyword:
          type: string
          nullable: true
        alert_after_failures:
          type: integer
        is_paused:
          type: boolean
        is_public:
          type: boolean

    ChartPoint:
      type: object
      properties:
        timestamp:
          type: string
          format: date-time
        avg_response_ms:
          type: number
        check_count:
          type: integer

    Incident:
      type: object
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

    StatusPageResponse:
      type: object
      properties:
        slug:
          type: string
        all_up:
          type: boolean
        show_branding:
          type: boolean
        monitors:
          type: array
          items:
            $ref: "#/components/schemas/StatusMonitor"
        incidents:
          type: array
          items:
            $ref: "#/components/schemas/Incident"

    StatusMonitor:
      type: object
      properties:
        name:
          type: string
        current_status:
          type: string
        uptime_90d:
          type: number

    HealthResponse:
      type: object
      properties:
        status:
          type: string

    ErrorResponse:
      type: object
      properties:
        error:
          type: string
```

- [ ] **Step 3: Create oapi-codegen config**

Create `internal/api/oapi-config.yaml`:

```yaml
package: gen
output: gen/server.go
generate:
  fiber-server: true
  strict-server: true
  models: true
  embedded-spec: true
```

- [ ] **Step 4: Generate code**

```bash
mkdir -p internal/api/gen
oapi-codegen -config internal/api/oapi-config.yaml api/openapi.yaml
```

- [ ] **Step 5: Add oapi-codegen runtime dependency**

```bash
go get github.com/oapi-codegen/runtime
go mod tidy
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add api/ internal/api/ tools/
git commit -m "feat: OpenAPI spec and oapi-codegen Fiber server generation"
```

---

## Task 5: Auth Service & Middleware (Fiber)

**Files:**
- Create: `internal/auth/service.go`
- Create: `internal/auth/service_test.go`
- Create: `internal/auth/middleware.go`
- Create: `internal/auth/ratelimit.go`

- [ ] **Step 1: Write failing tests**

Create `internal/auth/service_test.go`:

```go
package auth_test

import (
	"testing"

	"github.com/kirillinakin/pingcast/internal/auth"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		slug string
		ok   bool
	}{
		{"myslug", true},
		{"my-slug-123", true},
		{"ab", false},
		{"UPPERCASE", false},
		{"login", false},
		{"admin", false},
		{"a-valid-slug", true},
		{"this-slug-is-way-too-long-for-validation-rules", false},
	}

	for _, tt := range tests {
		err := auth.ValidateSlug(tt.slug)
		if tt.ok && err != nil {
			t.Errorf("ValidateSlug(%q) = %v, want nil", tt.slug, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidateSlug(%q) = nil, want error", tt.slug)
		}
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !auth.CheckPassword(hash, "mysecretpassword") {
		t.Error("CheckPassword returned false for correct password")
	}
	if auth.CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword returned true for wrong password")
	}
}
```

- [ ] **Step 2: Implement auth service**

Create `internal/auth/service.go`:

```go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 30 * 24 * time.Hour

var (
	slugRegex     = regexp.MustCompile(`^[a-z0-9-]{3,30}$`)
	reservedSlugs = map[string]bool{
		"login": true, "logout": true, "register": true, "api": true,
		"admin": true, "status": true, "health": true, "webhook": true,
		"pricing": true, "docs": true, "app": true, "dashboard": true,
		"settings": true, "billing": true,
	}
)

type Service struct {
	queries *gen.Queries
}

func NewService(queries *gen.Queries) *Service {
	return &Service{queries: queries}
}

func ValidateSlug(slug string) error {
	if !slugRegex.MatchString(slug) {
		return fmt.Errorf("slug must be 3-30 characters, lowercase alphanumeric and hyphens only")
	}
	if reservedSlugs[slug] {
		return fmt.Errorf("slug %q is reserved", slug)
	}
	return nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *Service) Register(ctx context.Context, email, slug, password string) (*gen.User, *gen.Session, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, nil, err
	}
	if len(password) < 8 {
		return nil, nil, fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.queries.CreateUser(ctx, gen.CreateUserParams{
		Email:        email,
		Slug:         slug,
		PasswordHash: hash,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create user: %w", err)
	}

	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}

	return &user, session, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*gen.User, *gen.Session, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid email or password")
	}

	if !CheckPassword(user.PasswordHash, password) {
		return nil, nil, fmt.Errorf("invalid email or password")
	}

	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}

	return &user, session, nil
}

func (s *Service) ValidateSession(ctx context.Context, sessionID string) (*gen.User, error) {
	session, err := s.queries.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}

	user, err := s.queries.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Rolling renewal
	_ = s.queries.TouchSession(ctx, gen.TouchSessionParams{
		ID:        sessionID,
		ExpiresAt: time.Now().Add(sessionDuration),
	})

	return &user, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.queries.DeleteSession(ctx, sessionID)
}

func (s *Service) createSession(ctx context.Context, userID uuid.UUID) (*gen.Session, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	session, err := s.queries.CreateSession(ctx, gen.CreateSessionParams{
		ID:        token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionDuration),
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &session, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

- [ ] **Step 3: Implement Fiber middleware**

Create `internal/auth/middleware.go`:

```go
package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

const UserContextKey = "user"

func (s *Service) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		user, err := s.ValidateSession(c.Context(), sessionID)
		if err != nil {
			c.ClearCookie("session_id")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid session"})
		}

		c.Locals(UserContextKey, user)
		return c.Next()
	}
}

// PageMiddleware redirects to login instead of returning 401 (for HTML pages)
func (s *Service) PageMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Redirect("/login")
		}

		user, err := s.ValidateSession(c.Context(), sessionID)
		if err != nil {
			c.ClearCookie("session_id")
			return c.Redirect("/login")
		}

		c.Locals(UserContextKey, user)
		return c.Next()
	}
}

func UserFromCtx(c *fiber.Ctx) *gen.User {
	user, _ := c.Locals(UserContextKey).(*gen.User)
	return user
}
```

- [ ] **Step 4: Implement rate limiter**

Create `internal/auth/ratelimit.go`:

```go
package auth

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu          sync.Mutex
	attempts    map[string][]time.Time
	maxAttempts int
	window      time.Duration
}

func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts:    make(map[string][]time.Time),
		maxAttempts: maxAttempts,
		window:      window,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var valid []time.Time
	for _, t := range rl.attempts[key] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.attempts[key] = valid

	return len(rl.attempts[key]) < rl.maxAttempts
}

// Record records a failed login attempt. Call only on failure.
func (rl *RateLimiter) Record(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts[key] = append(rl.attempts[key], time.Now())
}

func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, key)
}
```

- [ ] **Step 5: Run tests and commit**

```bash
go test ./internal/auth/ -v -count=1
git add internal/auth/ go.mod go.sum
git commit -m "feat: auth service with registration, login, Fiber middleware, rate limiting"
```

---

## Task 6: HTTP Checker Client

**Files:**
- Create: `internal/checker/client.go`
- Create: `internal/checker/client_test.go`
- Create: `internal/checker/hostlimit.go`

- [ ] **Step 1: Implement the HTTP checker client**

Create `internal/checker/client.go`:

```go
package checker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxBodyRead  = 1 << 20 // 1 MB
	maxRedirects = 5
	httpTimeout  = 10 * time.Second
	userAgent    = "PingCast/1.0"
)

type MonitorInfo struct {
	ID                 uuid.UUID
	URL                string
	Method             string
	IntervalSeconds    int
	ExpectedStatus     int
	Keyword            *string
	AlertAfterFailures int
}

type CheckResult struct {
	MonitorID      uuid.UUID
	Status         string // "up" or "down"
	StatusCode     *int32
	ResponseTimeMs int
	ErrorMessage   *string
	CheckedAt      time.Time
}

type Client struct {
	httpClient *http.Client
	hostLimit  *HostLimiter
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: httpTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return fmt.Errorf("stopped after %d redirects", maxRedirects)
				}
				return nil
			},
		},
		hostLimit: NewHostLimiter(3),
	}
}

func (c *Client) Check(ctx context.Context, m *MonitorInfo) *CheckResult {
	result := &CheckResult{
		MonitorID: m.ID,
		CheckedAt: time.Now(),
	}

	c.hostLimit.Acquire(m.URL)
	defer c.hostLimit.Release(m.URL)

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, m.Method, m.URL, nil)
	if err != nil {
		errMsg := fmt.Sprintf("create request: %v", err)
		result.Status = "down"
		result.ErrorMessage = &errMsg
		return result
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	result.ResponseTimeMs = int(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := fmt.Sprintf("request failed: %v", err)
		result.Status = "down"
		result.ErrorMessage = &errMsg
		return result
	}
	defer resp.Body.Close()

	statusCode := int32(resp.StatusCode)
	result.StatusCode = &statusCode

	if resp.StatusCode != m.ExpectedStatus {
		errMsg := fmt.Sprintf("expected status %d, got %d", m.ExpectedStatus, resp.StatusCode)
		result.Status = "down"
		result.ErrorMessage = &errMsg
		return result
	}

	// Keyword check
	if m.Keyword != nil && *m.Keyword != "" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyRead))
		if err != nil {
			errMsg := fmt.Sprintf("read body: %v", err)
			result.Status = "down"
			result.ErrorMessage = &errMsg
			return result
		}
		if !strings.Contains(string(body), *m.Keyword) {
			errMsg := fmt.Sprintf("keyword %q not found in response body", *m.Keyword)
			result.Status = "down"
			result.ErrorMessage = &errMsg
			return result
		}
	}

	result.Status = "up"
	return result
}
```

- [ ] **Step 2: Implement client tests**

Create `internal/checker/client_test.go`:

```go
package checker_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/checker"
)

func TestCheckUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))
	defer srv.Close()

	client := checker.NewClient()
	m := &checker.MonitorInfo{
		ID:             uuid.New(),
		URL:            srv.URL,
		Method:         "GET",
		ExpectedStatus: 200,
	}

	result := client.Check(context.Background(), m)
	if result.Status != "up" {
		t.Fatalf("expected up, got %s (error: %v)", result.Status, result.ErrorMessage)
	}
	if result.StatusCode == nil || *result.StatusCode != 200 {
		t.Fatalf("expected status code 200")
	}
}

func TestCheckDownWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := checker.NewClient()
	m := &checker.MonitorInfo{
		ID:             uuid.New(),
		URL:            srv.URL,
		Method:         "GET",
		ExpectedStatus: 200,
	}

	result := client.Check(context.Background(), m)
	if result.Status != "down" {
		t.Fatalf("expected down, got %s", result.Status)
	}
}

func TestCheckDownTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second)
	}))
	defer srv.Close()

	client := checker.NewClient()
	m := &checker.MonitorInfo{
		ID:             uuid.New(),
		URL:            srv.URL,
		Method:         "GET",
		ExpectedStatus: 200,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := client.Check(ctx, m)
	if result.Status != "down" {
		t.Fatalf("expected down on timeout, got %s", result.Status)
	}
}

func TestKeywordCheckFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
	}))
	defer srv.Close()

	client := checker.NewClient()
	keyword := "Hello"
	m := &checker.MonitorInfo{
		ID:             uuid.New(),
		URL:            srv.URL,
		Method:         "GET",
		ExpectedStatus: 200,
		Keyword:        &keyword,
	}

	result := client.Check(context.Background(), m)
	if result.Status != "up" {
		t.Fatalf("expected up with keyword found, got %s (error: %v)", result.Status, result.ErrorMessage)
	}
}

func TestKeywordCheckMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
	}))
	defer srv.Close()

	client := checker.NewClient()
	keyword := "Goodbye"
	m := &checker.MonitorInfo{
		ID:             uuid.New(),
		URL:            srv.URL,
		Method:         "GET",
		ExpectedStatus: 200,
		Keyword:        &keyword,
	}

	result := client.Check(context.Background(), m)
	if result.Status != "down" {
		t.Fatalf("expected down with keyword missing, got %s", result.Status)
	}
}
```

- [ ] **Step 3: Implement per-host semaphore**

Create `internal/checker/hostlimit.go`:

```go
package checker

import (
	"net/url"
	"sync"
)

// HostLimiter limits concurrent checks per host.
type HostLimiter struct {
	mu       sync.Mutex
	sems     map[string]chan struct{}
	perHost  int
}

func NewHostLimiter(perHost int) *HostLimiter {
	return &HostLimiter{
		sems:    make(map[string]chan struct{}),
		perHost: perHost,
	}
}

func (h *HostLimiter) hostKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}

func (h *HostLimiter) getSem(rawURL string) chan struct{} {
	host := h.hostKey(rawURL)
	h.mu.Lock()
	defer h.mu.Unlock()
	sem, ok := h.sems[host]
	if !ok {
		sem = make(chan struct{}, h.perHost)
		h.sems[host] = sem
	}
	return sem
}

func (h *HostLimiter) Acquire(rawURL string) {
	h.getSem(rawURL) <- struct{}{}
}

func (h *HostLimiter) Release(rawURL string) {
	<-h.getSem(rawURL)
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/checker/
git commit -m "feat: HTTP checker client with timeout, redirects, keyword check, host limiter"
```

---

## Task 7: Scheduler & Worker Pool

**Files:**
- Create: `internal/checker/scheduler.go`
- Create: `internal/checker/scheduler_test.go`
- Create: `internal/checker/worker.go`

- [ ] **Step 1: Implement the scheduler**

Create `internal/checker/scheduler.go`:

```go
package checker

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type schedulerEntry struct {
	monitor *MonitorInfo
	ticker  *time.Ticker
	stopCh  chan struct{}
}

// Scheduler manages timer-per-monitor tickers that fire check jobs.
type Scheduler struct {
	mu      sync.Mutex
	entries map[uuid.UUID]*schedulerEntry
	onFire  func(m *MonitorInfo)
}

func NewScheduler(onFire func(m *MonitorInfo)) *Scheduler {
	return &Scheduler{
		entries: make(map[uuid.UUID]*schedulerEntry),
		onFire:  onFire,
	}
}

func (s *Scheduler) Add(m *MonitorInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing entry for this monitor if present
	if existing, ok := s.entries[m.ID]; ok {
		existing.ticker.Stop()
		close(existing.stopCh)
		delete(s.entries, m.ID)
	}

	interval := time.Duration(m.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	stopCh := make(chan struct{})

	entry := &schedulerEntry{
		monitor: m,
		ticker:  ticker,
		stopCh:  stopCh,
	}
	s.entries[m.ID] = entry

	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				s.onFire(m)
			}
		}
	}()
}

func (s *Scheduler) Remove(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.entries[id]; ok {
		entry.ticker.Stop()
		close(entry.stopCh)
		delete(s.entries, id)
	}
}

func (s *Scheduler) Update(m *MonitorInfo) {
	s.Add(m) // Add handles removal of existing entry
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entry := range s.entries {
		entry.ticker.Stop()
		close(entry.stopCh)
		delete(s.entries, id)
	}
}

func (s *Scheduler) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
```

- [ ] **Step 2: Implement scheduler tests**

Create `internal/checker/scheduler_test.go`:

```go
package checker_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/checker"
)

func TestSchedulerAddAndFire(t *testing.T) {
	var fired atomic.Int32

	sched := checker.NewScheduler(func(m *checker.MonitorInfo) {
		fired.Add(1)
	})
	defer sched.Stop()

	m := &checker.MonitorInfo{
		ID:              uuid.New(),
		URL:             "http://example.com",
		Method:          "GET",
		IntervalSeconds: 1,
		ExpectedStatus:  200,
	}

	sched.Add(m)

	if sched.Count() != 1 {
		t.Fatalf("expected count 1, got %d", sched.Count())
	}

	time.Sleep(2500 * time.Millisecond)
	if fired.Load() < 1 {
		t.Fatal("expected at least 1 fire, got 0")
	}
}

func TestSchedulerRemove(t *testing.T) {
	var fired atomic.Int32

	sched := checker.NewScheduler(func(m *checker.MonitorInfo) {
		fired.Add(1)
	})
	defer sched.Stop()

	id := uuid.New()
	m := &checker.MonitorInfo{
		ID:              id,
		URL:             "http://example.com",
		Method:          "GET",
		IntervalSeconds: 1,
		ExpectedStatus:  200,
	}

	sched.Add(m)
	sched.Remove(id)

	if sched.Count() != 0 {
		t.Fatalf("expected count 0 after remove, got %d", sched.Count())
	}

	time.Sleep(1500 * time.Millisecond)
	if fired.Load() != 0 {
		t.Fatalf("expected 0 fires after remove, got %d", fired.Load())
	}
}
```

- [ ] **Step 3: Implement the worker pool**

Create `internal/checker/worker.go`:

```go
package checker

import (
	"context"
	"sync"
)

type CheckHandler func(ctx context.Context, monitor *MonitorInfo, result *CheckResult)

// WorkerPool is a bounded goroutine pool for running checks.
type WorkerPool struct {
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	jobs    chan *MonitorInfo
	client  *Client
	handler CheckHandler
}

func NewWorkerPool(ctx context.Context, size int, client *Client, handler CheckHandler) *WorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)
	wp := &WorkerPool{
		ctx:     poolCtx,
		cancel:  cancel,
		jobs:    make(chan *MonitorInfo, size*2),
		client:  client,
		handler: handler,
	}

	for i := 0; i < size; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return wp
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case m, ok := <-wp.jobs:
			if !ok {
				return
			}
			result := wp.client.Check(wp.ctx, m)
			wp.handler(wp.ctx, m, result)
		}
	}
}

func (wp *WorkerPool) Submit(m *MonitorInfo) {
	select {
	case <-wp.ctx.Done():
		return
	case wp.jobs <- m:
	}
}

func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/checker/scheduler.go internal/checker/scheduler_test.go internal/checker/worker.go
git commit -m "feat: scheduler with timer-per-monitor and bounded worker pool"
```

---

## Task 8: Notifier (PG LISTEN/NOTIFY + Telegram + Email)

**Files:**
- Create: `internal/notifier/listener.go`
- Create: `internal/notifier/telegram.go`
- Create: `internal/notifier/telegram_test.go`
- Create: `internal/notifier/email.go`

- [ ] **Step 1: Implement Telegram sender**

Create `internal/notifier/telegram.go`:

```go
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TelegramSender struct {
	token   string
	baseURL string
	client  *http.Client
}

func NewTelegramSender(token string) *TelegramSender {
	return &TelegramSender{
		token:   token,
		baseURL: "https://api.telegram.org",
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// NewTelegramSenderWithBase creates a sender with a custom base URL (for testing).
func NewTelegramSenderWithBase(token, baseURL string) *TelegramSender {
	return &TelegramSender{
		token:   token,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TelegramSender) sendMessage(chatID int64, text string) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.token)
	body, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})

	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram returned status %d", resp.StatusCode)
	}
	return nil
}

func (t *TelegramSender) SendDown(chatID int64, monitorName, monitorURL, details string) error {
	text := fmt.Sprintf(
		"🔴 <b>%s is DOWN</b>\n\nURL: %s\nReason: %s",
		monitorName, monitorURL, details,
	)
	return t.sendMessage(chatID, text)
}

func (t *TelegramSender) SendUp(chatID int64, monitorName, monitorURL string) error {
	text := fmt.Sprintf(
		"🟢 <b>%s is UP</b>\n\nURL: %s\nRecovered.",
		monitorName, monitorURL,
	)
	return t.sendMessage(chatID, text)
}
```

Create `internal/notifier/telegram_test.go`:

```go
package notifier_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/internal/notifier"
)

func TestTelegramSendDown(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	sender := notifier.NewTelegramSenderWithBase("test-token", srv.URL)

	err := sender.SendDown(12345, "My Monitor", "https://example.com", "timeout")
	if err != nil {
		t.Fatalf("SendDown failed: %v", err)
	}

	chatID, ok := received["chat_id"].(float64)
	if !ok || int64(chatID) != 12345 {
		t.Fatalf("unexpected chat_id: %v", received["chat_id"])
	}

	text, ok := received["text"].(string)
	if !ok || text == "" {
		t.Fatal("expected non-empty text")
	}
}

func TestTelegramSendUp(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	sender := notifier.NewTelegramSenderWithBase("test-token", srv.URL)

	err := sender.SendUp(12345, "My Monitor", "https://example.com")
	if err != nil {
		t.Fatalf("SendUp failed: %v", err)
	}

	chatID, ok := received["chat_id"].(float64)
	if !ok || int64(chatID) != 12345 {
		t.Fatalf("unexpected chat_id: %v", received["chat_id"])
	}
}
```

Create `internal/notifier/email.go`:

```go
package notifier

import (
	"fmt"
	"log/slog"
	"net/smtp"
)

type EmailSender struct {
	host string
	port int
	user string
	pass string
	from string
}

func NewEmailSender(host string, port int, user, pass, from string) *EmailSender {
	return &EmailSender{
		host: host,
		port: port,
		user: user,
		pass: pass,
		from: from,
	}
}

func (e *EmailSender) send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	auth := smtp.PlainAuth("", e.user, e.pass, e.host)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		e.from, to, subject, body)

	return smtp.SendMail(addr, auth, e.from, []string{to}, []byte(msg))
}

func (e *EmailSender) SendDown(to, monitorName, monitorURL, details string) {
	subject := fmt.Sprintf("[PingCast] %s is DOWN", monitorName)
	body := fmt.Sprintf("%s is DOWN\n\nURL: %s\nReason: %s", monitorName, monitorURL, details)
	if err := e.send(to, subject, body); err != nil {
		slog.Error("email send failed", "to", to, "error", err)
	}
}

func (e *EmailSender) SendUp(to, monitorName, monitorURL string) {
	subject := fmt.Sprintf("[PingCast] %s is UP", monitorName)
	body := fmt.Sprintf("%s is back UP\n\nURL: %s", monitorName, monitorURL)
	if err := e.send(to, subject, body); err != nil {
		slog.Error("email send failed", "to", to, "error", err)
	}
}
```

- [ ] **Step 2: Implement PG LISTEN/NOTIFY listener using sqlc queries**

Create `internal/notifier/listener.go`:

```go
package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type MonitorEvent struct {
	MonitorID string `json:"monitor_id"`
	Event     string `json:"event"`
	Details   string `json:"details"`
}

type Listener struct {
	pool     *pgxpool.Pool
	queries  *gen.Queries
	telegram *TelegramSender
	email    *EmailSender
}

func NewListener(pool *pgxpool.Pool, queries *gen.Queries, tg *TelegramSender, email *EmailSender) *Listener {
	return &Listener{pool: pool, queries: queries, telegram: tg, email: email}
}

func (l *Listener) Start(ctx context.Context) {
	go l.listen(ctx)
}

func (l *Listener) listen(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := l.listenOnce(ctx); err != nil {
			slog.Error("listener error, reconnecting", "error", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (l *Listener) listenOnce(ctx context.Context) error {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN monitor_events")
	if err != nil {
		return fmt.Errorf("LISTEN: %w", err)
	}

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait for notification: %w", err)
		}

		var event MonitorEvent
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			slog.Error("invalid event payload", "payload", notification.Payload, "error", err)
			continue
		}

		l.handleEvent(ctx, &event)
	}
}

func (l *Listener) handleEvent(ctx context.Context, event *MonitorEvent) {
	monitorID, _ := uuid.Parse(event.MonitorID)
	monitor, err := l.queries.GetMonitorByID(ctx, monitorID)
	if err != nil {
		slog.Error("monitor not found", "monitor_id", event.MonitorID, "error", err)
		return
	}

	user, err := l.queries.GetUserByID(ctx, monitor.UserID)
	if err != nil {
		slog.Error("user not found", "user_id", monitor.UserID, "error", err)
		return
	}

	// Telegram
	if user.TgChatID != nil && l.telegram != nil {
		switch event.Event {
		case "down":
			if err := l.telegram.SendDown(*user.TgChatID, monitor.Name, monitor.Url, event.Details); err != nil {
				slog.Error("telegram send failed", "error", err)
			}
		case "up":
			if err := l.telegram.SendUp(*user.TgChatID, monitor.Name, monitor.Url); err != nil {
				slog.Error("telegram send failed", "error", err)
			}
		}
	}

	// Email (Pro only)
	if user.Plan == "pro" && l.email != nil {
		switch event.Event {
		case "down":
			l.email.SendDown(user.Email, monitor.Name, monitor.Url, event.Details)
		case "up":
			l.email.SendUp(user.Email, monitor.Name, monitor.Url)
		}
	}

	slog.Info("alert sent", "monitor_id", event.MonitorID, "event", event.Event)
}
```

- [ ] **Step 3: Run tests and commit**

```bash
go test ./internal/notifier/ -v -count=1
git add internal/notifier/
git commit -m "feat: notifier with PG LISTEN/NOTIFY, Telegram, and email"
```

---

## Task 9: API Server Implementation (oapi-codegen)

**Files:**
- Create: `internal/handler/server.go`

This implements the `StrictServerInterface` generated by oapi-codegen.

- [ ] **Step 1: Implement the server interface**

Create `internal/handler/server.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kirillinakin/pingcast/internal/api/gen"
	"github.com/kirillinakin/pingcast/internal/auth"
	"github.com/kirillinakin/pingcast/internal/checker"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type Server struct {
	queries     *sqlcgen.Queries
	pool        *pgxpool.Pool
	authService *auth.Service
	rateLimiter *auth.RateLimiter
	onChanged   func(monitorID uuid.UUID, deleted bool)
}

func NewServer(
	queries *sqlcgen.Queries,
	pool *pgxpool.Pool,
	authService *auth.Service,
	rateLimiter *auth.RateLimiter,
	onChanged func(monitorID uuid.UUID, deleted bool),
) *Server {
	return &Server{
		queries:     queries,
		pool:        pool,
		authService: authService,
		rateLimiter: rateLimiter,
		onChanged:   onChanged,
	}
}

// The implementation methods match the generated StrictServerInterface.
// Each method receives the parsed request and returns a typed response.
// The exact method signatures depend on oapi-codegen output.
// Below is the implementation logic — adapt method signatures to match generated interface.

// Register handles POST /api/auth/register
func (s *Server) Register(ctx context.Context, req gen.RegisterRequest) (*gen.AuthResponse, error) {
	user, session, err := s.authService.Register(ctx, req.Email, req.Slug, req.Password)
	if err != nil {
		return nil, err
	}

	tgLinked := user.TgChatID != nil
	return &gen.AuthResponse{
		User: &gen.User{
			Id:        uuidPtr(user.ID),
			Email:     &user.Email,
			Slug:      &user.Slug,
			Plan:      (*gen.UserPlan)(&user.Plan),
			TgLinked:  &tgLinked,
			CreatedAt: &user.CreatedAt,
		},
		SessionId: &session.ID,
	}, nil
}

// Login handles POST /api/auth/login
func (s *Server) Login(ctx context.Context, req gen.LoginRequest) (*gen.AuthResponse, error) {
	if !s.rateLimiter.Allow(req.Email) {
		return nil, fmt.Errorf("too many login attempts")
	}

	user, session, err := s.authService.Login(ctx, req.Email, req.Password)
	if err != nil {
		s.rateLimiter.Record(req.Email)
		return nil, err
	}

	s.rateLimiter.Reset(req.Email)

	tgLinked := user.TgChatID != nil
	return &gen.AuthResponse{
		User: &gen.User{
			Id:        uuidPtr(user.ID),
			Email:     &user.Email,
			Slug:      &user.Slug,
			Plan:      (*gen.UserPlan)(&user.Plan),
			TgLinked:  &tgLinked,
			CreatedAt: &user.CreatedAt,
		},
		SessionId: &session.ID,
	}, nil
}

// ListMonitors handles GET /api/monitors (authenticated)
func (s *Server) ListMonitors(ctx context.Context, user *sqlcgen.User) ([]gen.MonitorWithUptime, error) {
	monitors, err := s.queries.ListMonitorsByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	result := make([]gen.MonitorWithUptime, 0, len(monitors))
	for _, m := range monitors {
		uptime, _ := s.queries.GetUptimePercent(ctx, sqlcgen.GetUptimePercentParams{
			MonitorID: m.ID,
			CheckedAt: time.Now().Add(-24 * time.Hour),
		})
		result = append(result, toMonitorWithUptime(m, uptime))
	}

	return result, nil
}

// CreateMonitor handles POST /api/monitors (authenticated)
func (s *Server) CreateMonitor(ctx context.Context, user *sqlcgen.User, req gen.CreateMonitorRequest) (*sqlcgen.Monitor, error) {
	count, _ := s.queries.CountMonitorsByUserID(ctx, user.ID)
	limit := 5
	if user.Plan == "pro" {
		limit = 50
	}
	if int(count) >= limit {
		return nil, fmt.Errorf("monitor limit reached")
	}

	minInterval := 300
	if user.Plan == "pro" {
		minInterval = 30
	}

	interval := 300
	if req.IntervalSeconds != nil {
		interval = int(*req.IntervalSeconds)
	}
	if interval < minInterval {
		interval = minInterval
	}

	method := "GET"
	if req.Method != nil {
		method = string(*req.Method)
	}

	expectedStatus := 200
	if req.ExpectedStatus != nil {
		expectedStatus = int(*req.ExpectedStatus)
	}

	alertAfter := 3
	if req.AlertAfterFailures != nil {
		alertAfter = int(*req.AlertAfterFailures)
	}

	isPublic := false
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	mon, err := s.queries.CreateMonitor(ctx, sqlcgen.CreateMonitorParams{
		UserID:             user.ID,
		Name:               req.Name,
		Url:                req.Url,
		Method:             method,
		IntervalSeconds:    int32(interval),
		ExpectedStatus:     int32(expectedStatus),
		Keyword:            req.Keyword,
		AlertAfterFailures: int32(alertAfter),
		IsPaused:           false,
		IsPublic:           isPublic,
	})
	if err != nil {
		return nil, err
	}

	if s.onChanged != nil {
		s.onChanged(mon.ID, false)
	}

	return &mon, nil
}

// GetStatusPage handles GET /api/status/{slug}
func (s *Server) GetStatusPage(ctx context.Context, slug string) (*gen.StatusPageResponse, error) {
	user, err := s.queries.GetUserBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}

	monitors, _ := s.queries.ListPublicMonitorsByUserSlug(ctx, slug)

	allUp := true
	statusMonitors := make([]gen.StatusMonitor, 0, len(monitors))
	var incidents []gen.Incident

	for _, m := range monitors {
		uptime, _ := s.queries.GetUptimePercent(ctx, sqlcgen.GetUptimePercentParams{
			MonitorID: m.ID,
			CheckedAt: time.Now().Add(-90 * 24 * time.Hour),
		})
		if m.CurrentStatus != "up" {
			allUp = false
		}
		name := m.Name
		status := m.CurrentStatus
		statusMonitors = append(statusMonitors, gen.StatusMonitor{
			Name:          &name,
			CurrentStatus: &status,
			Uptime90d:     &uptime,
		})

		// Collect incidents
		dbIncidents, _ := s.queries.ListIncidentsByMonitorID(ctx, sqlcgen.ListIncidentsByMonitorIDParams{
			MonitorID: m.ID,
			Limit:     5,
		})
		for _, inc := range dbIncidents {
			incidents = append(incidents, toIncident(inc))
		}
	}

	showBranding := user.Plan == "free"
	return &gen.StatusPageResponse{
		Slug:         &slug,
		AllUp:        &allUp,
		ShowBranding: &showBranding,
		Monitors:     &statusMonitors,
		Incidents:    &incidents,
	}, nil
}

// Helper conversion functions
func uuidPtr(id uuid.UUID) *uuid.UUID { return &id }

func toMonitorWithUptime(m sqlcgen.Monitor, uptime float64) gen.MonitorWithUptime {
	// Map sqlc model to API model
	// Exact field mapping depends on generated types
	return gen.MonitorWithUptime{
		Uptime24h: &uptime,
		// ... map remaining fields from m
	}
}

func toIncident(i sqlcgen.Incident) gen.Incident {
	id := int64(i.ID)
	return gen.Incident{
		Id:         &id,
		MonitorId:  uuidPtr(i.MonitorID),
		StartedAt:  &i.StartedAt,
		ResolvedAt: i.ResolvedAt,
		Cause:      &i.Cause,
	}
}
```

Note: The exact method signatures will need to match what oapi-codegen generates. The logic above is the implementation — adapt the function signatures, parameter types, and return types to match the generated `StrictServerInterface`.

- [ ] **Step 2: Commit**

```bash
git add internal/handler/server.go
git commit -m "feat: API server implementing oapi-codegen interface"
```

---

## Task 10: HTML Page Handlers & Templates

**Files:**
- Create: `internal/handler/pages.go`
- Create: `internal/web/embed.go`
- Create: all templates in `internal/web/templates/`
- Create: `internal/web/static/css/style.css`
- Create: `internal/web/static/js/chart-init.js`

- [ ] **Step 1: Create web embed**

Create `internal/web/embed.go`:

```go
package web

import "embed"

//go:embed templates static
var FS embed.FS
```

- [ ] **Step 2: Create all templates**

Same templates as previous plan (layout.html, landing.html, login.html, register.html, dashboard.html, monitor_detail.html, monitor_form.html, statuspage.html, style.css, chart-init.js).

The key difference: HTMX forms now POST to `/api/auth/login` (JSON API) instead of form-encoded. HTMX can handle JSON responses with `hx-ext="json-enc"` or the forms can still use traditional form POST to page handlers that call the auth service internally.

**Recommended approach:** Keep page handlers (login form, register form) as traditional form-POST handlers in `pages.go`. They call the auth service directly and redirect. The JSON API endpoints (`/api/...`) are for programmatic access and the HTMX dashboard.

- [ ] **Step 3: Implement page handlers**

Create `internal/handler/pages.go`:

```go
package handler

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/auth"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
	"github.com/kirillinakin/pingcast/internal/web"
)

type PageHandler struct {
	queries     *sqlcgen.Queries
	authService *auth.Service
	rateLimiter *auth.RateLimiter
	templates   *template.Template
}

func NewPageHandler(queries *sqlcgen.Queries, authService *auth.Service, rateLimiter *auth.RateLimiter) *PageHandler {
	tmplFS, _ := fs.Sub(web.FS, "templates")
	templates := template.Must(template.ParseFS(tmplFS,
		"layout.html", "landing.html", "login.html", "register.html",
		"dashboard.html", "monitor_detail.html", "monitor_form.html", "statuspage.html",
	))

	return &PageHandler{
		queries:     queries,
		authService: authService,
		rateLimiter: rateLimiter,
		templates:   templates,
	}
}

func (h *PageHandler) Landing(c *fiber.Ctx) error {
	return h.render(c, "landing.html", fiber.Map{"User": auth.UserFromCtx(c)})
}

func (h *PageHandler) LoginPage(c *fiber.Ctx) error {
	return h.render(c, "login.html", nil)
}

func (h *PageHandler) LoginSubmit(c *fiber.Ctx) error {
	email := c.FormValue("email")
	password := c.FormValue("password")

	if !h.rateLimiter.Allow(email) {
		return h.render(c, "login.html", fiber.Map{"Error": "Too many login attempts. Try again later."})
	}

	_, session, err := h.authService.Login(c.Context(), email, password)
	if err != nil {
		h.rateLimiter.Record(email)
		return h.render(c, "login.html", fiber.Map{"Error": "Invalid email or password."})
	}

	h.rateLimiter.Reset(email)
	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})

	return c.Redirect("/dashboard")
}

func (h *PageHandler) RegisterPage(c *fiber.Ctx) error {
	return h.render(c, "register.html", nil)
}

func (h *PageHandler) RegisterSubmit(c *fiber.Ctx) error {
	email := c.FormValue("email")
	slug := c.FormValue("slug")
	password := c.FormValue("password")

	_, session, err := h.authService.Register(c.Context(), email, slug, password)
	if err != nil {
		return h.render(c, "register.html", fiber.Map{"Error": err.Error()})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})

	return c.Redirect("/dashboard")
}

func (h *PageHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		h.authService.Logout(c.Context(), sessionID)
	}
	c.ClearCookie("session_id")
	return c.Redirect("/")
}

func (h *PageHandler) Dashboard(c *fiber.Ctx) error {
	user := auth.UserFromCtx(c)
	monitors, _ := h.queries.ListMonitorsByUserID(c.Context(), user.ID)

	type MonitorRow struct {
		Monitor sqlcgen.Monitor
		Uptime  float64
	}

	var rows []MonitorRow
	for _, m := range monitors {
		uptime, _ := h.queries.GetUptimePercent(c.Context(), sqlcgen.GetUptimePercentParams{
			MonitorID: m.ID,
			CheckedAt: time.Now().Add(-24 * time.Hour),
		})
		rows = append(rows, MonitorRow{Monitor: m, Uptime: uptime})
	}

	return h.render(c, "dashboard.html", fiber.Map{
		"User":     user,
		"Monitors": rows,
	})
}

func (h *PageHandler) MonitorDetail(c *fiber.Ctx) error {
	user := auth.UserFromCtx(c)
	monID, _ := uuid.Parse(c.Params("id"))

	mon, err := h.queries.GetMonitorByID(c.Context(), monID)
	if err != nil || mon.UserID != user.ID {
		return c.Redirect("/dashboard")
	}

	now := time.Now()
	uptime24h, _ := h.queries.GetUptimePercent(c.Context(), sqlcgen.GetUptimePercentParams{MonitorID: monID, CheckedAt: now.Add(-24 * time.Hour)})
	uptime7d, _ := h.queries.GetUptimePercent(c.Context(), sqlcgen.GetUptimePercentParams{MonitorID: monID, CheckedAt: now.Add(-7 * 24 * time.Hour)})
	uptime30d, _ := h.queries.GetUptimePercent(c.Context(), sqlcgen.GetUptimePercentParams{MonitorID: monID, CheckedAt: now.Add(-30 * 24 * time.Hour)})

	chartData, _ := h.queries.GetAggregatedCheckResults(c.Context(), sqlcgen.GetAggregatedCheckResultsParams{
		MonitorID:       monID,
		CheckedAt:       now.Add(-24 * time.Hour),
		IntervalMinutes: 5,
	})
	chartJSON, _ := json.Marshal(chartData)

	incidents, _ := h.queries.ListIncidentsByMonitorID(c.Context(), sqlcgen.ListIncidentsByMonitorIDParams{MonitorID: monID, Limit: 10})

	return h.render(c, "monitor_detail.html", fiber.Map{
		"User":      user,
		"Monitor":   mon,
		"Uptime24h": uptime24h,
		"Uptime7d":  uptime7d,
		"Uptime30d": uptime30d,
		"ChartData": template.JS(chartJSON),
		"Incidents": incidents,
	})
}

func (h *PageHandler) StatusPage(c *fiber.Ctx) error {
	slug := c.Params("slug")
	user, err := h.queries.GetUserBySlug(c.Context(), slug)
	if err != nil {
		return c.Status(404).SendString("Not found")
	}
	monitors, _ := h.queries.ListPublicMonitorsByUserSlug(c.Context(), slug)
	allUp := true
	type StatusMon struct {
		Name          string
		CurrentStatus string
		Uptime90d     float64
	}
	var statusMons []StatusMon
	for _, m := range monitors {
		uptime, _ := h.queries.GetUptimePercent(c.Context(), sqlcgen.GetUptimePercentParams{
			MonitorID: m.ID,
			CheckedAt: time.Now().Add(-90 * 24 * time.Hour),
		})
		if m.CurrentStatus != "up" {
			allUp = false
		}
		statusMons = append(statusMons, StatusMon{Name: m.Name, CurrentStatus: m.CurrentStatus, Uptime90d: uptime})
	}
	showBranding := user.Plan == "free"
	return h.render(c, "statuspage.html", fiber.Map{
		"Slug": slug, "AllUp": allUp, "Monitors": statusMons, "ShowBranding": showBranding,
	})
}

func (h *PageHandler) render(c *fiber.Ctx, name string, data fiber.Map) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return h.templates.ExecuteTemplate(c, name, data)
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/handler/pages.go internal/web/
git commit -m "feat: Fiber page handlers with HTMX templates"
```

---

## Task 11: Webhook Handlers (Lemon Squeezy + Telegram Bot)

**Files:**
- Create: `internal/handler/webhook.go`

- [ ] **Step 1: Implement webhook handlers**

Create `internal/handler/webhook.go`:

```go
package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type WebhookHandler struct {
	queries             *sqlcgen.Queries
	lemonSqueezySecret string
}

func NewWebhookHandler(queries *sqlcgen.Queries, lemonSqueezySecret string) *WebhookHandler {
	return &WebhookHandler{queries: queries, lemonSqueezySecret: lemonSqueezySecret}
}

type lemonSqueezyWebhook struct {
	Meta struct {
		EventName string `json:"event_name"`
	} `json:"meta"`
	Data struct {
		Attributes struct {
			CustomerID int    `json:"customer_id"`
			Status     string `json:"status"`
			UserEmail  string `json:"user_email"`
		} `json:"attributes"`
		ID string `json:"id"`
	} `json:"data"`
}

func (h *WebhookHandler) HandleLemonSqueezy(c *fiber.Ctx) error {
	body := c.Body()
	sig := c.Get("X-Signature")

	if !h.verifySignature(body, sig) {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	var webhook lemonSqueezyWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	email := webhook.Data.Attributes.UserEmail
	user, err := h.queries.GetUserByEmail(c.Context(), email)
	if err != nil {
		slog.Error("webhook: user not found", "email", email)
		return c.SendStatus(fiber.StatusOK)
	}

	switch webhook.Meta.EventName {
	case "subscription_created", "subscription_updated":
		if webhook.Data.Attributes.Status == "active" {
			h.queries.UpdateUserPlan(c.Context(), sqlcgen.UpdateUserPlanParams{ID: user.ID, Plan: "pro"})
			slog.Info("user upgraded to pro", "user_id", user.ID)
		}
	case "subscription_cancelled":
		h.queries.UpdateUserPlan(c.Context(), sqlcgen.UpdateUserPlanParams{ID: user.ID, Plan: "free"})
		slog.Info("user downgraded to free", "user_id", user.ID)
	case "subscription_payment_failed":
		slog.Warn("payment failed", "user_id", user.ID)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	if h.lemonSqueezySecret == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(h.lemonSqueezySecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Telegram Bot /start handler
func (h *WebhookHandler) HandleTelegramWebhook(c *fiber.Ctx) error {
	var update struct {
		Message struct {
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
			Text string `json:"text"`
		} `json:"message"`
	}

	if err := c.BodyParser(&update); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	text := update.Message.Text
	chatID := update.Message.Chat.ID

	if strings.HasPrefix(text, "/start ") {
		userIDStr := strings.TrimPrefix(text, "/start ")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return c.SendStatus(fiber.StatusOK)
		}

		if err := h.queries.UpdateUserTelegramChatID(c.Context(), sqlcgen.UpdateUserTelegramChatIDParams{
			ID:       userID,
			TgChatID: &chatID,
		}); err != nil {
			slog.Error("failed to link telegram", "user_id", userID, "error", err)
		} else {
			slog.Info("telegram linked", "user_id", userID, "chat_id", chatID)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/handler/webhook.go
git commit -m "feat: Lemon Squeezy and Telegram bot webhook handlers"
```

---

## Task 12: Fiber App Setup & Route Wiring

**Files:**
- Create: `internal/handler/setup.go`

- [ ] **Step 1: Create Fiber app setup**

Create `internal/handler/setup.go`:

```go
package handler

import (
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/kirillinakin/pingcast/internal/auth"
	"github.com/kirillinakin/pingcast/internal/web"
)

func SetupApp(
	authService *auth.Service,
	pageHandler *PageHandler,
	server *Server,
	webhookHandler *WebhookHandler,
) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(logger.New())
	app.Use(recover.New())

	// Static files
	staticFS, _ := fs.Sub(web.FS, "static")
	app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(staticFS),
	}))

	// Health
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Public pages
	app.Get("/", pageHandler.Landing)
	app.Get("/login", pageHandler.LoginPage)
	app.Post("/login", pageHandler.LoginSubmit)
	app.Get("/register", pageHandler.RegisterPage)
	app.Post("/register", pageHandler.RegisterSubmit)
	app.Post("/logout", pageHandler.Logout)

	// Public status page (HTML)
	app.Get("/status/:slug", pageHandler.StatusPage)

	// Webhooks
	app.Post("/webhook/lemonsqueezy", webhookHandler.HandleLemonSqueezy)
	app.Post("/webhook/telegram/:token", webhookHandler.HandleTelegramWebhook)

	// Public API (no auth)
	app.Post("/api/auth/register", wrapRegister(server))
	app.Post("/api/auth/login", wrapLogin(server))

	// Authenticated API
	api := app.Group("/api", authService.Middleware())
	// oapi-codegen's RegisterHandlers wires all routes from the spec.
	// If using strict mode, use the generated middleware wrapper.
	// The exact function depends on generated output.
	// Alternatively, wire manually:
	api.Get("/monitors", wrapHandler(server.ListMonitors))
	api.Post("/monitors", wrapHandler(server.CreateMonitor))
	api.Get("/monitors/:id", wrapHandler(server.GetMonitor))
	api.Put("/monitors/:id", wrapHandler(server.UpdateMonitor))
	api.Delete("/monitors/:id", wrapHandler(server.DeleteMonitor))
	api.Post("/monitors/:id/pause", wrapHandler(server.ToggleMonitorPause))
	api.Post("/auth/logout", wrapHandler(server.Logout))

	// Authenticated pages
	pages := app.Group("", authService.PageMiddleware())
	pages.Get("/dashboard", pageHandler.Dashboard)
	pages.Get("/monitors/:id", pageHandler.MonitorDetail)

	return app
}

// Wrapper functions to adapt Server methods to Fiber handlers
// These bridge the oapi-codegen interface to Fiber's handler signature
func wrapList(s *Server) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := auth.UserFromCtx(c)
		result, err := s.ListMonitors(c.Context(), user)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	}
}

func wrapCreate(s *Server) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := auth.UserFromCtx(c)
		// Parse request body and call s.CreateMonitor
		// ...
		return nil
	}
}
```

Note: The exact route registration depends on oapi-codegen's generated `RegisterHandlers` function. If using strict server mode, oapi-codegen generates a `RegisterHandlersWithOptions` function that takes a Fiber app and the server implementation. Adapt accordingly.

- [ ] **Step 2: Commit**

```bash
git add internal/handler/setup.go
git commit -m "feat: Fiber app setup with all routes and middleware"
```

---

## Task 13: Wire Everything in main.go

**Files:**
- Modify: `cmd/pingcast/main.go`

- [ ] **Step 1: Wire all components**

Update `cmd/pingcast/main.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/auth"
	"github.com/kirillinakin/pingcast/internal/checker"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/handler"
	"github.com/kirillinakin/pingcast/internal/notifier"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Database
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// sqlc queries
	queries := sqlcgen.New(pool)

	// Auth
	authService := auth.NewService(queries)
	rateLimiter := auth.NewRateLimiter(5, 15*time.Minute)

	// Notifier
	var tgSender *notifier.TelegramSender
	if cfg.TelegramToken != "" {
		tgSender = notifier.NewTelegramSender(cfg.TelegramToken)
	}

	var emailSender *notifier.EmailSender
	if cfg.SMTPHost != "" {
		emailSender = notifier.NewEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
	}

	listener := notifier.NewListener(pool, queries, tgSender, emailSender)
	listener.Start(ctx)

	// Check handler
	checkHandler := func(ctx context.Context, monitor *checker.MonitorInfo, result *checker.CheckResult) {
		queries.InsertCheckResult(ctx, sqlcgen.InsertCheckResultParams{
			MonitorID:      monitor.ID,
			Status:         string(result.Status),
			StatusCode:     result.StatusCode,
			ResponseTimeMs: int32(result.ResponseTimeMs),
			ErrorMessage:   result.ErrorMessage,
			CheckedAt:      result.CheckedAt,
		})

		// Read current status from DB
		currentMon, err := queries.GetMonitorByID(ctx, monitor.ID)
		previousStatus := "unknown"
		if err == nil {
			previousStatus = currentMon.CurrentStatus
		}

		queries.UpdateMonitorStatus(ctx, sqlcgen.UpdateMonitorStatusParams{
			ID:            monitor.ID,
			CurrentStatus: string(result.Status),
		})

		if previousStatus != string(result.Status) {
			if result.Status == "down" {
				failures, _ := queries.ConsecutiveFailures(ctx, monitor.ID)
				if failures >= int32(monitor.AlertAfterFailures) {
					inCooldown, _ := queries.IsInCooldown(ctx, monitor.ID)
					if !inCooldown {
						errMsg := ""
						if result.ErrorMessage != nil {
							errMsg = *result.ErrorMessage
						}
						queries.CreateIncident(ctx, sqlcgen.CreateIncidentParams{
							MonitorID: monitor.ID,
							Cause:     errMsg,
						})

						event, _ := json.Marshal(map[string]string{
							"monitor_id": monitor.ID.String(),
							"event":      "down",
							"details":    errMsg,
						})
						pool.Exec(ctx, "SELECT pg_notify('monitor_events', $1)", string(event))
					}
				}
			} else if result.Status == "up" && previousStatus == "down" {
				incident, err := queries.GetOpenIncidentByMonitorID(ctx, monitor.ID)
				if err == nil {
					now := time.Now()
					queries.ResolveIncident(ctx, sqlcgen.ResolveIncidentParams{
						ID:         incident.ID,
						ResolvedAt: &now,
					})
				}

				event, _ := json.Marshal(map[string]string{
					"monitor_id": monitor.ID.String(),
					"event":      "up",
					"details":    "recovered",
				})
				pool.Exec(ctx, "SELECT pg_notify('monitor_events', $1)", string(event))
			}
		}
	}

	// Checker
	client := checker.NewClient()
	workerPool := checker.NewWorkerPool(ctx, 100, client, checkHandler)

	scheduler := checker.NewScheduler(func(m *checker.MonitorInfo) {
		workerPool.Submit(m)
	})

	// Load existing monitors
	monitors, _ := queries.ListActiveMonitors(ctx)
	for _, m := range monitors {
		scheduler.Add(&checker.MonitorInfo{
			ID:                 m.ID,
			URL:                m.Url,
			Method:             m.Method,
			IntervalSeconds:    int(m.IntervalSeconds),
			ExpectedStatus:     int(m.ExpectedStatus),
			Keyword:            m.Keyword,
			AlertAfterFailures: int(m.AlertAfterFailures),
		})
	}
	slog.Info("loaded monitors", "count", len(monitors))

	// Monitor change callback
	onChanged := func(monitorID uuid.UUID, deleted bool) {
		if deleted {
			scheduler.Remove(monitorID)
		} else {
			mon, err := queries.GetMonitorByID(ctx, monitorID)
			if err != nil {
				return
			}
			if mon.IsPaused {
				scheduler.Remove(monitorID)
			} else {
				scheduler.Add(&checker.MonitorInfo{
					ID:                 mon.ID,
					URL:                mon.Url,
					Method:             mon.Method,
					IntervalSeconds:    int(mon.IntervalSeconds),
					ExpectedStatus:     int(mon.ExpectedStatus),
					Keyword:            mon.Keyword,
					AlertAfterFailures: int(mon.AlertAfterFailures),
				})
			}
		}
	}

	// Handlers
	pageHandler := handler.NewPageHandler(queries, authService, rateLimiter)
	server := handler.NewServer(queries, pool, authService, rateLimiter, onChanged)
	webhookHandler := handler.NewWebhookHandler(queries, cfg.LemonSqueezyWebhookSecret)

	app := handler.SetupApp(authService, pageHandler, server, webhookHandler)

	// Data retention cleanup (daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Pro: 90 days, Free: 7 days
				// Delete data older than 90 days (covers all plans)
				cutoff90 := time.Now().Add(-90 * 24 * time.Hour)
				deleted, err := queries.DeleteCheckResultsOlderThan(ctx, cutoff90)
				if err != nil {
					slog.Error("retention cleanup failed", "error", err)
				} else if deleted > 0 {
					slog.Info("retention cleanup (90d)", "deleted_rows", deleted)
				}

				// Note: Per-plan 7-day cleanup for free users requires a more complex query
				// joining check_results with monitors and users. Deferred to post-MVP.
				// For MVP, all users get 90-day retention.

				sessDeleted, err := queries.DeleteExpiredSessions(ctx)
				if err != nil {
					slog.Error("session cleanup failed", "error", err)
				} else if sessDeleted > 0 {
					slog.Info("session cleanup", "deleted", sessDeleted)
				}
			}
		}
	}()

	// Start server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Port)
		slog.Info("pingcast started", "port", cfg.Port, "monitors", len(monitors))
		if err := app.Listen(addr); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	scheduler.Stop()
	workerPool.Stop()
	app.Shutdown()

	slog.Info("shutdown complete")
}
```

Note: The checker package needs a `MonitorInfo` struct (lightweight, decoupled from sqlc models) so the checker doesn't depend on the sqlc package. Define it in `internal/checker/client.go`:

```go
type MonitorInfo struct {
	ID                 uuid.UUID
	URL                string
	Method             string
	IntervalSeconds    int
	ExpectedStatus     int
	Keyword            *string
	AlertAfterFailures int
}
```

And update the `Check` method to accept `*MonitorInfo` instead of `*model.Monitor`.

- [ ] **Step 2: Verify compilation**

```bash
go mod tidy && go build ./cmd/pingcast/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/pingcast/main.go
git commit -m "feat: wire all components with Fiber, sqlc, graceful shutdown"
```

---

## Task 14: Dockerfile & Docker Compose

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`

Same as previous plan. No changes needed for Fiber/sqlc.

- [ ] **Step 1: Create Dockerfile**

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /pingcast ./cmd/pingcast/

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /pingcast /pingcast
EXPOSE 8080
CMD ["/pingcast"]
```

- [ ] **Step 2: Create docker-compose.yml**

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://pingcast:pingcast@db:5432/pingcast?sslmode=disable
      - PORT=8080
      - BASE_URL=http://localhost:8080
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: pingcast
      POSTGRES_PASSWORD: pingcast
      POSTGRES_DB: pingcast
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U pingcast"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
```

- [ ] **Step 3: Commit**

```bash
git add Dockerfile docker-compose.yml
git commit -m "feat: Dockerfile and docker-compose"
```

---

## Task 15: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: pingcast
          POSTGRES_PASSWORD: pingcast
          POSTGRES_DB: pingcast_test
        ports:
          - 5432:5432
        options: >-
          --health-cmd="pg_isready -U pingcast"
          --health-interval=10s
          --health-timeout=5s
          --health-retries=5

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go mod download
      - run: go build ./...
      - run: go test ./... -v -count=1
        env:
          TEST_DATABASE_URL: postgres://pingcast:pingcast@localhost:5432/pingcast_test?sslmode=disable
      - run: go vet ./...
```

- [ ] **Step 1: Commit**

```bash
mkdir -p .github/workflows
git add .github/workflows/ci.yml
git commit -m "feat: GitHub Actions CI"
```

---

## Implementation Notes

- **Table partitioning deferred to post-MVP.** Simple `DELETE` cleanup via `DeleteCheckResultsOlderThan` is sufficient for the initial launch. Partitioning `check_results` by month can be added later when data volume justifies it.
- **`subscription_cancelled` should ideally respect period end.** MVP performs an immediate downgrade to free. Track as tech debt: the webhook should store `ends_at` from Lemon Squeezy and only downgrade after that date.
- **`subscription_payment_failed` grace period deferred.** MVP relies on Lemon Squeezy's built-in retry logic. No immediate plan change on payment failure.
- **`UpdateUserLemonSqueezy` should be called in the webhook handler.** In `HandleLemonSqueezy`, after updating the plan, also call `queries.UpdateUserLemonSqueezy(ctx, sqlcgen.UpdateUserLemonSqueezyParams{ID: user.ID, LemonSqueezyCustomerID: ..., LemonSqueezySubscriptionID: ...})` to persist the Lemon Squeezy IDs.
- **Consider `c.UserContext()` instead of `c.Context()` for Fiber handlers.** Fiber's `c.Context()` returns `*fasthttp.RequestCtx` which does not behave like a standard `context.Context` after the handler returns. For background-safe contexts (e.g., passing to goroutines), use `c.UserContext()` instead.
- **Add `requestid.New()` middleware for structured logging.** Import `github.com/gofiber/fiber/v2/middleware/requestid` and add `app.Use(requestid.New())` in `SetupApp` to attach a unique request ID to each request, making log correlation easier.

---

## Summary

15 tasks, ordered by dependency. Each task produces a working commit.

**Key differences from previous plan:**
- **sqlc** replaces hand-written store layer (Tasks 4-7 → Task 3)
- **oapi-codegen** generates API types and Fiber server interface (Task 4)
- **Fiber** replaces Chi throughout (handlers, middleware, app setup)
- **API-first**: JSON API for CRUD, page handlers for HTML rendering
- Fewer tasks (15 vs 21) because sqlc eliminates boilerplate

**Task order:**
1. Project scaffolding & config
2. Database & migrations
3. sqlc setup & query generation
4. OpenAPI spec & oapi-codegen
5. Auth service & Fiber middleware
6. HTTP checker client
7. Scheduler & worker pool
8. Notifier (PG LISTEN/NOTIFY + Telegram + email)
9. API server implementation (oapi-codegen interface)
10. HTML page handlers & templates
11. Webhook handlers (Lemon Squeezy + Telegram bot)
12. Fiber app setup & route wiring
13. Wire everything in main.go
14. Dockerfile & Docker Compose
15. GitHub Actions CI
