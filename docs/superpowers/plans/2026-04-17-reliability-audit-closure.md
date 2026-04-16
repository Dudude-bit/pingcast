# Reliability Audit Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Safely land the already-written ~1130-line uncommitted reliability-audit changes onto `main` as 6 clean, green-on-each commits, matching the PR boundaries defined in the parent spec.

**Architecture:** Verify-first workflow on a feature branch. Stage 0 captures dual baselines (shipped `main` + target working-tree). Stage 1 audits the diff against the 29 issues in the parent spec and records the result inline in the closure-design document's Verification Log. Stage 2 splits the unstaged diff into 6 ordered commits using `git add -p`, with a build+test gate after each commit. Stage 4 fast-forward-merges the feature branch into `main` and retains a safety tag for rollback.

**Tech Stack:** Go 1.x, `git`, `go test`, `go vet`, `go build`, `golangci-lint`, Postgres testcontainers for integration tests.

**Source specs:**
- Closure design: `docs/superpowers/specs/2026-04-17-reliability-audit-closure-design.md`
- Parent audit spec: `docs/superpowers/specs/2026-03-23-reliability-audit-design.md`

---

## File Structure

This plan does not create new source files — it lands existing uncommitted files into git. Target files fall into three groups:

**Group G1 — Audit-changed files (must end up committed across PR1–PR5):**
- `cmd/api/main.go`, `cmd/notifier/main.go`, `cmd/scheduler/main.go`, `cmd/worker/main.go`
- `internal/adapter/channel/registry.go`
- `internal/adapter/checker/http.go`
- `internal/adapter/http/{handler_test.go,middleware.go,pages.go,server.go,setup.go}`
- `internal/adapter/nats/{check_subscriber.go,client.go,publisher.go,subscriber.go}`
- `internal/adapter/postgres/{channel_repo.go,monitor_repo.go}`
- `internal/adapter/smtp/sender.go`, `internal/adapter/telegram/sender.go`, `internal/adapter/webhook/sender.go`
- `internal/app/{alert.go,alert_test.go,monitoring.go}`
- `internal/config/config.go`
- `internal/crypto/{crypto.go,crypto_test.go}`
- `internal/domain/errors.go`
- `internal/mocks/mocks.go`
- `internal/observability/metrics.go`
- `internal/port/{channel.go,eventbus.go,metrics.go,repository.go}`
- `internal/sqlc/gen/monitors.sql.go`, `internal/sqlc/queries/monitors.sql`
- `tests/integration/repo_test.go`

**Group G2 — New files/dirs that ARE part of the audit (must end up committed):**
- `internal/adapter/httperr/classify.go` — PR4b (issue 4.4)
- `internal/bootstrap/cipher.go` — PR5 (issue 4.10 wiring)
- `internal/database/migrations/016_incident_uniqueness.sql` — PR4a (issue 4.6)
- `internal/port/crypto.go` — PR5 (issue 4.10 port)
- `internal/sqlc/internal/` — classify in Stage 1; most likely PR1 or PR2 by dependency
- `internal/xcontext/detached.go` — PR1 (shared utility)

**Group G3 — Untracked and OUT of scope (do NOT commit in this plan):**
- `docs/articles/` — Habr launch article work, unrelated to audit.

**Closure-design document (single file modified during Stage 1):**
- `docs/superpowers/specs/2026-04-17-reliability-audit-closure-design.md` — append Verification Log entries in Stage 1.

---

## Preconditions

Before Task 1:
- Working directory is `/Users/kirillinakin/GolandProjects/pingcast`.
- `git status --short` shows: 36 modified Go files, 6 untracked dirs/files (`docs/articles/`, `internal/adapter/httperr/`, `internal/bootstrap/`, `internal/database/migrations/016_incident_uniqueness.sql`, `internal/port/crypto.go`, `internal/sqlc/internal/`, `internal/xcontext/`), and `HEAD` commit is `docs: add reliability-audit closure design spec`.
- `go`, `golangci-lint`, and `docker` (for testcontainers) are available on `PATH`.

---

## Task 1: Stage 0 — Snapshot and Dual Baseline

**Goal:** Establish a safe rollback point and confirm both the shipped `main` and the working-tree target state are green before touching anything.

**Files:**
- Modify: git state only (tag, branch, stash).
- Create: `/tmp/pingcast-baseline-A.log`, `/tmp/pingcast-baseline-B.log`.

- [ ] **Step 1.1: Tag current `HEAD` as rollback point**

```bash
git tag pre-audit-closure
git tag --list pre-audit-closure
```

Expected output:
```
pre-audit-closure
```

- [ ] **Step 1.2: Stash the full working tree (including untracked)**

```bash
git stash push -u -m "audit-closure-stage-0"
git status --short
```

Expected output: only `?? docs/articles/` remains (not stashed if already ignored — verify it's present; if `-u` stashed it, that's OK, we restore later and re-exclude).

If `docs/articles/` was stashed unintentionally because `-u` captures untracked, that's fine — it will come back on unstash and be excluded in Stage 2.

- [ ] **Step 1.3: Run Baseline A (shipped `main`)**

```bash
{ go build ./... && \
  go vet ./... && \
  go test -count=1 ./... && \
  go test -race -count=1 ./... && \
  golangci-lint run; } 2>&1 | tee /tmp/pingcast-baseline-A.log
echo "Exit: $?"
```

Expected: all 5 commands succeed; last line is `Exit: 0`. `go test` output contains lines beginning with `ok ` for each package and zero `FAIL`.

