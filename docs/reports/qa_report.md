# QA Report: Security Settings Save, Field-Level Update

- Head: `82c9367c` (6 commits on `development` `890ab782`). During the run the branch was rebased onto the current `development`; the backend tree is identical to the tree tested before the rebase (`2b55b5d5`), and the frontend dependency set changed by two patch-level bumps. Frontend gates, the image rebuild and the targeted E2E specs were re-run on `82c9367c`; the container and integration checks ran on the pre-rebase backend image, which has identical backend code.
- Date: 2026-10-26
- Scope: the security settings save endpoint (request binding, the update service, the save path into the stored row, and the Caddy apply that follows a save), the security settings pages that call it, the related integration scripts, and the docs that describe the save.
- Working tree: nothing committed, pushed or opened as a PR by QA. This report is the only file written by QA.
- **Verdict: READY for merge after F1 was fixed (`make lint-backend` re-run on the touched packages: 0 issues).**

## Gate Summary

| # | Gate | Result | Numbers |
|---|------|--------|---------|
| 1 | `go build ./...` | PASS | clean |
| 2 | `go test -race -count=1`, handlers | PASS | package ok |
| 3 | `go test -race -count=1 -timeout 45m`, services | PASS | package ok |
| 4 | `go test -race -count=1`, models, routes, caddy, server, cmd | PASS | 7 packages ok, 0 failures |
| 5 | Backend coverage (`scripts/go-test-coverage.sh`, gate 87%) | PASS | line 91.7%, statements 92.8% |
| 6 | Local patch coverage (`scripts/local-patch-report.sh`, after frontend coverage) | PASS | backend 131 of 131 changed lines (100%), overall 100%, no uncovered changed line |
| 7 | Frontend coverage (`scripts/frontend-test-coverage.sh`) | PASS (lines), WATCH (branches) | lines 91.98%, statements 90.94%, functions 88.77%, branches 84.68% |
| 8 | Vitest (full) | PASS | 312 files, 3925 passed, 4 skipped, 2 todo, 0 failed |
| 9 | `npm run type-check` | PASS | clean |
| 10 | `npm run lint` | PASS | 0 errors, 1358 warnings (existing backlog, #1743) |
| 11 | `npm run build` | PASS | clean |
| 12 | i18n | PASS (project test), known gap | locale parity test 19 of 19 passed; the wider key gap is pre-existing, see F3 |
| 13 | `make lint-fast` | PASS | 0 issues (backend and agent) |
| 14 | `make lint-backend` | PASS after fix | 2 gosec G101 findings in new test fixtures (F1) were annotated; re-run clean |
| 15 | markdownlint on `docs/api.md`, `docs/cerberus.md` | PASS | 0 issues in 2 files |
| 16 | GORM security scan (`--check`) | PASS | 0 CRITICAL, 0 HIGH (74 Go files) |
| 17 | govulncheck | PASS | 0 vulnerabilities in called code; 1 advisory in a required module that the code does not call (documented in `SECURITY.md`) |
| 18 | CodeQL Go (`lefthook run codeql`) | PASS | 4 results, all suppressed in code this change does not touch; 0 blocking |
| 19 | CodeQL JS | PASS | 0 results |
| 20 | Trivy filesystem, tracked tree | PASS | 0 CRITICAL, 0 HIGH (vulnerability, secret and misconfiguration scanners) |
| 21 | Trivy image, branch image | PASS with a known item | 2 HIGH, both in a bundled third-party component, documented in `SECURITY.md`; same count on the base image |
| 22 | Playwright, project `security-tests`, targeted specs (4 files) | PASS | 32 passed, 0 failed |
| 23 | Coraza WAF integration script (throwaway copy with ports remapped) | PASS | exit 0; block-mode request answered 403, monitor-mode answered 200 |
| 24 | WAF integration script (throwaway copy with ports remapped) | PASS | all checks passed (19 checks) |
| 25 | Rate-limit integration script (throwaway copy with ports remapped) | PASS | all checks passed (15 checks) |
| 26 | Container behaviour checks, branch image, throwaway container | PASS | 51 of 51 checks passed, 0 failed |
| 27 | Changelog and `go.work.sum` invariants | PASS | `changelog.json` is `[]`; `backend/internal/changelog` diff is 0 lines; `go.work.sum` unchanged |
| 28 | Gotify token review | PASS | no token values in the branch diff |

Not run in this session: `lefthook run pre-commit` (the codeql hook set was run). It may rewrite files, so the orchestrator should run it on the final tree.

## Findings

**F1, fixed in this PR.** `make lint-backend` reports two gosec G101 findings on new test fixtures: one in `backend/internal/services/security_config_update_test.go` (fixture constructor) and one in `backend/internal/api/handlers/security_config_save_pins_test.go` (fixture literal). Both are sample values, not credentials. Fix: a `//nolint:gosec // G101` annotation on each fixture, matching the repository convention. Severity: CI-blocking, not a security defect.

**F2, watch.** Frontend branch coverage is 84.68%, below 85%. The gate is on lines (91.96%), which passes. This value is not new in this change. Suggested follow-up: tests for the frontend branches in the settings pages that the save changes touched.

**F3, tracked.** The locale key gap in the translation files is pre-existing, and the project's locale parity test covers only a fixed list of key roots. Issue #1741 tracks three `proxyHosts.source` keys in four locales. The same three keys are also missing in the other locales, and a follow-up comment on #1741 is recommended. Other locale gaps are tracked as part of the translation backlog. Severity: low.

**F4, known.** Trivy image scan reports 2 HIGH findings in a third-party component bundled in the image. Both are documented in `SECURITY.md` with no upstream fix yet. The count is the same on the base image, so this change does not add to it.

**F5, watch, not in this change.** The integration script for the Coraza WAF cannot read the Caddy admin API through a host-mapped port in a throwaway container, so its readiness check logs warnings. The checks it performs still pass. Environment-only; no product change proposed.

## Container Behaviour Checks

Throwaway containers were built from the branch image, given their own volumes and unique host ports, and removed after the run. The live `charon` container, its data, database and volumes were not touched. The `charon-e2e` container was not modified by these checks.

- Partial saves (rate-limit numbers only, allow-list only) leave every other stored field and the status flags unchanged. PASS.
- A full-row save by an existing client stores every field. PASS.
- An explicit empty allow-list clears it; explicit `false` and `0` are applied; `null` and absent keys keep the stored value; an empty body and a CrowdSec-mode-only body change nothing, including the update timestamp. PASS.
- Invalid CIDR and invalid CrowdSec mode answer 400 and write nothing. PASS.
- A non-admin user is refused with 403 and nothing changes. PASS.
- 30 pairs of concurrent saves to different columns (60 requests): all answered 200, both columns kept a value from their own set, no lock or deadlock errors. PASS.
- A save while CrowdSec is running leaves the engine state and the stored CrowdSec mode unchanged. PASS.
- Emergency reset and break-glass disable still work after a partial save; a wrong break-glass token is refused. PASS.
- A WAF mode change reaches the Caddy configuration (block, then monitor). PASS. The "apply error still answers 200" path is covered by a unit test, not by this container run.

## Limits

- The behaviour checks used throwaway containers with Cerberus enabled and the WAF mode set at start, as the WAF integration scripts do.
- Playwright ran only the four targeted specs in one project. The full suite and cross-browser runs are deferred to CI, per the repository rules.
- Race-detector runs cover the touched packages and the packages named in the gate list; concurrency results are pass/fail counts over the runs, not proofs.

## Required before merge

1. ~~Fix F1~~ done.
2. Run `lefthook run pre-commit` on the final tree.
3. Decide on F2 and F4 (F4 is already documented; F2 is a watch item).
