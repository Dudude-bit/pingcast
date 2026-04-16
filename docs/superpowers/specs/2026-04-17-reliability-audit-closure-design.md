# Reliability Audit Closure — Design Document

**Date:** 2026-04-17
**Parent spec:** `2026-03-23-reliability-audit-design.md`
**Scope:** Safely land the already-written ~1130-line uncommitted reliability-audit changes into `main` with full verification and a clean, bisectable history.

## Summary

The reliability audit (parent spec) identified 29 issues across 4 components and grouped them into 6 PRs. The implementation is already written in the working tree (unstaged, 36 modified files + 6 new files/dirs) but unverified and uncommitted. This document defines the **closure strategy**: verify the diff against the parent spec, split it into the 6 planned commits in the prescribed order, and land it on `main` without regressions.

This is a process design — not a new feature. No functional changes beyond what the parent spec prescribes.

---

## Out of Scope (explicit exclusions)

- `docs/articles/` (untracked directory, unrelated Habr launch article work) — not part of audit, not touched by this workstream.
- Any new feature work.
- Any refactoring or cleanup not prescribed by the parent spec.
- Bug fixes beyond the 29 issues already in the parent spec — those belong to sub-project B (bug triage).
- Frontend / UX changes — sub-projects C and D.

---

## Current State (snapshot 2026-04-17)

**Modified files (36):** see `git status`. Highlights:
- `MM` files (staged + unstaged): `cmd/api/main.go`, `internal/adapter/http/server.go`, `internal/port/channel.go`.
- Staged-only (`M ` in index): `internal/adapter/nats/client.go`.
- Rest: unstaged (` M`).

**New files/dirs (untracked) that ARE part of audit:**
- `internal/adapter/httperr/classify.go` — issue 4.4 (error sanitization) extraction.
- `internal/bootstrap/cipher.go` — issue 4.10 (cipher DI wiring).
- `internal/database/migrations/016_incident_uniqueness.sql` — issue 4.6.
- `internal/port/crypto.go` — issue 4.10 (Cipher port interface).
- `internal/sqlc/internal/` (likely generated mocks) — to be classified in Stage 1.
- `internal/xcontext/detached.go` — shared utility from parent spec §Shared Utility.

**New files/dirs that are NOT part of audit:**
- `docs/articles/` — Habr launch article, excluded per Out of Scope.

---

## Approach

Feature-branch + 6 commits in the order prescribed by the parent spec's §Implementation Order. No GitHub PRs (solo repo — `main` receives direct commits per recent git history). Merge to `main` is a fast-forward after all verification passes.

### Why verify-first (rejected alternatives)

- **One giant commit:** destroys `git bisect`, unreviewable, loses the explicit PR boundaries the parent spec justified.
- **Split-then-verify:** risk of mid-sequence broken commit that doesn't build, because files are touched by multiple PRs.
- **Verify-first, split by spec boundaries:** picked. Matches parent spec's own ordering and risk analysis.

---

## Stages

### Stage 0 — Snapshot & dual baseline

1. Create git tag `pre-audit-closure` at current `HEAD` for rollback.
2. Create branch `reliability-audit-closure` from `main`.
3. `git stash` the working tree → verify stash applies cleanly on unstash.
4. **Baseline A (shipped code):** on `main` HEAD with clean working tree, run and capture output of:
   - `go build ./...`
   - `go vet ./...`
   - `go test -count=1 ./...`
   - `go test -race ./...`
   - `golangci-lint run`
5. **Baseline B (target code):** unstash → re-run all 5 commands. Capture output.
6. **Gate:** Baseline A must be fully green. Baseline B can reveal new failures — these are the work of Stage 1 to diagnose (may be bugs in the audit implementation, may be new tests with bugs, may be pre-existing). Do not proceed to Stage 2 until Baseline B is also green.

### Stage 1 — Audit diff vs parent spec (verify-first)

**Artifact:** append a table to this document (see §Verification Log below) with:

| Issue | Status | Files Touched | Notes |
|---|---|---|---|
| 1.1 | ✅/⚠/❌ | file:line | deviation or TODO |
| ... | ... | ... | ... |

Status legend:
- ✅ implemented correctly per parent spec
- ⚠ implemented with deviation (document + decide: accept or fix)
- ❌ not implemented or incorrect (fix before Stage 2)

**Process:**
1. For each of the 29 issues in the parent spec: read the prescribed files, confirm the change matches the spec's prescription.
2. Classify extra files (not in parent spec):
   - `bootstrap/cipher.go` → likely PR5 (encryption).
   - `sqlc/internal/mocks/` → classify based on what it contains.
   - `httperr/classify.go` → PR4b (4.4 error sanitization extraction).
   - `xcontext/detached.go` → PR1 (shared utility, first consumer).
3. Resolve all ❌ entries (fix in working tree) before moving on.
4. Accept or fix ⚠ entries; document decisions in the table.

### Stage 2 — Split by PR boundaries

Planned commit order (from parent spec §Implementation Order):