**Gate:** If Baseline A fails, STOP — the `main` branch itself is broken and must be fixed before continuing. Unstash and investigate before restarting.

- [ ] **Step 1.4: Create the feature branch and unstash**

```bash
git checkout -b reliability-audit-closure
git stash pop
git status --short | wc -l
```

Expected: `43` (or close — 36 modified + 6 new dirs/files + `docs/articles/`).

- [ ] **Step 1.5: Run Baseline B (working-tree target)**

```bash
{ go build ./... && \
  go vet ./... && \
  go test -count=1 ./... && \
  go test -race -count=1 ./... && \
  golangci-lint run; } 2>&1 | tee /tmp/pingcast-baseline-B.log
echo "Exit: $?"
```

Expected: all 5 commands succeed; `Exit: 0`.

**Gate:** If Baseline B fails:
- If the failure is in Go code (build/vet/test/race/lint): DO NOT proceed. Record the failure in the closure-spec's Verification Log as a ❌ entry for the relevant issue, fix in the working tree, re-run Step 1.5, repeat until green.
- If a test is flaky, retry up to 2 times to confirm. If persistently flaky, treat as a ❌ finding requiring a fix in working tree.

- [ ] **Step 1.6: Verify rollback path works**

```bash
git log --oneline pre-audit-closure -1
git log --oneline main -1
git diff --stat pre-audit-closure..HEAD
```

Expected: both tags/branches point to the same commit (`docs: add reliability-audit closure design spec`). The `git diff --stat` output is empty (no committed changes since the tag) — confirming rollback via `git reset --hard pre-audit-closure` would restore this exact point.

- [ ] **Step 1.7: No commit here — setup only.**

Baselines are logs in `/tmp/`, not files in the repo. This task produces no git commit.

---

## Task 2: Stage 1 — Audit Diff Against 29 Parent-Spec Issues

**Goal:** For each of the 29 issues in the parent spec, confirm the working-tree diff implements the prescribed change correctly, and record the result in the closure-design doc's Verification Log.

**Files:**
- Modify: `docs/superpowers/specs/2026-04-17-reliability-audit-closure-design.md` — fill in the Verification Log section.

### Audit procedure (apply per issue)

For each issue identified by its parent-spec number (1.1 through 4.10), perform these steps:

1. Open the parent spec at `docs/superpowers/specs/2026-03-23-reliability-audit-design.md` and re-read the issue's `**Problem:**` and `**Fix:**` paragraphs.
2. For each file listed in the issue's `**Files:**`, run `git diff -- <file>` and read every hunk.
3. Compare hunks to the prescribed `**Fix:**` — are all prescribed changes present? Any extra changes not justified by the spec? Any missing prescribed changes?
4. Classify as ✅ / ⚠ / ❌ and append a row to the Verification Log.

- [ ] **Step 2.1: Audit Component 1 — NATS Messaging (issues 1.1–1.6)**

Per-issue checklist (tick each when its row is added to the log):

- [ ] 1.1 — app-layer publish error handling + correct DeleteMonitor ordering. Check: `internal/app/monitoring.go` creates/updates/deletes wrap publish errors; DeleteMonitor publishes AFTER DB delete.
- [ ] 1.2 — per-message context in subscribe closures. Check: `internal/adapter/nats/subscriber.go` MonitorSubscriber uses 5s timeout, AlertSubscriber 30s; `internal/adapter/nats/check_subscriber.go` uses 60s. Contexts derived from `context.Background()` (or `xcontext.Detached`), not the captured app context.
- [ ] 1.3 — `MaxDeliver: 5` + BackOff on MonitorSubscriber in `internal/adapter/nats/subscriber.go`.
- [ ] 1.4 — `MaxBytes` on MONITORS/CHECKS/ALERTS streams in `internal/adapter/nats/client.go` (1 GB / 100 MB / 1 GB).
- [ ] 1.5 — event DTO layer: `internal/port/eventbus.go` defines `MonitorChangedEvent`; publisher/subscriber signatures use the DTO; domain-to-DTO mapping happens in app layer, not adapter.
- [ ] 1.6 — publish moved into `MonitoringService` (app layer); `internal/adapter/http/server.go` no longer publishes events; `events` field removed from `Server` struct; wiring in `cmd/api/main.go` updated.

Row template for Verification Log (append one per issue):

```markdown
| 1.1 | ✅ | internal/app/monitoring.go | publish errors wrapped to domain.ErrEventPublishFailed; DeleteMonitor DB-first, publish-after |
```

- [ ] **Step 2.2: Audit Component 2 — Notifier (issues 2.1–2.9)**

- [ ] 2.1 — `internal/adapter/smtp/sender.go`: uses `net/smtp` with dial timeout + context cancellation; no longer `_ context.Context`.
- [ ] 2.2 — `internal/app/alert.go` parallel delivery via `errgroup.Group` with `SetLimit(10)`; per-channel 10s timeout; group 30s timeout.
- [ ] 2.3 — `internal/app/alert.go` DLQ write retries 3 times with 1s/2s/4s backoff; final failure logs at Error level; NOT returned to NATS.
- [ ] 2.4 — `cmd/notifier/main.go` constructs and starts `DLQConsumer`.
- [ ] 2.5 — `internal/adapter/channel/registry.go` per-channel CBs keyed by `channelType:channelID` in `sync.Map`; 1h TTL eviction.
- [ ] 2.6 — `io.Copy(io.Discard, resp.Body)` in `internal/adapter/telegram/sender.go`, `internal/adapter/webhook/sender.go`, `internal/adapter/checker/http.go`.
- [ ] 2.7 — BackOff array has 10 elements in `internal/adapter/nats/subscriber.go` (AlertSubscriber): `2s, 5s, 10s, 30s, 60s, 120s, 120s, 120s, 120s, 120s`.
- [ ] 2.8 — `RecordAlertSent` in `internal/observability/metrics.go` / `internal/port/metrics.go` adds `reason` label; `internal/app/alert.go` classifies errors to reasons.
- [ ] 2.9 — `internal/port/channel.go` method renamed `CreateSenderWithRetry` → `CreateSender`; all call sites updated in `internal/app/alert.go`; adapter still wraps retry+CB internally.

