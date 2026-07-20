# QA Report — Orthrus External Docker Proxy Hotfix

**Date:** 2026-07-18
**Branch:** `development`
**Base commit:** `0d81fc9e` (fix(deps): pin gosu's golang.org/x/sys to v0.46.0)
**Head commit:** `1caa4c65`
**Scope:** Backend-only hotfix — three verified, independent gaps in the Orthrus External Docker Proxy subsystem. Zero frontend files touched.

**Commits (5, in order):**
1. `98a68b67` fix(orthrus): allow read-only image/distribution inspect through Docker proxy muzzle
2. `7f307c3c` fix(orthrus): resolve external proxy hostname from request context instead of hardcoding it
3. `1eb266d2` fix(orthrus): remove hardcoded "charon" hostname from ExternalProxyStatus
4. `c778b53a` docs(orthrus): document the External Docker Proxy feature
5. `1caa4c65` docs(plans): record Orthrus external proxy hotfix spec

**Changed files:**
- `backend/internal/orthrus/muzzle.go`, `backend/internal/orthrus/muzzle_test.go`
- `backend/internal/orthrus/session.go`
- `backend/internal/api/handlers/orthrus_handler.go`, `backend/internal/api/handlers/orthrus_handler_test.go`
- `docs/features/orthrus.md`, `docs/guides/remote-docker-setup.md`
- `docs/plans/current_spec.md`

---

## DoD Gate Results Summary

| # | Gate | Status | Notes |
|---|---|---|---|
| 1 | Backend build (`go build ./...`) | PASS | Clean build, zero errors |
| 2 | Backend full test suite (`go test ./...`) | PASS | All packages `ok` |
| 2b | Targeted tests (muzzle, orthrus_handler) | PASS | All new/updated test cases pass |
| 3 | `make lint-fast` | PASS | 0 issues |
| 3b | `make lint-staticcheck-only` | ENV-BROKEN (pre-existing) | golangci-lint v2 vs `--disable-all` flag incompatibility; not introduced by this hotfix |
| 4 | `lefthook run pre-commit` (scoped to changed files) | PASS | go-vet, golangci-lint-fast (0 issues), semgrep (0 findings) all green |
| 5 | GORM security scan | N/A — confirmed not triggered | Zero diff under `backend/internal/models/**`, no GORM query changes |
| 6 | `bash scripts/local-patch-report.sh` | PASS | 100% overall/backend patch coverage (target 90%) |
| 7 | `scripts/go-test-coverage.sh` (backend ≥85%) | PASS | 88.9% statement coverage |
| 7b | Frontend coverage (`test:coverage`) | PASS | 88.86% statements / 90% lines (informational — zero frontend files in this patch) |
| 8 | Playwright E2E — Orthrus/uptime specs | PARTIAL PASS (2 pre-existing, unrelated failures) | See detail below |
| 9 | Muzzle allowlist security verification | PASS | Additive-only, GET-only, no widening |
| 10 | `docs/features/orthrus.md` read-only promise | PASS | Textually and functionally intact |
| 11 | `tcp://charon` literal grep | PASS | Zero matches in `backend/` |
| — | Frontend diff check | PASS | `git diff --stat -- frontend/` empty |

---

## Detail

### 1–2. Backend Build & Tests

```
cd backend && go build ./...        → clean, no output
go test ./...                       → ok for every package
go test ./internal/orthrus/... -run TestMuzzle -v                    → all PASS
go test ./internal/api/handlers/... -run 'ProxyStatus|ExternalProxy' -v → all PASS
```

New/updated tests all pass, including:
- `TestMuzzle_DynamicPaths_Passthrough` (extended with `/images/*/json`, `/distribution/*/json` cases)
- `TestMuzzle_UnknownPath_Blocked` (extended with the documented multi-segment-name limitation case)
- `TestMuzzle_ImageAndDistributionEndpoints_POSTBlocked` (new — regression guard)
- `TestOrthrusHandler_GetProxyStatus_Connected` (updated assertion for handler-resolved hostname)
- `TestOrthrusHandler_GetProxyStatus_ConnectionString_UsesXCharonURLHeader` (new)
- `TestOrthrusHandler_GetProxyStatus_ConnectionString_HostPortStripped` (new)
- `TestOrthrusHandler_GetProxyStatus_ConnectionString_EmptyWhenInactive` (new)

Per-function coverage on all new/changed code is 100%: `resolveExternalProxyHost`, `GetProxyStatus`, `Muzzle.ServeHTTP`, `GetExternalProxyStatus`.

### 3. Lint / Staticcheck

`make lint-fast` (golangci-lint with staticcheck, govet, errcheck, ineffassign, unused): **0 issues**.

`make lint-staticcheck-only` fails with `Error: unknown flag: --disable-all` — this is a golangci-lint **v2.11.4** incompatibility with a v1-only CLI flag baked into the Makefile target. Verified this is pre-existing and unrelated to this hotfix: `git diff 0d81fc9e..HEAD -- Makefile` is empty (Makefile untouched by this PR). `lint-fast` is authoritative for staticcheck coverage per the task brief and reports clean.

### 4. Lefthook

A full unscoped `lefthook run pre-commit` run pulls in the entire frontend lint/CodeQL suite (unrelated to this backend-only diff) and exceeds 2 minutes, confirming the prior agent's note. Scoped the run to exactly the 8 changed files using `lefthook run pre-commit --file <path>...` (glob-based job skip logic preserved, unlike `--files-from-stdin --force` which bypasses glob skipping and force-runs irrelevant jobs like `shellcheck`/`check-version-match` against files that don't match their globs).

