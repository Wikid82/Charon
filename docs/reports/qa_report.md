# QA Report: CrowdSec Lifecycle Consistency (PR 14)

- Branch: `fix/crowdsec-lifecycle-consistency`, HEAD `33c3e395`, base `development` `a950b3fe`, 18 commits
- Date: 2026-10-26
- Scope: CrowdSec engine lifecycle owner, switch and settings consistency, emergency reset (Tier 1 and Tier 2), proxy-update detachment, the managed whitelist file, preset busy checks, and the related docs and E2E.
- Working tree: nothing committed, pushed, or opened as a PR. This report is the only change (uncommitted).
- **Verdict: NOT READY for merge.** One new blocking CodeQL finding (F1). Every other gate passes, or is a known or tracked item (see Findings and Limits).

## Gate Summary

| # | Gate | Result | Numbers |
|---|------|--------|---------|
| 1 | `go build ./...` | PASS | clean |
| 2 | `go test -race -count=1` on touched packages (crowdsec, handlers, server, routes, caddy, models, testutil, cmd, services) | PASS | 14 packages `ok`, 0 failures |
| 3 | Backend coverage (`scripts/go-test-coverage.sh`, gate 87%) | PASS | line 91.7%, statements 92.8% |
| 4 | Local patch coverage (`scripts/local-patch-report.sh`, regenerated after step 3) | PASS | overall 99.9% (713/714), backend 99.9% (701/702), frontend 100% (12/12) |
| 5 | Frontend coverage (`scripts/frontend-test-coverage.sh`) | PASS (lines), WATCH (branches) | lines 91.96%, statements 90.92%, functions 88.77%, branches 84.65% |
| 6 | Vitest | PASS | 312 files, 3922 passed, 4 skipped, 2 todo, 0 failed |
| 7 | `npm run type-check` | PASS | clean |
| 8 | `npm run build` | PASS | clean |
| 9 | `npm run lint` | PASS | 0 errors, 1358 warnings (same backlog as #1743) |
| 10 | i18n (locale key parity, en vs 27 locales) | PASS for the new keys, pre-existing gaps | the 5 keys added on this branch exist in de, es, fr and zh; the older gaps (3 keys in de/es/fr/zh, 44 keys in the other locales) are present at the base commit and tracked in #1741 |
| 11 | `make lint-fast` | PASS | 0 issues (backend and agent) |
| 12 | `make lint-backend` (golangci-lint, full) | PASS | 0 issues |
| 13 | markdownlint on changed docs | PASS | `ARCHITECTURE.md`, `docs/features/crowdsec.md`, `docs/troubleshooting/crowdsec.md` |
| 14 | GORM security scan (`--check`) | PASS | no CRITICAL or HIGH findings |
| 15 | govulncheck | PASS | 0 vulnerabilities in code paths; 1 in required modules, not called (informational) |
| 16 | CodeQL Go (`lefthook run codeql`) | **FAIL** | 5 results: 4 suppressed, **1 new blocking** (F1) |
| 17 | CodeQL JS | PASS | 0 results |
| 18 | Trivy filesystem, tracked tree (`git archive HEAD`) | PASS | 0 CRITICAL or HIGH |
| 19 | Trivy image, branch image | PASS with a known item | 1 HIGH, CVE-2026-32286 in the bundled CrowdSec and cscli binaries; already documented in `SECURITY.md` (Awaiting Upstream) |
| 20 | Playwright `security-tests` (8 targeted specs, one project) | PASS | 52 passed, 0 failed |
| 21 | Playwright `firefox` (`tests/crowdsec-whitelist.spec.ts`) | PASS | 11 passed, 0 failed |
| 22 | Changelog and `go.work.sum` | PASS | `backend/internal/changelog` diff 0 bytes; `changelog.json` is `[]`; `go.work.sum` unchanged |
| 23 | Gotify token review | PASS | no token values in the branch diff, logs, test artifacts or URLs |
| 24 | Container gate (throwaway containers from the branch image) | PASS, with items not run (see below) | see Container Gate |

## Findings

**F1, blocking (fix in this PR, then re-run `lefthook run codeql`).** New CodeQL finding `go/log-injection` at `backend/internal/api/handlers/security_handler.go:318`, introduced by commit `bb82891c6`. The warning logs the error from a re-read of the saved row, and that error text embeds the request's `name`. The endpoint is admin-only and the name is quoted, so practical impact is low, but the gate blocks the PR. Suggested fix: log a fixed message, or sanitise the error text before logging (`util.SanitizeForLog` already exists in the codebase). Severity: low.

**F2, watch.** Frontend branch coverage is 84.65%, below 85%. The script gate is on lines (91.96%) and passes. The value has been flat since #1717, so this is not new. Recommend tests for the new coded-error branches to lift it.

**F3, known.** CVE-2026-32286 (pgproto3, bundled in the CrowdSec and cscli binaries) appears in the branch image scan. It is already documented in `SECURITY.md` with status Awaiting Upstream. No new exposure from this PR.

**F4, low.** One changed line is uncovered: `backend/internal/api/routes/routes.go:453`, the error return when the sign-in throttle constructor fails. Patch coverage for the file is 95.2%. A unit test that passes invalid trusted proxies would cover it.

**F5, tracked.** Locale gaps (#1741) and the lint warning backlog (#1743) are unchanged by this PR.

No out-of-scope, non-security, non-CrowdSec finding was found, so no issue was filed. Security-sensitive observations from this QA are handled privately per `SECURITY.md` and are not part of this report.

## Container Gate

Throwaway containers were built from the branch image, bound to loopback, used their own volumes, and removed at the end. The live `charon` container, its data, database and volumes were not touched.

- (a) PASS. Enable through `PATCH /security/crowdsec` returns `applied:true`, `lapi_ready:true`, and the engine runs. Two concurrent enables give one 200 and one 409 `crowdsec_busy`, and the engine keeps running. Disable stops the engine. Not run: the decision-ban block check (no proxy host was created).
- (b) PASS. Raw enable and disable endpoints return 200 with `applied`. `POST /settings` `security.crowdsec.enabled`: `true` and `false` drive the engine; empty is a no-op; `garbage` and `yes` return 400 `invalid_crowdsec_setting`. The mode key on `POST` and `PATCH /settings`, and `PATCH /config` (flat and nested), return 400 `crowdsec_setting_use_switch`. Refusals write nothing.
- (c) PASS. Tier 1 and Tier 2 emergency resets return 200 in 13 to 19 ms, and the engine stops after the reply (reply finished about 1 s before the engine count reached zero). Start racing reset, 15 rounds across three timings: the setting never stayed `true` with the engine stopped. A Start sent immediately after a reset runs for 20 s in 5 of 5 rounds. A reset during a LAPI wait answers the Start with 409 `crowdsec_state_changed` and leaves the engine stopped.
- (d) PARTIAL. During a Start that waits for LAPI, raw enable, raw disable and admin stop all return 409 `crowdsec_busy`. An async non-curated preset apply returns 400 `preset_cache_key_required` before the busy check, so the refusal-at-once path was not exercised. A real preset install needs hub access, which the test environment did not have.
- (e) PASS. File-editor save of `charon-whitelist.yaml` returns 400 `file_managed_by_charon`. Hand edits are reverted at the next Start, including the `already_running` path. An import from another instance carries its own file: the import returns `whitelist_regenerated:true` and lists the extra file in `other_whitelist_files`, and Charon's file matches this instance's list.
- (f) PASS. `POST /security/config` keeps the stored CrowdSec mode when the body omits it. An invalid mode returns 400 and the row is unchanged.
- (g) PASS. Boot with the setting on and the security suite off: the engine starts and verifies, one Info line says the proxy leaves CrowdSec out, and the proxy configuration has no CrowdSec entries.
- (h) Observation only, pre-existing behaviour. A whitelist write in the first seconds after a Start has returned can stop the engine. The engine survived writes made 2 s or later after the Start returned, and died in the earlier samples. Recorded for follow-up.
- (i) PASS. A SIGTERM during a Start that waits for LAPI logs the wait timeout after the shutdown line on this branch. The base image does not log it, so the check discriminates.
- (j) PASS. See Gate Summary row 22.

## Limits

- The real preset install (async refusal at once, gate (d)) and the decision-ban block (gate 8(a)) were not run in this environment.
- Playwright ran only the targeted specs listed above. The full suite and cross-browser runs are deferred to CI, per the repository rules.
- The local Trivy filesystem run also reported local, git-ignored or stale paths (generated docs build output, stale worktree copies, a local key file). These are not tracked content and are excluded from the result above.
- Only one CrowdSec and emergency reset pass was run per timing; race results are rounds, not proofs.

## Required before merge

1. Fix F1 and re-run `lefthook run codeql` until the findings gate passes.
2. Decide on F2 (branch coverage) and F4 (one uncovered line); both are small.
3. Re-run the targeted E2E specs only if the F1 fix touches the security handler.