- [ ] **Step 2.3: Audit Component 3 — Checker (issues 3.1–3.4)**

- [ ] 3.1 — `cmd/scheduler/main.go` calls `monitorSub.Subscribe()` before `go leaderScheduler.Run()`.
- [ ] 3.2 — `cmd/scheduler/main.go` adds `sync.WaitGroup` for cleanup goroutine; shutdown waits before closing DB.
- [ ] 3.3 — `cmd/scheduler/main.go` differentiates `redsync.ErrFailed` (Debug) from other errors (Warn).
- [ ] 3.4 — `internal/adapter/checker/http.go` caches `*http.Client` by timeout in `sync.Map`.

- [ ] **Step 2.4: Audit Component 4 — API (issues 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 4.10)**

Note: Issue 4.9 is explicitly REMOVED in the parent spec (see §4.9 strikethrough) — skip.

- [ ] 4.1 — `internal/sqlc/queries/monitors.sql` contains a CTE-based atomic status-update query returning pre-update `current_status`; `internal/adapter/postgres/monitor_repo.go` and `internal/app/monitoring.go` use it.
- [ ] 4.2 — `ListAPIKeys`, `CreateAPIKey`, `RevokeAPIKey` in `internal/adapter/http/server.go` nil-check via a helper (e.g., `requireUser`).
- [ ] 4.3 — `internal/sqlc/queries/monitors.sql` `TogglePause` is a single atomic UPDATE RETURNING the full monitor; repo/app layer use it.
- [ ] 4.4 — `internal/adapter/httperr/classify.go` exists and maps domain errors to client-safe strings; `internal/domain/errors.go` defines `ErrUserExists`; `internal/adapter/http/{server.go,setup.go,pages.go}` use the classifier instead of raw `err.Error()`.
- [ ] 4.5 — `internal/adapter/http/middleware.go` background Touch uses `context.WithTimeout(context.Background(), 5*time.Second)` and a semaphore (buffered channel, cap ~50).
- [ ] 4.6 — `internal/database/migrations/016_incident_uniqueness.sql` adds the partial unique index; postgres adapter catches the constraint violation.
- [ ] 4.7 — `internal/adapter/http/server.go` applies `rateLimiter.Allow()` to `/api/auth/register` (5 per 15min, by IP).
- [ ] 4.8 — `internal/adapter/http/pages.go` validates `interval_seconds` (30..86400) and `alert_after_failures` (1..10); returns 400 with message on violation.
- [ ] 4.10 — `internal/port/crypto.go` defines `Cipher` interface + `NoOpCipher`; `internal/crypto/crypto.go` implements key-versioned AES-256-GCM with `[version][nonce][ct+tag]` format and `NeedsReEncryption`; `internal/config/config.go` parses `ENCRYPTION_KEYS` / `ENCRYPTION_PRIMARY_VERSION`; `internal/adapter/postgres/{monitor_repo.go,channel_repo.go}` depend on `port.Cipher` (single constructor); `internal/bootstrap/cipher.go` wires the cipher from config; `cmd/api/main.go` uses bootstrap.

- [ ] **Step 2.5: Classify extra (not-in-parent-spec) files**

For each untracked path that is NOT `docs/articles/`:

- `internal/xcontext/detached.go` → classify as part of parent spec §Shared Utility → PR1.
- `internal/adapter/httperr/classify.go` → part of 4.4 extraction → PR4b (already implied in Step 2.4 row 4.4).
- `internal/bootstrap/cipher.go` → part of 4.10 wiring → PR5.
- `internal/database/migrations/016_incident_uniqueness.sql` → part of 4.6 → PR4a.
- `internal/port/crypto.go` → part of 4.10 → PR5.
- `internal/sqlc/internal/` → run `ls internal/sqlc/internal/` and inspect contents. If it contains generated mocks used by any Go test file touched in the audit, include with the **first PR** whose code depends on them. If contents are unclear, default to PR1 (earliest PR).

Add an "Extra-files classification" subsection to the Verification Log with one line per path.

- [ ] **Step 2.6: Append Verification Log to closure-design doc**

Open `docs/superpowers/specs/2026-04-17-reliability-audit-closure-design.md` and replace the empty `### Issues table`, `### Extra-files classification`, and `### Deviation decisions` placeholders with the populated tables from Steps 2.1–2.5.

- [ ] **Step 2.7: Check Verification Log has no ❌ or unresolved ⚠**

```bash
grep -c '| ❌ |' docs/superpowers/specs/2026-04-17-reliability-audit-closure-design.md
grep -c '| ⚠ |' docs/superpowers/specs/2026-04-17-reliability-audit-closure-design.md
```

Expected: `0` for ❌.
For ⚠: each ⚠ row must have a "Notes" column explaining either (a) why the deviation is acceptable, or (b) what the fix is. If a fix is needed, proceed to Task 3.

- [ ] **Step 2.8: Commit the Verification Log**