Result (14.6s):
```
✔️ block-codeql-db / block-data-backups / check-lfs-large-files / trailing-whitespace / end-of-file-fixer
✔️ go-vet
✔️ golangci-lint-fast   → 0 issues
✔️ semgrep              → 120 rules run on 5 Go files, 0 findings (secrets scan included)
```
All frontend/actionlint/shellcheck/check-version-match jobs correctly skip ("no matching staged files") since none of the changed files match their globs.

### 5. GORM Security Scan

Not applicable. Confirmed via `git diff 0d81fc9e..HEAD --stat -- backend/internal/models/` (empty) and a targeted grep of the three touched Go files for GORM query patterns (only a pre-existing, unmodified `gorm.ErrRecordNotFound` check in `Patch`, outside the diff). Per CLAUDE.md's conditional gate, this scan is correctly skipped rather than run blindly.

### 6. Local Patch Coverage

```
bash scripts/local-patch-report.sh
→ Local patch report generated (mode=warn)
→ test-results/local-patch-report.json, test-results/local-patch-report.md
```

| Scope | Changed Lines | Covered Lines | Patch Coverage | Status |
|---|---:|---:|---:|---|
| Overall | 17 | 17 | 100.0% | pass |
| Backend | 17 | 17 | 100.0% | pass |
| Frontend | 0 | 0 | 100.0% (vacuous) | pass |

Well above the 90% informational target in `codecov.yml`. Baseline: `origin/main...HEAD`.

Note: frontend coverage (`frontend/coverage/lcov.info`) had to be generated solely to satisfy this script's hard precondition that both backend and frontend coverage inputs exist — it contributes 0 changed lines to the patch calculation since this hotfix touches no frontend files. Frontend suite ran clean: 88.86% statements / 90% lines overall (informational, not part of this hotfix's correctness signal).

### 7. Coverage Gates

Backend (`scripts/go-test-coverage.sh`): **88.9%** statement / **89.0%** line coverage, gate is 87% → **PASS**.
Frontend (`scripts/frontend-test-coverage.sh` equivalent): **88.86%** statements / **90%** lines → PASS (informational for this backend-only PR).

### 8. Playwright E2E

Ran the 5 named specs with `npx playwright test --project=firefox`, against the already-healthy `charon-e2e` container (no rebuild needed — test-only/backend-only change, container was already healthy).

| Spec | Result | Notes |
|---|---|---|
| `orthrus-agents.spec.ts` | 24/24 PASS | |
| `orthrus-external-proxy.spec.ts` | 8/8 PASS | Includes `connection_string` display/format assertions — contract unchanged, confirms Gap 3 fix is transparent to the frontend |
| `orthrus-proxy-paths.spec.ts` | 9/9 PASS | |
| `uptime-orthrus.spec.ts` | 3/4 PASS | 1 failure — **pre-existing, unrelated** (see below) |
| `orthrus-agent-install.spec.ts` | **ALL FAILING** | **Pre-existing, unrelated** (see below) |

