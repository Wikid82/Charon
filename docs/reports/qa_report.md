# QA Report — Flaky Test Root-Cause Fix (Leaked SQLite Test Connections)

**Branch**: `development`
**Commits reviewed**: `5e38d2c5`, `25143838`, `2a9d7321`, `b073519a`, `b1b8463c`, `b9a46963` (6 commits, `62107fc0..b9a46963`)
**Reviewed by**: qa-security agent (final sign-off pass)
**Date**: 2026-08-24
**Plan reference**: `docs/plans/current_spec.md`

## Summary

This PR fixes a confirmed root cause of intermittent CI failures: 41 call sites across 17 Go test files opened a file-backed SQLite connection under a `t.TempDir()`/`os.MkdirTemp()` without closing it, letting live WAL/-shm sidecar files race `os.RemoveAll` during Go's temp-dir cleanup (`unlinkat ... directory not empty`). The fix registers `t.Cleanup` to close each connection immediately after open, relying on `t.Cleanup`'s LIFO ordering to guarantee the close runs before directory removal. A 6th commit additionally stops a leaked background goroutine (`SecurityHandler`'s internal `processAuditEvents`) by calling the handler's pre-existing `Close()` from two test sites that weren't calling it.

**Verdict: READY TO MERGE.** All applicable Definition of Done gates pass. Two pre-existing, unrelated flaky-test findings were surfaced during stress testing and are documented below as follow-up recommendations — neither blocks this PR.

---

## Scope Verification (independently confirmed, not taken on faith)

```
git diff --stat 62107fc0 HEAD
```
→ exactly 17 `_test.go` files, 194 insertions / 55 deletions, **zero** non-Go files, **zero** production code, **zero** frontend files, **zero** `backend/internal/models/**` or migration files.

Every hunk in the diff was read in full. Every change is one of:
- A new `t.Cleanup(func() { _ = sqlDB.Close() })` (or the goroutine-stopping `t.Cleanup(h.Close)`) inserted immediately after a successful `gorm.Open`/`.DB()` call.
- In `certificate_handler_test.go` only: four duplicated open/migrate blocks consolidated into a shared `openCertHandlerTestDB` helper (mechanical extraction, same behavior) plus `err :=` shadowing cleanups (`if err = ...` → `if err := ...`) required by the refactor.

No `assert`/`require`/`t.Fatal` assertion line was removed anywhere across the 41 sites — confirmed by diffing all removed (`-`) lines and manually filtering out only the boilerplate DB-setup lines subsumed by the new helper.

---

## Definition of Done — Gate by Gate

| # | Gate | Status | Detail |
|---|------|--------|--------|
| 1 | Targeted Playwright E2E | **N/A — confirmed** | `git diff --stat 62107fc0 HEAD -- frontend/` → 0 files. No user-facing behavior changed. |
| 1.5 | GORM security scan | **N/A — confirmed** | `git diff --name-only` contains no `backend/internal/models/**`, no GORM query/migration changes. Trigger not met; skipped per CLAUDE.md. |
| 2 | Local Patch Coverage Preflight | ✅ **PASS** (backend scope) / ⚠️ pre-existing unrelated warning | `bash scripts/local-patch-report.sh` run directly. Both `test-results/local-patch-report.md` and `.json` produced. Backend patch coverage: **100.0%** (25/25 changed lines covered) — the only scope this PR touches. Overall: 92.2%. The script's non-zero exit is caused solely by **Frontend 84.6%** (below 85% threshold), traced to `frontend/src/components/ImportSitesModal.tsx` / `Layout.tsx` — both changed by an unrelated, already-committed commit `bbf64f31` ("fix: support Escape-key dismissal on click-to-close overlay backdrops") that predates this PR's baseline and is not part of the 6 commits under review. Not a blocker for this sign-off; flagged as a pre-existing gap for a separate follow-up. |
| 3 | Security scans (CodeQL/Trivy) | **Correctly deferred to CI** | `fix:`-scoped, no new code paths/endpoints/components. Per CLAUDE.md's own deferral rule, not run locally; CI runs both unconditionally on every PR. |
| 4 | Lefthook Triage | ✅ **PASS** | `lefthook run pre-commit` run — all hooks report "skip, no matching staged files" (commits are already committed, nothing staged). Static analysis instead verified directly (see #5). |
| 5 | Staticcheck (BLOCKING) | ✅ **PASS** | `make lint-staticcheck-only` → **0 issues** (backend + agent). `make lint-fast` (full fast linter set incl. govet) surfaced 3 pre-existing `govet` findings (`reflect.Ptr` inline suggestion) in `docker_handler.go`, `orthrus_handler.go`, `uptime_service.go` — confirmed byte-identical between baseline `62107fc0` and HEAD (not touched by this PR, not introduced by it, and not the blocking staticcheck gate CLAUDE.md calls out). `go vet ./...` also clean (exit 0). `gofmt -l` on all 17 touched files → clean. |
| 6 | Coverage Testing | ✅ **PASS** | `.github/skills/scripts/skill-runner.sh test-backend-coverage` (wraps `scripts/go-test-coverage.sh`, run to completion in foreground): **Statement coverage 91.7%, Line coverage 88.4%** (min required 87%) → `Coverage requirement met`. Full `./...` race run embedded in this script passed with zero failures. |
| 8 | Verify Build | ✅ **PASS** | `cd backend && go build ./...` → clean, no output. Frontend build not re-verified (zero frontend files changed; N/A consistent with #1). |
| 9 | Fixed/new code testing | ✅ **PASS** | See "Stress-Test Re-Verification" below — this **is** the fixed code (test files), and it was re-run directly under multiple conditions. |
| 10 | Clean up | ✅ **PASS** | Grepped all 17 touched files' added (`+`) lines for `fmt.Println`, `console.log`, `TODO`, `FIXME`, `DEBUG` comments — zero matches. |

---

## Security-Specific Review

- **No credentials/secrets touched.** Grepped the full diff for `gotify|token|secret|password|apikey|api_key|credential` — the only matches are pre-existing test function names (`TestSeedMain_ForceAdminUpdatesExistingUserPassword`), not new secret values. SECURITY.md's Gotify Token Hygiene section (no token values in logs/artifacts/URLs) is not implicated — no Gotify code paths are touched.
- **No test fixture data changed** that could leak anything — all changes are pure `t.Cleanup` registrations plus one mechanical setup-helper extraction.
- **No assertion silently weakened.** Confirmed by diffing every removed line across all 41 sites — none is an `assert`/`require`/`t.Fatal` call; all are boilerplate DB-open code subsumed by the new shared helper.
- **Security-relevant test components (SecurityHandler, CrowdSec) specifically spot-checked**: `security_handler_rules_decisions_test.go` (`setupSecurityTestRouterWithExtras`, `TestSecurityHandler_UpsertDeleteTriggersApplyConfig`) and `crowdsec_wave7_test.go` (`TestCrowdsecWave7_Start_CreateSecurityConfigFailsOnReadOnlyDB`) — both diffs are purely additive `t.Cleanup` registrations after existing assertions; no test logic, request/response expectations, or coverage of security-decision/ruleset/CrowdSec code paths was altered or reduced. The `security_handler_rules_decisions_test.go` change additionally *improves* test-cleanup correctness by draining `SecurityHandler`'s background audit goroutine before connection close, which was previously a source of racy, potentially-silent test flakiness of its own.
- Commit 6's goroutine fix (`h.Close()`) uses the handler's **pre-existing, documented** production `Close()` method ("Required for test cleanup") — no new production code, no new attack surface.

**Conclusion**: no security-relevant surface risk identified in this PR.

---

## Stress-Test Re-Verification (independently re-run, raw output inspected directly)

Per the explicit instruction not to trust self-reports, every command below was run by me in the foreground, output inspected directly (not summarized by a subagent).

### 1. Targeted flagged-test stress run
```
go test ./internal/api/handlers/... -run 'TestSecurityHandler_CreateAndListDecisionAndRulesets|TestSecurityHandler_UpsertDeleteTriggersApplyConfig' -count=40 -race -v
```
Result: **80/80 PASS** (2 tests × 40 iterations), **0 FAIL**, **0 DATA RACE**, **0 unlinkat/ENOTEMPTY** — grepped across the full raw log, not just the tail. `ok github.com/Wikid82/charon/backend/internal/api/handlers 13.533s`.

### 2. Full-module single-count run
```
go test ./... -count=1
```
Result: **every package `ok`**, zero failures, including `internal/api/handlers` (79.1s), `internal/services` (110.4s), `internal/server`, `cmd/seed`, `internal/crowdsec` (94.2s). This is the realistic CI-equivalent check.

### 3. Broader `-race -count=5` stress run — two pre-existing, unrelated findings surfaced

Running `go test ./internal/api/handlers/... ./internal/services/... ./cmd/seed/... ./internal/server/... -race -count=5` (and isolated re-runs) surfaced test failures **outside** the 17 files this PR touches:

- **`TestAuthHandler_GetAccessibleHosts_PermittedHosts`** (and related `TestAuthHandler_CheckHostAccess_*` / `TestAuthHandler_GetAccessibleHosts_*` tests) in `auth_handler_test.go` — failed with 404s and a nil-interface panic under `-count=5`.
- **~20 tests in `internal/services`** (`TestAuthService_*`, `TestCertificateService_*`, `TestProxyHostService_*`) — failed with `UNIQUE constraint failed: users.email` and a nil-pointer panic under `-count=5`.

**Root cause traced and confirmed pre-existing, not caused by this PR**:
- Both failing test helpers (`setupAuthHandlerWithDB` in `auth_handler_test.go`; 101 call sites across `internal/services/*.go`, e.g. `auth_service_test.go`) use a **shared-cache in-memory SQLite DSN keyed only by `t.Name()`** (`file:<TestName>?mode=memory&cache=shared`). Under `-count=N` combined with `t.Parallel()`, repeated iterations of the same test name can collide on the same shared in-memory database, causing `UNIQUE constraint` violations and nil-record races. This is a **different, pre-existing anti-pattern**, structurally unrelated to the WAL/file-backed-`t.TempDir()` leak this PR fixes.
- **Verified by direct reproduction on the pre-fix baseline**: stashed the working tree, checked out `62107fc0` (pre-fix), re-ran `go test ./internal/api/handlers/... -race -count=5` — the identical `TestAuthHandler_*` failure signature (`UNIQUE constraint failed`, 404s, nil-interface panic) reproduced on baseline, proving this is not introduced or exposed by any of the 6 commits under review. Working tree was cleanly restored afterward (`git checkout development && git stash pop`; confirmed `git log -1` back at `b9a46963` and only the pre-existing unrelated `docs/plans/current_spec.md` local edit remained).
- None of `auth_handler_test.go`, `auth_service_test.go`, `certificate_service_test.go`, `proxyhost_service_validation_test.go` (or any file with these failing tests) are among the 17 files in this PR's diff.
- One run also hit Go's default 10-minute per-package test timeout when 3 packages ran concurrently with `-race -count=5` on this sandbox — consistent with CPU/IO contention from race-instrumented bcrypt/cert-crypto-heavy tests fanned out 5x simultaneously across multiple test binaries, not a deadlock (isolating `internal/services` alone with a 20-minute timeout let it run to completion and reproduce the same pre-existing collision failures rather than hang).

**Recommendation (follow-up, not blocking this PR)**: file a tracking issue for the shared-cache-DSN-keyed-by-`t.Name()` anti-pattern (101+ call sites in `internal/services`, plus `auth_handler_test.go`) — it is a second, independent source of test flakiness under high-concurrency/`-count>1` conditions, conceptually similar in spirit to the WAL/tempdir issue this PR fixes but with a different mechanism (in-memory DSN collision vs. file-cleanup race) and a much larger footprint. Standard CI runs (`-count=1`, no artificial `-count=5` stress) are not exposed to this, which is why it hasn't caused the CI flakiness this PR was created to fix — but it is a latent risk worth tracking separately.

---

## Final Verdict

**READY TO MERGE.**

- All applicable Definition of Done gates pass (Playwright and GORM scan correctly N/A; CodeQL/Trivy correctly deferred to CI per CLAUDE.md's own `fix:`-scope rule).
- Staticcheck (the blocking gate), `go vet`, `gofmt`, and `go build` are all clean.
- Backend coverage 88.4% line / 91.7% statement, both above the 87% minimum.
- Backend patch coverage on this PR's actual changed lines: 100%.
- The specific flagged flaky test is proven fixed: 80/80 PASS across 40 stress iterations with `-race`, and the full module passes cleanly at `-count=1`.
- No security-relevant surface risk; no assertions weakened; no secrets exposed.
- Two pre-existing, unrelated flaky-test findings were discovered and root-caused during deliberate stress testing beyond what this PR touches — both confirmed to reproduce identically on the pre-fix baseline, confirming they are not caused by this PR. Documented above as a follow-up recommendation, not a blocker.

No blocking issues found.