```bash
git add docs/superpowers/specs/2026-04-17-reliability-audit-closure-design.md
git commit -m "$(cat <<'EOF'
docs: fill reliability-audit closure verification log

Per-issue audit of the 29 parent-spec issues against the uncommitted
working-tree diff, plus classification of extra (not-in-parent-spec)
files to their target PRs.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git log --oneline -1
```

Expected: commit created on `reliability-audit-closure` branch with exactly 1 file changed.

---

## Task 3: Resolve ❌ / ⚠ Findings from Stage 1 (Conditional)

**Condition:** Only execute if Task 2 identified ❌ entries or ⚠ entries that require fixes. If all 29 issues are ✅, skip to Task 4.

**Goal:** Fix each ❌ in the working tree so Baseline B is green and every issue matches the parent spec.

**Files:** any file listed in a ❌ or fix-required ⚠ row.

- [ ] **Step 3.1: For each ❌ or fix-required ⚠ entry, fix in place**

Per entry:
1. Open the file at the prescribed location.
2. Apply the change described in the parent spec's `**Fix:**` paragraph for the matching issue.
3. Run `go build ./<package-path>` — must succeed.
4. Run the relevant unit test: `go test -count=1 ./<package-path>` — must pass.

- [ ] **Step 3.2: Re-run full Baseline B**

```bash
{ go build ./... && \
  go vet ./... && \
  go test -count=1 ./... && \
  go test -race -count=1 ./... && \
  golangci-lint run; } 2>&1 | tee /tmp/pingcast-baseline-B-after-fixes.log
echo "Exit: $?"
```

Expected: `Exit: 0`.

- [ ] **Step 3.3: Update Verification Log entries to ✅**

Change each fixed row's status from ❌ / ⚠ to ✅ and add a "fixed in closure Stage 1" note. Re-commit the log update:

```bash
git add docs/superpowers/specs/2026-04-17-reliability-audit-closure-design.md
git commit -m "docs: update verification log — fixes applied in closure stage 1"
```

- [ ] **Step 3.4: Do NOT commit working-tree fixes as a separate commit**

The working-tree fixes stay unstaged — they will be split into PR1–PR5 commits in Tasks 4–9 alongside the original audit changes for the same issue.

---

## Task 4: PR1 — NATS Messaging + Event Architecture (issues 1.1–1.6)

**Goal:** Produce a single commit on `reliability-audit-closure` containing ONLY the changes for issues 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, plus `internal/xcontext/detached.go` (shared utility used here first), plus any `internal/sqlc/internal/` items assigned to PR1 in Step 2.5.

**Files (prescribed by parent spec §1.1–1.6 + §Shared Utility + Step 2.5):**
- Modify: `internal/port/eventbus.go`, `internal/adapter/nats/{client.go,publisher.go,subscriber.go,check_subscriber.go}`, `internal/app/monitoring.go`, `internal/adapter/http/server.go`, `cmd/api/main.go`
- Create: `internal/xcontext/detached.go`
- Conditional (per Step 2.5): `internal/sqlc/internal/` contents if classified to PR1

**Files NOT in this commit (will be committed in later PRs):**
- Anything touching issues 2.x, 3.x, 4.x — even if they share a file with PR1 (e.g., `subscriber.go` issue 2.7 BackOff extension, or `server.go` issue 4.2 nil-checks).

- [ ] **Step 4.1: Add the new file first (safe — whole-file add)**

```bash
git add internal/xcontext/detached.go
```

If Step 2.5 classified `internal/sqlc/internal/` to PR1:
```bash
git add internal/sqlc/internal/
```

- [ ] **Step 4.2: Stage per-issue hunks using `git add -p`**

For each file below, run `git add -p <file>` and for each hunk answer `y` only if the hunk implements issue 1.1–1.6, otherwise `n`:

- `internal/port/eventbus.go` — all hunks belong to 1.5 (DTO). Answer `y` to all.
- `internal/adapter/nats/publisher.go` — hunks reflect 1.5 (signature change) and 1.1 (error return). Answer `y` to all.
- `internal/adapter/nats/client.go` — hunks reflect 1.4 (MaxBytes). Answer `y` to all.
- `internal/adapter/nats/subscriber.go` — hunks for 1.2 (per-message ctx), 1.3 (MaxDeliver/BackOff on MonitorSubscriber) → `y`. Hunks for 2.7 (AlertSubscriber BackOff extension to 10 elements) → `n` (PR2).
- `internal/adapter/nats/check_subscriber.go` — hunks for 1.2 (per-message ctx) → `y`. Any hunks for issue 3.x → `n` (PR3).
- `internal/app/monitoring.go` — hunks for 1.1 (publish error wrapping), 1.5 (DTO construction), 1.6 (publish-in-service) → `y`. Hunks for 4.1 (atomic ProcessCheckResult) or 4.3 (atomic TogglePause) → `n` (PR4a).
- `internal/adapter/http/server.go` — hunks removing `events` publishes (1.6) → `y`. Hunks for 4.2 (nil-check), 4.4 (error sanitization), 4.7 (register rate-limit) → `n` (PR4b).
- `cmd/api/main.go` — hunks wiring the publisher into `MonitoringService` (1.6) → `y`. Hunks wiring `bootstrap.Cipher` (4.10) → `n` (PR5).

If a hunk is genuinely inseparable (i.e., the hunk contains both PR1 and non-PR1 changes in the same contiguous range), split it: press `s` in `git add -p` to split, or `e` to edit the hunk manually, keeping only the PR1 lines staged.