**Both failures are confirmed unrelated to this hotfix.** `git diff 0d81fc9e..HEAD --stat -- frontend/ tests/` is empty — this PR touches zero files under `frontend/` or `tests/`, so neither failure can be a regression introduced by these five commits.

**Finding A (non-blocking, pre-existing) — `orthrus-agent-install.spec.ts`, all 18 tests fail:**
Every test in this file routes through a shared `openOrthrusWizard()` helper that does `page.locator('#connection-type').selectOption('orthrus')`. That element no longer exists: the actual UI on `/remote-servers` now renders a "Connection mode" **radio button group** (Direct / Agent / Provider), not a `<select id="connection-type">`. Root-caused via `git log -S "connection-type" -- frontend/`: commit `4e9c5a9e` ("fix(hecate): fix stale E2E selectors...", 2026-05-05) performed exactly this `<select>`→radio-group migration and updated the *other* affected spec (`tests/hecate-tunnel-manager.spec.ts`) to use `getByRole('radio')`, but never touched `tests/orthrus-agent-install.spec.ts`. `4e9c5a9e` is already an ancestor of this hotfix's base commit `0d81fc9e` (confirmed via `git merge-base --is-ancestor`), so this breakage predates this hotfix by roughly 2.5 months. Each test times out at 90s on the stale locator; with the file's 18 tests split across 2 workers this takes ~15 minutes to exhaust, so the full run was not driven to completion, but the failure signature is 100% consistent across every test-results artifact produced and matches the root cause exactly.
**Recommendation:** file a follow-up issue to update `tests/orthrus-agent-install.spec.ts`'s `openOrthrusWizard()` helper to use the radio-button locator, matching the pattern already applied to `hecate-tunnel-manager.spec.ts` in `4e9c5a9e`. Not a code defect — a stale test fixture.

**Finding B (non-blocking, pre-existing) — `uptime-orthrus.spec.ts:160`, "non-Orthrus monitor at same IP is checked independently":**
Fails deterministically (reproduced twice). The test mocks `**/api/v1/uptime/monitors` via `page.route()` to return exactly 2 monitors (one Orthrus, one TCP) and asserts card order. The first `monitor-card` observed instead reads `"Dockhand Service...100.99.23.57:3001...TCP...Last Check: over 1 year ago"` — real, persisted data from the long-lived `charon-e2e` container's database (the container has been up 24h+ and accumulates cross-run state), not the mocked fixture. This points to either a route-mock race (an unmocked initial fetch or background refetch winning) or genuine test-data leakage in the shared E2E environment — not a rendering-order bug introduced by this hotfix, which touches zero uptime/monitor code (backend or frontend). `cards.toHaveCount(2)` passes in an earlier step, which is consistent with a subsequent refetch replacing the mocked data.
**Recommendation:** follow-up to either harden the route mock (assert `route.fulfilled` before proceeding, or force a fresh `charon-e2e` container between runs) or purge stale uptime-monitor fixtures from the shared E2E environment.

**Conclusion for gate 8:** the plan's Section 8 assumption that all 5 specs "pass unmodified" does not hold for 2 of the 5 files, but both are demonstrably pre-existing environment/test-fixture drift, unconnected to this hotfix's actual code changes. The specs that directly exercise this hotfix's contract — `orthrus-external-proxy.spec.ts` (connection_string shape/display) and `orthrus-proxy-paths.spec.ts`/`orthrus-agents.spec.ts` (unaffected surface) — all pass cleanly.

### 9. Muzzle Allowlist Security Verification

