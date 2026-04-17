# B3 — Lint Hygiene Pass — Design & Plan

**Date:** 2026-04-17
**Parent:** Sub-project B — `B3` slice
**Scope:** Drive `golangci-lint run` to **0 findings**. Every current warning
gets a human decision: real fix, explicit `_ =` discard, or `//nolint` with
a justification comment. Blanket config-level exclusions are used ONLY for
boilerplate-OK patterns in test fixtures.

## Baseline (2026-04-17, after A/B1/B2)

67 findings across 5 classes:
- 32 errcheck
- 19 govet (shadow)
- 10 gosec (8× G115 int→int32, 1× G402 in new test helper, 1× other)
- 5 staticcheck SA4006 (Go 1.26 `new(var)` false-positive from B1 Task 8)
- 1 unusedfunc (stale const)

## Principles

1. **Each warning gets a decision.** No blanket "silence everything".
2. **Real bugs get fixed** (e.g., tx.Rollback silently ignoring errors inside a real error-recovery path).
3. **Intentional discards get `_ = x`** with a short comment if non-obvious.
4. **False positives get `//nolint:<linter> // <reason>`** — inline, not config.
5. **Config-level exclusions** used ONLY for `_test.go` fixture noise (mock HTTP server `w.Write`, `json.Decoder.Decode` in fake handlers) — these aren't production code and suppressing per-call would be line noise.

## Decisions by class

### SA4006 (5 sites, server.go 514/548/561/580/593)

**Root cause:** Go 1.26 added `new(v T) *T` taking a value. `new(msg)` *does* read `msg`, but staticcheck hasn't caught up and falsely reports SA4006. Also the `apigen.ErrorResponse.Error` field is `*string`.

**Fix:** rewrite `new(msg)` → `&msg` at all 5 sites. Semantically identical, no lint warning.

### unusedfunc (checker/http.go — `defaultTimeout` const)

**Fix:** delete. The checker now uses `clientByTTL` cache + `c.httpClient.Timeout` as the default; `defaultTimeout` hasn't been referenced since the B2 cache refactor.

### govet shadow (19 sites)

**Fix:** rename inner `err` to a context-specific name (`parseErr`, `rollbackErr`, `listErr`, …). Preserves the outer variable, makes intent explicit, kills the warning.

### gosec G115 (8 sites)

All 8 are `int → int32` conversions on values that are domain-bounded:
- `internal/adapter/postgres/mapper.go` (6): `IntervalSeconds`, `AlertAfterFailures`, `ResponseTimeMs` — bounds enforced by `parseIntInRange` (B2) and check-result invariants.
- `internal/adapter/postgres/incident_repo.go:65`: list `limit` arg passed from app layer, always small.
- `cmd/notifier/main.go:48`, `cmd/worker/main.go:48`, `cmd/scheduler/main.go:50`: `MaxDBConns` from config, envDefault=5-15.

**Fix:** `//nolint:gosec // G115: bounded by <source>` inline on each of the 8 lines. Document the bound.

### errcheck (32 sites — per-site classification)

Expected split:
- **~15 intentional discards** (`defer resp.Body.Close()`, `defer rdb.Close()`, `defer nc.Drain()`, `tx.Rollback()` in cleanup paths) → `_ = x.Close()` pattern.
- **~5 real bugs** → add `slog.Warn`.
- **~12 test-fixture writes** → config exclude on `_test.go`.

**Config addition** (`.golangci.yml`):

```yaml
issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
      text: "Error return value of .* is not checked"
```

## Files touched (rough)

Most of `cmd/` + `internal/adapter/{http,nats,postgres,checker,channel,telegram,webhook,smtp}/` + `internal/database/`. ~20 files, ~60 targeted edits.

## Commit structure

Single commit `chore: B3 — drive golangci-lint to zero findings`. Detailed
per-class summary in commit body. Diff is wide but shallow; bisect within
would not add value since every change is independent.

## Success criteria

1. `golangci-lint run` prints zero findings.
2. `go build/vet/test/test -race` remains green.
3. Commit message enumerates each class's treatment.

## Out of scope

- Systematic bug hunt beyond lint (→ B4).
- Refactoring file layouts or renames unrelated to lint warnings.

---

## Plan (single execution pass — no formal task decomposition)

Given the per-site nature, this executes as ONE task with per-class phases:

1. **Phase 1 — Config rule:** add `_test.go` errcheck exclude to `.golangci.yml`; re-run lint; confirm test-file errcheck warnings gone.
2. **Phase 2 — Cheap fixes:** SA4006 × 5, unusedfunc × 1. Verify lint count.
3. **Phase 3 — Shadow renames:** 19 edits across ~8 files. Verify lint count.
4. **Phase 4 — G115 nolint:** 8 inline nolint with bound-source comments. Verify lint count.
5. **Phase 5 — errcheck per-site:** classify each → `_ =` or `slog.Warn`. Verify lint count → 0.
6. **Phase 6 — Final gate:** build/vet/test/race/lint all green. Commit. FF-merge main.