If splitting still fails (e.g., a single-line change that implements both issues), **do not force-split**. Record this in the Verification Log under `### PR-merges (if tangling forced any)` and fold the affected later PR's content into PR1 — document which issues are now in PR1.

- [ ] **Step 4.3: Verify the staged set compiles**

```bash
git stash push -u -k -m "pr1-unstaged-leftover"
go build ./...
echo "Exit: $?"
```

Expected: `Exit: 0`.

**Gate:** If build fails, the PR1 staging is missing a required hunk (or has one too many). Pop the stash (`git stash pop`) and revise Step 4.2 selections.

- [ ] **Step 4.4: Run tests on the staged-only state**

```bash
go vet ./...
go test -count=1 ./...
go test -race -count=1 ./...
golangci-lint run
echo "Exit: $?"
```

Expected: `Exit: 0`.

**Gate:** Any failure → `git stash pop` and revise staging. Do not commit until green.

- [ ] **Step 4.5: Commit PR1**

```bash
git commit -m "$(cat <<'EOF'
feat: PR1 — NATS messaging + event architecture (reliability audit 1.1–1.6)

Issues addressed:
- 1.1: app-layer publish error handling + correct Delete ordering
- 1.2: per-message context in subscribe closures
- 1.3: MaxDeliver + BackOff on MonitorSubscriber
- 1.4: MaxBytes on MONITORS/CHECKS/ALERTS streams
- 1.5: event DTO layer — decouple events from domain models
- 1.6: move event publishing into app layer (MonitoringService)

Shared utility: internal/xcontext/detached.go (detached-context helper
used by 1.2 and later issues).

Spec: docs/superpowers/specs/2026-03-23-reliability-audit-design.md §1

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 4.6: Restore the remaining unstaged changes**

```bash
git stash pop
git status --short | wc -l
```

Expected: count should be original_count - (files_fully_committed_in_PR1). Files fully committed (no remaining hunks) disappear; files with leftover hunks still show as `M`.

- [ ] **Step 4.7: Post-commit verification**

```bash
go build ./... && go test -count=1 ./...
echo "Exit: $?"
```

Expected: `Exit: 0` (working tree still has PR2–PR5 changes unstaged but should still compile and pass tests — the audit was designed so each PR boundary is a valid intermediate state).

If it fails, the PR boundary is not independent — this indicates a spec-design issue. Record in Verification Log and either (a) add the offending hunks to PR1 retroactively via `git commit --amend`, or (b) proceed to PR2 noting the coupling.

---

## Task 5: PR2 — Notifier (issues 2.1–2.9)

**Goal:** Commit containing only issues 2.1–2.9.

**Files (prescribed by parent spec §2.1–2.9):**
- `internal/adapter/smtp/sender.go` (2.1)
- `internal/app/alert.go`, `internal/app/alert_test.go` (2.2, 2.3, 2.8)
- `cmd/notifier/main.go` (2.4)
- `internal/adapter/channel/registry.go` (2.5)
- `internal/adapter/telegram/sender.go`, `internal/adapter/webhook/sender.go`, `internal/adapter/checker/http.go` (2.6)
- `internal/adapter/nats/subscriber.go` (2.7 — remaining AlertSubscriber BackOff hunks not staged in PR1)
- `internal/observability/metrics.go`, `internal/port/metrics.go` (2.8)
- `internal/port/channel.go` (2.9 rename)
- `internal/mocks/mocks.go` (if mocks regenerated for port rename / DTO change — include here)

- [ ] **Step 5.1: Stage PR2 hunks**

For each file in the list above, `git add -p <file>` and select only hunks matching issues 2.1–2.9:

- `internal/adapter/smtp/sender.go` — all hunks → `y`.
- `internal/app/alert.go` — all remaining hunks → `y` (assuming only 2.x changes; re-check that no 1.x hunks remain).
- `internal/app/alert_test.go` — all remaining hunks → `y`.
- `cmd/notifier/main.go` — all hunks → `y`.
- `internal/adapter/channel/registry.go` — all hunks → `y`.
- `internal/adapter/telegram/sender.go`, `internal/adapter/webhook/sender.go` — all hunks → `y`.
- `internal/adapter/checker/http.go` — hunks for 2.6 (body drain) → `y`; hunks for 3.4 (HTTP client cache) → `n` (PR3).
- `internal/adapter/nats/subscriber.go` — remaining hunks (2.7 AlertSubscriber BackOff) → `y`.
- `internal/observability/metrics.go`, `internal/port/metrics.go` — hunks for 2.8 → `y`.
- `internal/port/channel.go` — all hunks → `y`.
- `internal/mocks/mocks.go` — hunks regenerated for DTO or port rename → `y`.

- [ ] **Step 5.2: Build + test the staged set**

```bash
git stash push -u -k -m "pr2-unstaged-leftover"
go build ./... && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./... && golangci-lint run
echo "Exit: $?"
```

Expected: `Exit: 0`. If red → pop stash, revise.

- [ ] **Step 5.3: Commit PR2**

```bash
git commit -m "$(cat <<'EOF'
feat: PR2 — notifier reliability fixes (audit 2.1–2.9)

Issues addressed:
- 2.1: SMTP sender — context + timeout
- 2.2: parallel channel delivery with errgroup (limit=10)
- 2.3: reliable DLQ write with retries
- 2.4: wire DLQConsumer in notifier main
- 2.5: per-channel circuit breaker (TTL eviction)
- 2.6: response body drain across HTTP senders/checker
- 2.7: BackOff array matches MaxDeliver=10
- 2.8: error classification label on alert metrics
- 2.9: rename ChannelRegistry.CreateSenderWithRetry → CreateSender