1. **PR1** — NATS + event DTO (issues 1.1–1.6) + `xcontext/detached.go`
2. **PR2** — Notifier (2.1–2.9)
3. **PR3** — Checker (3.1–3.4)
4. **PR4a** — API race conditions (4.1, 4.3, 4.6) + migration 016
5. **PR4b** — API defensive (4.2, 4.4, 4.5, 4.7, 4.8) + `httperr/classify.go`
6. **PR5** — Encryption overhaul (4.10) + `port/crypto.go` + `bootstrap/cipher.go`

**File-level tangling — known overlaps:**
- `internal/adapter/nats/subscriber.go` — issues 1.2, 1.3, 2.7 (spans PR1 and PR2)
- `internal/app/alert.go` — issues 2.2, 2.3, 2.8 (all in PR2)
- `internal/adapter/http/server.go` — issues 1.6, 4.2, 4.4, 4.7 (spans PR1, PR4b)
- `internal/app/monitoring.go` — issues 1.1, 1.5, 1.6, 4.1, 4.3 (spans PR1, PR4a)
- `internal/adapter/postgres/monitor_repo.go` — issues 4.1, 4.3, 4.10 (spans PR4a, PR5)

**Splitting strategy:**
1. Prefer `git add -p` to select only the hunks for the current PR.
2. If hunks are genuinely inseparable (e.g., a single function body changed for two different issues in different PRs), **merge the affected PRs** rather than force-split. Record the merge in §Verification Log with justification.
3. Before each `git commit`, run `go build ./...` on the staged subset — if it doesn't compile, the split is incomplete.

**Handling `MM` files:** for each, run `git diff --cached <file>` to see the staged portion. Decide whether it belongs to this audit; if orphan (not audit-related), unstage with `git reset HEAD <file>` and treat the unstaged part as usual.

### Stage 3 — Per-commit verification

After each commit:
- `go build ./...`
- `go vet ./...`
- `go test -count=1 ./...`
- `go test -race ./...` (can be slower — run at least once per commit)
- `golangci-lint run`

If red:
- Fix in-place with `git commit --amend` (the commit is on a local feature branch, not yet in `main`, so amending is safe and expected).
- If the fix requires hunks that belong in a later PR, reconsider the split (may be another tangling case).

### Stage 4 — Final integration

1. After all 6 commits: re-run the full Baseline B command set on `HEAD` of the feature branch. Must match the green state from Stage 0 Baseline B.
2. Check test count: `go test -count=1 ./... | grep -c "^ok"` should be >= baseline (audit adds tests, does not remove them).
3. Check lines-changed: `git diff main...reliability-audit-closure --stat` should roughly match the initial 1130-line diff (minus any deletions of dead code prescribed by the audit).
4. Fast-forward merge: `git checkout main && git merge --ff-only reliability-audit-closure`.
5. Delete feature branch.
6. Keep the `pre-audit-closure` tag for one week for safety, then delete.

### Rollback Plan

Trigger: Stage 3 or 4 discovers a regression that cannot be fixed in-place.

1. `git checkout main` (or stay on the feature branch if in Stage 3).
2. `git reset --hard pre-audit-closure`.
3. `git stash pop` to restore the original unstaged working tree.
4. Diagnose the root cause. Fix in working tree.
5. Restart from Stage 0.

---

## Success Criteria

- All 6 commits on `main`, each green across build/vet/test/race/lint.
- `git log --oneline main` shows exactly 6 new commits with PR-aligned titles matching the parent spec's §Implementation Order.
- `go test -count=1 ./...`: zero failures, test count ≥ pre-closure baseline.
- Verification Log table below is fully filled in, all statuses are ✅ or documented ⚠.
- This design document is committed and the closure work is fully traceable from spec → commits → final state.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Mid-sequence commit doesn't compile due to cross-file coupling | Medium | Stage 2's "build-staged-subset" check; fallback to PR-merge. |
| Baseline B reveals failing tests in the audit itself | Medium | Diagnosed in Stage 1; fix before Stage 2. |
| Issue in parent spec implemented incorrectly, tests still green | Low–Medium | Stage 1 manual audit reads code against spec prescription, not against tests. |
| Out-of-spec files belong to unrelated workstream | Low | Stage 1 step 2 classifies every new file. `docs/articles/` explicitly excluded. |
| Migration 016 conflicts with existing data | Low | Parent spec 4.6 documents pre-migration duplicate check. Tested via testcontainers integration tests in PR4a verification. |
| Rollback corrupts working tree | Very low | `git tag pre-audit-closure` + `git stash` give two independent rollback points. |

---

## Verification Log

*(Populated during Stage 1. Empty at spec-commit time.)*

### Issues table

*(Stage 1 fills this in.)*

### Extra-files classification

*(Stage 1 fills this in.)*

### Deviation decisions

*(Stage 1 fills this in, if any ⚠ entries.)*

### PR-merges (if tangling forced any)

*(Stage 2 fills this in, if applicable.)*