Diff of `backend/internal/orthrus/muzzle.go` is purely additive: two new entries appended to `allowedDockerPatterns` (`/images/*/json`, `/distribution/*/json`) plus an explanatory doc comment. Verified:
- Both new endpoints are Docker Engine API `GET`-only inspect endpoints (Image Inspect, Distribution/registry digest inspect) with no documented write side effects.
- `Muzzle.ServeHTTP`'s unconditional method check (reject non-GET before any path match) is byte-for-byte unchanged — confirmed via full-file read and diff.
- `TestMuzzle_ImageAndDistributionEndpoints_POSTBlocked` (new) and pre-existing `TestMuzzle_POST_Blocked` (which explicitly cases `POST /images/create`) both pass, proving the write endpoint (`images/create`, image pull) was **not** accidentally exposed and the new read patterns remain GET-only.
- No changes to `allowedDockerPaths`, `versionPrefixRe`, `sanitizePath`, or the traversal-hardening `path.Clean` call.
- The documented `path.Match` single-segment-`*` limitation (namespaced image names like `bitnami/nginx:latest` still 403) is accurately disclosed in both the code comment and `muzzle_test.go`'s `TestMuzzle_UnknownPath_Blocked` case — a known, accepted, non-security-relevant gap (it only makes the allowlist *more* restrictive than intended, never less).

**Verdict: no widening beyond the two intended, verified-read-only patterns.**

### 10. `docs/features/orthrus.md` Read-Only Promise

The existing sentence "This restriction is enforced at every single request — there is no way to turn it off" (line ~106) is untouched. The new "External Docker Proxy (Advanced)" section explicitly reiterates "Still strictly read-only... there is no way to turn this restriction off" and correctly describes the registry-digest-check caveat (outbound network call to the registry is expected, but it cannot mutate the Docker host). Textually and functionally consistent — the promise still holds because `Muzzle.ServeHTTP`'s enforcement logic is unchanged.

### 11. Hostname Regression Guard

```
grep -rn 'tcp://charon' backend/ --include="*.go"   → zero matches
```
`session.go`'s `ExternalProxyStatus.ConnectionString` field and its hardcoded `fmt.Sprintf("tcp://charon:%d", ...)` construction were fully removed; `orthrus_handler.go`'s `GetProxyStatus` now builds `connection_string` from request context via the new `resolveExternalProxyHost` helper, preserving the `active && activePort > 0` guard (previously in `session.go`, now correctly relocated to the handler). Repo-wide grep for any remaining production reference to the removed `ConnectionString` struct field also returned zero matches (only test *function names* containing the substring "ConnectionString" remain, which is expected and correct).

### Frontend Diff Confirmation

`git diff 0d81fc9e..HEAD --stat -- frontend/` → empty. `git diff 0d81fc9e..HEAD --stat -- backend/go.mod backend/go.sum Dockerfile` → empty (no new dependencies, no Dockerfile changes) — satisfies the "Trivy: no new findings" expectation without needing a full container rescan, since no new packages were introduced and the base image/Dockerfile are untouched. Frontend type-check is correctly N/A.

---

## Findings Summary

| ID | Severity | Category | Blocking? | Description |
|---|---|---|---|---|
| DEF-001 | Low | Environment | No | `make lint-staticcheck-only` broken by golangci-lint v2 vs `--disable-all` flag; pre-existing, `lint-fast` is authoritative |
| DEF-002 | Medium | Pre-existing test drift | No — follow-up | `tests/orthrus-agent-install.spec.ts` uses a stale `#connection-type` selector removed by an unrelated May 2026 UI refactor; all 18 tests fail. Not caused by this hotfix. |
| DEF-003 | Low | Pre-existing E2E environment | No — follow-up | `tests/uptime-orthrus.spec.ts` "non-Orthrus monitor..." test fails deterministically due to stale/leaked real data in the long-lived `charon-e2e` container outracing a route mock. Not caused by this hotfix. |

No CRITICAL or HIGH severity findings. No security regressions identified in the Muzzle allowlist expansion.

---

## Final Verdict

**READY FOR PR.**

All gates that this hotfix's own code changes can affect — backend build, backend/targeted tests, lint-fast, scoped lefthook (govet/golangci-lint/semgrep), GORM applicability check, patch coverage (100%), overall backend coverage (88.9%), the Muzzle security review, the hostname regression guard, and the two E2E specs that actually exercise this hotfix's contract (`orthrus-external-proxy.spec.ts`, `orthrus-proxy-paths.spec.ts`) plus `orthrus-agents.spec.ts` — pass cleanly.

The two E2E failures (DEF-002, DEF-003) and the staticcheck tooling gap (DEF-001) are all independently confirmed, via git history and diff scope, to predate this hotfix and to be unrelated to the muzzle/session/handler changes it makes. None block this PR; DEF-002 and DEF-003 are recommended as tracked follow-up issues.