Spec: docs/superpowers/specs/2026-03-23-reliability-audit-design.md §2

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5.4: Restore remaining unstaged + post-commit verification**

```bash
git stash pop
go build ./... && go test -count=1 ./...
echo "Exit: $?"
```

Expected: `Exit: 0`.

---

## Task 6: PR3 — Checker (issues 3.1–3.4)

**Goal:** Commit containing only issues 3.1–3.4.

**Files:**
- `cmd/scheduler/main.go` (3.1, 3.2, 3.3)
- `cmd/worker/main.go` (any checker-side shutdown or subscribe-before-run changes — verify in Step 2.3)
- `internal/adapter/checker/http.go` (3.4 — remaining HTTP client cache hunks not staged in PR2)
- `internal/adapter/nats/check_subscriber.go` (any remaining hunks for 3.x)

- [ ] **Step 6.1: Stage PR3 hunks**

```bash
git add -p cmd/scheduler/main.go     # all remaining hunks: 3.1, 3.2, 3.3
git add -p cmd/worker/main.go        # all remaining hunks
git add -p internal/adapter/checker/http.go  # remaining 3.4 hunks
git add -p internal/adapter/nats/check_subscriber.go  # any remaining
```

For each: answer `y` to all remaining hunks (PR1 and PR2 should have consumed the non-3.x hunks already).

- [ ] **Step 6.2: Build + test**

```bash
git stash push -u -k -m "pr3-unstaged-leftover"
go build ./... && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./... && golangci-lint run
echo "Exit: $?"
```

Expected: `Exit: 0`.

- [ ] **Step 6.3: Commit PR3**

```bash
git commit -m "$(cat <<'EOF'
feat: PR3 — checker reliability fixes (audit 3.1–3.4)

Issues addressed:
- 3.1: subscribe before Run (avoid missed monitor updates)
- 3.2: cleanup goroutine WaitGroup + graceful stop
- 3.3: differentiate lock-held vs infrastructure errors
- 3.4: cache HTTP client by timeout (sync.Map)

Spec: docs/superpowers/specs/2026-03-23-reliability-audit-design.md §3

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git stash pop
go build ./... && go test -count=1 ./...
```

Expected final line: `Exit: 0` equivalent (all green).

---

## Task 7: PR4a — API Race Conditions (issues 4.1, 4.3, 4.6)

**Goal:** Commit containing only the high-risk DB atomicity fixes.

**Files:**
- `internal/sqlc/queries/monitors.sql` (4.1 CTE, 4.3 atomic TogglePause)
- `internal/sqlc/gen/monitors.sql.go` (generated)
- `internal/adapter/postgres/monitor_repo.go` (remaining hunks for 4.1, 4.3)
- `internal/app/monitoring.go` (remaining hunks for 4.1, 4.3 usage if not already in PR1)
- `internal/adapter/postgres/channel_repo.go` (if 4.6 constraint handling touches it — typically not)
- `tests/integration/repo_test.go` (coverage for atomic operations and incident uniqueness)
- `internal/database/migrations/016_incident_uniqueness.sql` (new file, 4.6)
- `internal/port/repository.go` (if repo signature changed for 4.3 returning full monitor)
- `internal/domain/errors.go` (if 4.6 introduces `ErrIncidentInCooldown` or similar)

- [ ] **Step 7.1: Add new migration file**

```bash
git add internal/database/migrations/016_incident_uniqueness.sql
```

- [ ] **Step 7.2: Stage PR4a hunks**

```bash
git add -p internal/sqlc/queries/monitors.sql
git add -p internal/sqlc/gen/monitors.sql.go
git add -p internal/adapter/postgres/monitor_repo.go
git add -p internal/app/monitoring.go    # only if 4.1/4.3 hunks remain
git add -p internal/adapter/postgres/channel_repo.go
git add -p tests/integration/repo_test.go
git add -p internal/port/repository.go
git add -p internal/domain/errors.go     # if 4.6 error sentinel added here and not already in PR4b
```

Answer `y` to hunks matching 4.1, 4.3, 4.6. Answer `n` to any 4.10 hunks (PR5) in these files.

- [ ] **Step 7.3: Build + test**

Integration tests in `tests/integration/repo_test.go` require Docker running for testcontainers.

```bash
git stash push -u -k -m "pr4a-unstaged-leftover"
go build ./... && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./... && golangci-lint run
echo "Exit: $?"
```

Expected: `Exit: 0`. Integration tests must pass — this is the PR where race fixes are validated.

- [ ] **Step 7.4: Commit PR4a**

```bash
git commit -m "$(cat <<'EOF'
feat: PR4a — API race condition fixes (audit 4.1, 4.3, 4.6)

Issues addressed:
- 4.1: atomic ProcessCheckResult via CTE (pre-update status snapshot)
- 4.3: atomic TogglePause returning full monitor row
- 4.6: incident cooldown via unique partial index + constraint catch

Includes migration 016_incident_uniqueness.sql. Pre-migration
duplicate-cleanup guidance is in parent spec §4.6.

Spec: docs/superpowers/specs/2026-03-23-reliability-audit-design.md §4.1, §4.3, §4.6

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git stash pop
go build ./... && go test -count=1 ./...
```

---

## Task 8: PR4b — API Defensive Coding (issues 4.2, 4.4, 4.5, 4.7, 4.8)

**Goal:** Commit containing only the defensive coding fixes (user-facing error sanitization, rate limiting, validation, nil checks, detached background context).

**Files:**
- `internal/adapter/http/server.go` (remaining hunks for 4.2, 4.4, 4.7)
- `internal/adapter/http/setup.go` (4.4 — global error handler sanitization)
- `internal/adapter/http/pages.go` (4.4 HTML error rendering, 4.8 range validation)
- `internal/adapter/http/middleware.go` (4.5 — detached context + semaphore)
- `internal/adapter/http/handler_test.go` (test coverage for the above)
- `internal/adapter/httperr/classify.go` (new, 4.4)
- `internal/domain/errors.go` (remaining hunks for 4.4 — `ErrUserExists` sentinel, if not already in PR4a)

- [ ] **Step 8.1: Add new file**

```bash
git add internal/adapter/httperr/classify.go
```

- [ ] **Step 8.2: Stage PR4b hunks**

```bash
git add -p internal/adapter/http/server.go
git add -p internal/adapter/http/setup.go
git add -p internal/adapter/http/pages.go
git add -p internal/adapter/http/middleware.go
git add -p internal/adapter/http/handler_test.go
git add -p internal/domain/errors.go
```

Answer `y` to remaining hunks (all non-4.10 hunks should be PR4b by this point).

- [ ] **Step 8.3: Build + test**

```bash
git stash push -u -k -m "pr4b-unstaged-leftover"
go build ./... && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./... && golangci-lint run
echo "Exit: $?"
```

Expected: `Exit: 0`.

- [ ] **Step 8.4: Commit PR4b**

```bash
git commit -m "$(cat <<'EOF'
feat: PR4b — API defensive coding (audit 4.2, 4.4, 4.5, 4.7, 4.8)

Issues addressed:
- 4.2: nil-check UserFromCtx across API-key handlers via requireUser helper
- 4.4: error sanitization (httperr.Classify) + ErrUserExists sentinel
- 4.5: background Touch uses detached context + semaphore
- 4.7: rate-limit /api/auth/register (5 per 15min, by IP)
- 4.8: range validation on interval_seconds and alert_after_failures

Spec: docs/superpowers/specs/2026-03-23-reliability-audit-design.md §4.2, §4.4, §4.5, §4.7, §4.8

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git stash pop
go build ./... && go test -count=1 ./...
```

---

## Task 9: PR5 — Encryption Overhaul (issue 4.10)

**Goal:** Commit the key-versioned encryption overhaul with `port.Cipher` interface.

**Files:**
- `internal/port/crypto.go` (new)
- `internal/crypto/crypto.go`, `internal/crypto/crypto_test.go` (rewrite + tests)
- `internal/config/config.go` (ENCRYPTION_KEYS / PRIMARY_VERSION parsing)
- `internal/bootstrap/cipher.go` (new, DI wiring)
- `internal/adapter/postgres/monitor_repo.go`, `internal/adapter/postgres/channel_repo.go` (remaining hunks for `port.Cipher` dep)
- `cmd/api/main.go` (remaining hunks wiring bootstrap.Cipher)
- `internal/domain/errors.go` (if cipher-specific errors not already committed)
- `internal/mocks/mocks.go` (if mocks for `port.Cipher` added, and not already in PR1/PR2)

- [ ] **Step 9.1: Add new files**

```bash
git add internal/port/crypto.go
git add internal/bootstrap/cipher.go
```

- [ ] **Step 9.2: Stage PR5 hunks (all remaining hunks should be 4.10)**

```bash
git add -p internal/crypto/crypto.go
git add -p internal/crypto/crypto_test.go
git add -p internal/config/config.go
git add -p internal/adapter/postgres/monitor_repo.go
git add -p internal/adapter/postgres/channel_repo.go
git add -p cmd/api/main.go
git add -p internal/domain/errors.go
git add -p internal/mocks/mocks.go
```

Answer `y` to all remaining hunks.

- [ ] **Step 9.3: Verify no leftover audit changes in working tree**

```bash
git status --short | grep -v '^??' | grep -v '^A' | grep -v '^$'
```

Expected: empty output (all audit changes staged). Untracked output (e.g., `?? docs/articles/`) is fine — it's out of scope.

If non-empty output remains: those are unclassified audit changes. Stop and reconcile against the Verification Log before committing.

- [ ] **Step 9.4: Build + test**

```bash
git stash push -u -k -m "pr5-unstaged-leftover"
go build ./... && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./... && golangci-lint run
echo "Exit: $?"
```

Expected: `Exit: 0`.

- [ ] **Step 9.5: Commit PR5**

```bash
git commit -m "$(cat <<'EOF'
feat: PR5 — key-versioned encryption + hex-arch port (audit 4.10)

- Replaces single-key Encryptor with version-prefixed AES-256-GCM
  ciphertext: [1B version][12B nonce][ct+16B GCM tag], AD=[version]
- Adds port.Cipher interface + NoOpCipher for disabled-encryption mode
- Repos depend on port.Cipher (single constructor); no more dual
  NewMonitorRepoWithEncryption
- ENCRYPTION_KEYS=version:base64key,... + ENCRYPTION_PRIMARY_VERSION
  (legacy ENCRYPTION_KEY treated as v1)
- NeedsReEncryption() detects stale-key ciphertext for batch migration

Spec: docs/superpowers/specs/2026-03-23-reliability-audit-design.md §4.10

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git stash pop 2>/dev/null || true
go build ./... && go test -count=1 ./...
```

If there is no stash to pop (we staged all remaining hunks), `git stash pop` may error — that's fine (`|| true` silences it).

- [ ] **Step 9.6: Verify only out-of-scope items remain**

```bash
git status --short
```

Expected: only `?? docs/articles/` remains (out of scope).

---

## Task 10: Stage 4 — Final Integration and Merge to main

**Goal:** Verify the six new commits match the planned structure, confirm green on the final `HEAD`, fast-forward-merge into `main`.

- [ ] **Step 10.1: Confirm commit structure**

```bash
git log --oneline main..reliability-audit-closure
```

Expected: exactly 6 commits (+ 1 if Task 3 produced a verification-log-update commit, so 6 or 7). Titles must start with `feat: PR1` through `feat: PR5` plus Task 2's `docs: fill reliability-audit closure verification log` (and optionally Task 3's update).

- [ ] **Step 10.2: Final Baseline B re-run on branch tip**

```bash
{ go build ./... && \
  go vet ./... && \
  go test -count=1 ./... && \
  go test -race -count=1 ./... && \
  golangci-lint run; } 2>&1 | tee /tmp/pingcast-final.log
echo "Exit: $?"
```

Expected: `Exit: 0`. Diff against `/tmp/pingcast-baseline-B.log`:

```bash
diff <(grep -E '^(ok|FAIL|---)' /tmp/pingcast-baseline-B.log | sort) \
     <(grep -E '^(ok|FAIL|---)' /tmp/pingcast-final.log | sort)
```

Expected: empty diff (same test packages passing) OR additions-only (new tests that pass).

- [ ] **Step 10.3: Test count check**

```bash
grep -c '^ok' /tmp/pingcast-final.log
grep -c '^ok' /tmp/pingcast-baseline-B.log
```

Expected: final count ≥ baseline-B count.

- [ ] **Step 10.4: Total-diff sanity check**

```bash
git diff --stat main...reliability-audit-closure
```

Expected: roughly 1130+ lines changed (matching the original unstaged diff, plus the Verification Log commits). If dramatically different, something was lost or duplicated.

- [ ] **Step 10.5: Fast-forward merge into main**

```bash
git checkout main
git merge --ff-only reliability-audit-closure
git log --oneline -8
```

Expected: merge succeeds (fast-forward, no merge commit). Log shows the 6+ new commits at `HEAD`.

**Gate:** If `--ff-only` fails (i.e., `main` advanced during the closure work), abort and rebase the feature branch onto the new `main` before retrying. Do NOT force-merge.

- [ ] **Step 10.6: Delete the feature branch**

```bash
git branch -d reliability-audit-closure
```

Expected: branch deleted (succeeds because the branch is fully merged into `main`).

- [ ] **Step 10.7: Keep the rollback tag for 7 days, then clean up**

```bash
git tag --list pre-audit-closure
echo "Reminder: delete 'pre-audit-closure' tag after 2026-04-24 if no rollback needed: git tag -d pre-audit-closure"
```

Expected: tag still present.

- [ ] **Step 10.8: Do NOT push to remote**

Per user-provided guidance, only push when explicitly requested. The user may review locally first. If the user asks to push:

```bash
git push origin main
git push origin --tags
```

---

## Rollback Procedure (execute only on failure)

Triggers:
- Task 3 can't produce a green Baseline B.
- Any `--ff-only` merge in Task 10 reveals regressions after the fact.
- Task 10 total-diff sanity check shows unexplained diff size.

Steps:

```bash
git checkout main
git reset --hard pre-audit-closure
git branch -D reliability-audit-closure
git stash list  # check if the pre-stage-0 stash survived
```

If original unstaged state was lost (no stash): restore from `git reflog`:

```bash
git reflog | head -40
# find the reflog entry BEFORE the first closure action, e.g. "HEAD@{N}: commit: docs: add reliability-audit closure design spec"
# the prior entry's working-tree state can be reconstructed from the object db
```

Root-cause before retrying — do not loop Stage 0 → Stage 4 without understanding what went wrong.

---

## Self-Review Notes (completed at plan-write time)

**Spec coverage:** every section of the closure-design doc has a matching task:
- Out of Scope → Task 2 Step 2.5 classifies, Task 4+ skip `docs/articles/`, Task 9 Step 9.6 verifies only `docs/articles/` remains.
- Current State (snapshot) → Preconditions section.
- Approach → Task 1 (branch + tag), Tasks 4–9 (six commits), Task 10 (ff merge).
- Stage 0 → Task 1.
- Stage 1 → Task 2 (+ conditional Task 3).
- Stage 2/3 → Tasks 4–9 (split + per-commit verification inside each task).
- Stage 4 → Task 10.
- Rollback plan → Rollback Procedure section.
- Success criteria → Task 10 Steps 10.1–10.4.
- Risks (each risk's mitigation mapped to a step):
  - Mid-sequence non-compile → every PR task has `git stash push -u -k` + `go build` gate before commit.
  - Baseline B failures → Task 1.5 gate + Task 3.
  - Silently-wrong implementation → Task 2's per-issue manual audit reads prescription, not tests.
  - Out-of-spec files → Task 2 Step 2.5 classifies each; `docs/articles/` flagged out of scope in Preconditions and Task 9 Step 9.6.
  - Migration conflict → Task 7 runs integration tests (testcontainers).
  - Rollback corrupts tree → Task 1 Steps 1.1 (tag) + 1.2 (stash) + Rollback Procedure reflog fallback.

**Placeholder scan:** no TBDs, no "appropriate error handling", no "similar to", no undefined symbols. Every code block shows the full command.

**Type consistency:** the plan refers to parent-spec names (`port.Cipher`, `NoOpCipher`, `MonitorChangedEvent`, `ErrUserExists`, `httperr.Classify`) consistently across tasks.
