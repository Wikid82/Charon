## QA/Security Audit — PR-1 Backend Slice (Notify HTTP Wrapper)

- Date: 2026-02-23
- Scope: Current PR-1 backend slice implementation (notification provider handler/service, wrapper path, security gating)
- Verdict: **READY (PASS WITH NON-BLOCKING WARNINGS)**

## Commands Run

1. `git rev-parse --abbrev-ref HEAD && git rev-parse --abbrev-ref --symbolic-full-name @{u} && git diff --name-only origin/main...HEAD`
2. `./.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
3. `PLAYWRIGHT_BASE_URL=http://localhost:8080 npx playwright test tests/settings/notifications.spec.ts`
4. `bash scripts/local-patch-report.sh`
5. `bash scripts/go-test-coverage.sh`
6. `pre-commit run --all-files`
7. `./.github/skills/scripts/skill-runner.sh security-scan-trivy`
8. `./.github/skills/scripts/skill-runner.sh security-scan-docker-image`
9. `bash scripts/pre-commit-hooks/codeql-go-scan.sh`
10. `bash scripts/pre-commit-hooks/codeql-js-scan.sh`
11. `bash scripts/pre-commit-hooks/codeql-check-findings.sh`
12. `./scripts/scan-gorm-security.sh --check`

## Gate Results

| Gate | Status | Evidence |
| --- | --- | --- |
| 1) Playwright E2E first | PASS | Notifications feature suite passed: **79/79** on local E2E environment. |
| 2) Local patch coverage preflight | PASS (WARN) | Artifacts generated: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`; mode=`warn` due missing `frontend/coverage/lcov.info`. |
| 3) Backend coverage + threshold | PASS | `scripts/go-test-coverage.sh` reported **87.7% line** / **87.4% statement**; threshold 85% met. |
| 4) `pre-commit --all-files` | PASS | All configured hooks passed. |
| 5a) Trivy filesystem scan | PASS | No CRITICAL/HIGH/MEDIUM findings reported by skill at configured scanners/severities. |
| 5b) Docker image security scan | PASS | No CRITICAL/HIGH; Grype summary from `grype-results.json`: **Medium=10, Low=4**. |
| 5c) CodeQL Go + JS CI-aligned + findings check | PASS | Go and JS scans completed; findings check reported no security issues in both languages. |
| 6) GORM scanner (`--check`) | PASS | 0 CRITICAL/HIGH/MEDIUM; 2 INFO suggestions only. |

## Blockers / Notes

- **No merge-blocking security or QA failures** were found for this PR-1 backend slice.
- Non-blocking operational notes:
	- E2E initially failed until stale conflicting container was removed and E2E environment was rebuilt.
	- `scripts/local-patch-report.sh` completed artifact generation in warning mode because frontend coverage input was absent.
	- `pre-commit run codeql-check-findings --all-files` hook id was not registered in this local setup; direct script execution (`scripts/pre-commit-hooks/codeql-check-findings.sh`) passed.

## Recommendation

- **Proceed to PR-2**.
- Carry forward two non-blocking follow-ups:
	1. Ensure frontend coverage artifact generation before local patch preflight to eliminate warning mode.
	2. Optionally align local pre-commit hook IDs with documented CodeQL findings check command.

## QA Report — PR-2 Security Patch Posture Audit

- Date: 2026-02-23
- Scope: PR-2 only (security patch posture, admin API hardening, rollback viability)
- Verdict: **READY (PASS)**

## Gate Summary

| Gate | Status | Evidence |
| --- | --- | --- |
| Targeted E2E for PR-2 | PASS | Security settings test for Caddy Admin API URL passed (2/2). |
| Local patch preflight artifacts | PASS | `test-results/local-patch-report.md` and `.json` regenerated. |
| Coverage and type-check | PASS | Backend coverage 87.7% line / 87.4% statement; frontend type-check passed; frontend coverage preflight input passed (88.99% lines). |
| Pre-commit gate | PASS | `pre-commit run --all-files` passed after resolving version and type-check hook issues. |
| Security scans | PASS | CodeQL Go/JS CI-aligned scans passed; findings gate passed with no HIGH/CRITICAL; Trivy passed at configured severities. |
| Runtime posture + rollback | PASS | Default scenario shifted `A -> B` for PR-2 posture; rollback remains explicit via `CADDY_PATCH_SCENARIO=A`; admin API URL now validated and normalized at config load. |

## Resolved Items

1. `check-version-match` mismatch fixed by syncing `.version` to `v0.19.1`.
2. `frontend-type-check` hook stabilized to `npx tsc --noEmit` for deterministic pre-commit behavior.

## PR-2 Closure Statement

All PR-2 QA/security gates required for merge are passing. No PR-3 scope is included in this report.

---

## QA Report — PR-3 Keepalive Controls Closure

- Date: 2026-02-23
- Scope: PR-3 only (keepalive controls, safe fallback/default behavior, non-exposure constraints)
- Verdict: **READY (PASS)**

## Reviewer Gate Summary (PR-3)

| Gate | Status | Reviewer evidence |
| --- | --- | --- |
| Targeted E2E rerun | PASS | Security settings targeted rerun completed: **30 passed, 0 failed**. |
| Local patch preflight | PASS | `frontend/coverage/lcov.info` present; `scripts/local-patch-report.sh` artifacts regenerated with `pass` status. |
| Coverage + type-check | PASS | Frontend coverage gate passed (89% lines vs 85% minimum); type-check passed. |
| Pre-commit + security scans | PASS | `pre-commit --all-files`, CodeQL Go/JS CI-aligned scans, findings gate, and Trivy checks passed (no HIGH/CRITICAL blockers). |
| Final readiness | PASS | All PR-3 closure gates are green. |

## Scope Guardrails Verified (PR-3)

- Keepalive controls are limited to approved PR-3 scope.
- Safe fallback behavior remains intact when keepalive values are missing or invalid.
- Non-exposure constraints remain intact (`trusted_proxies_unix` and certificate lifecycle internals are not exposed).

## Manual Verification Reference

- PR-3 manual test tracking plan: `docs/issues/manual_test_pr3_keepalive_controls_closure.md`

## PR-3 Closure Statement

PR-3 is **ready to merge** with no open QA blockers.

---

## QA/Security Audit — PR-2 Frontend Slice (Notifications)

- Date: 2026-02-24
- Scope: PR-2 frontend notifications slice only (UI/API contract alignment, tests, QA/security gates)
- Verdict: **READY (PASS WITH NON-BLOCKING WARNINGS)**

## Commands Run

1. `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
2. `/projects/Charon/node_modules/.bin/playwright test /projects/Charon/tests/settings/notifications.spec.ts --config=/projects/Charon/playwright.config.js --project=firefox`
3. `bash /projects/Charon/scripts/local-patch-report.sh`
4. `/projects/Charon/.github/skills/scripts/skill-runner.sh test-frontend-coverage`
5. `cd /projects/Charon/frontend && npm run type-check`
6. `cd /projects/Charon && pre-commit run --all-files`
7. VS Code task: `Security: CodeQL JS Scan (CI-Aligned) [~90s]`
8. VS Code task: `Security: CodeQL Go Scan (CI-Aligned) [~60s]`
9. `cd /projects/Charon && bash scripts/pre-commit-hooks/codeql-check-findings.sh`
10. `/projects/Charon/.github/skills/scripts/skill-runner.sh security-scan-trivy`

## Gate Results

| Gate | Status | Evidence |
| --- | --- | --- |
| 1) Playwright E2E first (notifications-focused) | PASS | `tests/settings/notifications.spec.ts`: **27 passed, 0 failed** after PR-2-aligned expectation update. |
| 2) Local patch coverage preflight artifacts | PASS (WARN) | Artifacts generated: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`; report mode=`warn` with `changed_lines=0` for current baseline range. |
| 3) Frontend coverage + threshold | PASS | `test-frontend-coverage` skill completed successfully; coverage gate **PASS** at **89% lines** vs minimum **87%**. |
| 4) TypeScript check | PASS | `npm run type-check` completed with `tsc --noEmit` and no type errors. |
| 5) `pre-commit run --all-files` | PASS | All configured hooks passed, including frontend lint/type checks and fast Go linters. |
| 6a) CodeQL JS (CI-aligned) | PASS | JS scan completed and SARIF generated (`codeql-results-js.sarif`). |
| 6b) CodeQL Go (CI-aligned) | PASS | Go scan completed and SARIF generated (`codeql-results-go.sarif`). |
| 6c) CodeQL findings gate | PASS | `scripts/pre-commit-hooks/codeql-check-findings.sh` reported no security issues in Go/JS. |
| 6d) Trivy filesystem scan | PASS | `security-scan-trivy` completed with **0 vulnerabilities** and **0 secrets** at configured severities. |
| 6e) GORM scanner | SKIPPED (N/A) | Not required for PR-2 frontend-only slice (no `backend/internal/models/**` or GORM persistence scope changes). |

## Low-Risk Fixes Applied During Audit

1. Updated Playwright notifications spec to match PR-2 provider UX (`discord/gotify/webhook` selectable, not disabled):
	- `tests/settings/notifications.spec.ts`
2. Updated legacy frontend API unit test expectations from Discord-only to supported provider contract:
	- `frontend/src/api/__tests__/notifications.test.ts`

## Blockers / Notes

- **No merge-blocking QA/security blockers** for PR-2 frontend slice.
- Non-blocking notes:
  - Local patch preflight is in `warn` mode with `changed_lines=0` against `origin/development...HEAD`; artifacts are present and valid.
  - Local command execution is cwd-sensitive; absolute paths were used for reliable gate execution.

## Recommendation

- **Proceed to PR-3**.
- No blocking items remain for the PR-2 frontend slice.

---

## Final QA/Security Audit — Notify Migration (PR-1/PR-2/PR-3)

- Date: 2026-02-24
- Scope: Final consolidated verification for completed notify migration slices (PR-1 backend, PR-2 frontend, PR-3 E2E/coverage hardening)
- Verdict: **ALL-PASS**

## Mandatory Gate Sequence Results

| Gate | Status | Evidence |
| --- | --- | --- |
| 1) Playwright E2E first (notifications-focused, including new payload suite) | PASS | `npx playwright test tests/settings/notifications.spec.ts tests/settings/notifications-payload.spec.ts --project=firefox --workers=1 --reporter=line` → **37 passed, 0 failed**. |
| 2) Local patch coverage preflight artifacts generation | PASS (WARN mode allowed) | `bash scripts/local-patch-report.sh` generated `test-results/local-patch-report.md` and `test-results/local-patch-report.json` with artifact verification. |
| 3) Backend coverage threshold check | PASS | `bash scripts/go-test-coverage.sh` → **Line coverage 87.4%**, minimum required **85%**. |
| 4) Frontend coverage threshold check | PASS | `bash scripts/frontend-test-coverage.sh` → **Lines 89%**, minimum required **85%** (coverage gate PASS). |
| 5) Frontend TypeScript check | PASS | `cd frontend && npm run type-check` completed with `tsc --noEmit` and no errors. |
| 6) `pre-commit run --all-files` | PASS | First run auto-fixed EOF in `tests/settings/notifications-payload.spec.ts`; rerun passed all hooks. |
| 7a) Trivy filesystem scan | PASS | `./.github/skills/scripts/skill-runner.sh security-scan-trivy` → no CRITICAL/HIGH/MEDIUM issues and no secrets detected. |
| 7b) Docker image scan | PASS | `./.github/skills/scripts/skill-runner.sh security-scan-docker-image` → **Critical 0 / High 0 / Medium 10 / Low 4**; gate policy passed (no critical/high). |
| 7c) CodeQL Go scan (CI-aligned) | PASS | CI-aligned Go scan completed; results written to `codeql-results-go.sarif`. |
| 7d) CodeQL JS scan (CI-aligned) | PASS | CI-aligned JS scan completed; results written to `codeql-results-js.sarif`. |
| 7e) CodeQL findings gate | PASS | `bash scripts/pre-commit-hooks/codeql-check-findings.sh` → no security issues in Go or JS findings gate. |
| 8) GORM security check mode (applicable) | PASS | `./scripts/scan-gorm-security.sh --check` → **0 CRITICAL / 0 HIGH / 0 MEDIUM**, INFO suggestions only. |

## Final Verdict

- all-pass / blockers: **ALL-PASS, no unresolved blockers**
- exact failing gates: **None (final reruns all passed)**
- proceed to handoff: **YES**

## Notes

- Transient issues were resolved during audit execution:
	- Initial Playwright run saw container availability drop (`ECONNREFUSED`); after E2E environment rebuild and deterministic rerun, gate passed.
	- Initial pre-commit run required one automatic EOF fix and passed on rerun.
	- Shell working-directory drift caused temporary command-not-found noise for root-level security scripts; rerun from repo root passed.

---

## Workflow Fix Validation — GHAS Trivy Compatibility (`docker-build.yml`)

- Date: 2026-02-24
- Scope: `.github/workflows/docker-build.yml` only
- Result: **PASS**

### Checks Run

1. Workflow lint/syntax:
	- `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/docker-build.yml` → `actionlint: OK`
	- `python3` YAML parse (`yaml.safe_load`) for `.github/workflows/docker-build.yml` → `YAML parse: OK`
2. Guard/category placement validation:
	- Verified Trivy compatibility uploads are gated with `if: always() && steps.trivy-pr-check.outputs.exists == 'true'`.
	- Verified compatibility uploads are non-blocking via `continue-on-error: true`.
	- Verified category aliases present:
	  - `.github/workflows/docker-build.yml:build-and-push`
	  - `.github/workflows/docker-publish.yml:build-and-push`
	  - `trivy-nightly`
	- Verified main Trivy SARIF upload for non-PR path now explicitly sets category `.github/workflows/docker-build.yml:build-and-push`.
3. Security regression review (workflow logic only):
	- Patch is additive for SARIF upload routing/compatibility and existence guard.
	- No new secret exposure, token scope elevation, or privilege expansion introduced.
	- No blocking behavior added to compatibility uploads.

### Blockers

- None.

### Proceed Recommendation

- **Proceed**. Workflow-only GHAS Trivy compatibility patch is validated and safe to merge.

---

## QA Validation — E2E Auth Helper + Local Docker Socket Diagnostics

- Date: 2026-02-24
- Scope: Validation only for:
	1. E2E shard failures previously tied to missing `Authorization` header in test helpers (`createUser` path)
	2. Local Docker socket connection diagnostics/behavior
- Verdict: **PASS for both target tracks** (with unrelated shard test failures outside this scope)

### Commands Executed

1. `./.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
2. `pushd /projects/Charon >/dev/null && if [ -f .env ]; then set -a; . ./.env; set +a; fi && : "${CHARON_EMERGENCY_TOKEN:?CHARON_EMERGENCY_TOKEN is required (set it in /projects/Charon/.env)}" && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false PLAYWRIGHT_SKIP_SECURITY_DEPS=1 TEST_WORKER_INDEX=1 npx playwright test --project=firefox --shard=1/4 --output=playwright-output/firefox-shard-1 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks`
3. `pushd /projects/Charon >/dev/null && if [ -f .env ]; then set -a; . ./.env; set +a; fi && : "${CHARON_EMERGENCY_TOKEN:?CHARON_EMERGENCY_TOKEN is required (set it in /projects/Charon/.env)}" && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false PLAYWRIGHT_SKIP_SECURITY_DEPS=1 npx playwright test --project=firefox tests/fixtures/api-helper-auth.spec.ts`
4. `pushd /projects/Charon/backend >/dev/null && go test -count=1 -v ./internal/services -run 'TestDockerService|TestIsDocker|TestResolveDockerHost|TestBuildLocalDockerUnavailableDetails|TestGetErrorResponseDetails' && go test -count=1 -v ./internal/api/handlers -run 'TestDockerHandler'`

### Results

| Check | Status | Output Summary |
| --- | --- | --- |
| E2E environment rebuild | PASS | `charon-e2e` rebuilt and healthy; health endpoint responsive. |
| CI-style non-security shard | PARTIAL (out-of-scope failures) | `124 passed`, `3 failed` in `tests/core/data-consistency.spec.ts` and `tests/core/domain-dns-management.spec.ts`; **no** `Failed to create user: {"error":"Authorization header required"}` observed. |
| Focused `createUser` auth-path spec | PASS | `tests/fixtures/api-helper-auth.spec.ts` → `2 passed (4.5s)`. |
| Backend docker service/handler tests | PASS | Targeted suites passed, including local diagnostics and mapping: `ok .../internal/services`, `ok .../internal/api/handlers`. |

### Local Docker API Path / Diagnostics Validation

- Verified via backend tests that local-mode behavior and diagnostics are correct:
	- Local host resolution includes unix socket preference path (`unix:///var/run/docker.sock`) in service tests.
	- Connectivity classification passes for permission denied, missing socket, daemon connectivity, timeout, and syscall/network error paths.
	- Handler mapping passes for docker-unavailable scenarios and returns actionable details with `503` path assertions.

### Env-only vs Regression Classification

- Track 1 (`createUser` Authorization helper path): **No regression detected**.
	- Focused spec passes and representative shard no longer shows prior auth-header failure signature.
- Track 2 (local Docker socket diagnostics/behavior): **No regression detected**.
	- Targeted backend tests pass across local unix socket and failure diagnostic scenarios.
- Remaining shard failures: **Out of scope for requested tracks** (not env bootstrap failures and not related to auth-helper/docker-socket fixes).

---

## Fast Playwright No-HTML Triage (PR #754)

- Date: 2026-02-25
- Scope: Focused CI-like local rerun for previously failing no-HTML Playwright specs on Firefox and Chromium
- Result: **PASS**

### Commands Used

1. `pushd /projects/Charon >/dev/null && if [ -f .env ]; then set -a; . ./.env; set +a; fi && export CHARON_EMERGENCY_TOKEN="${CHARON_EMERGENCY_TOKEN:-test-emergency-token-for-e2e-32chars}" && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false PLAYWRIGHT_SKIP_SECURITY_DEPS=1 npx playwright test --project=firefox tests/settings/no-html.spec.ts tests/settings/notifications-no-html.spec.ts tests/core/no-html-hardening.spec.ts tests/integration/no-html-regression.spec.ts`
2. `pushd /projects/Charon >/dev/null && if [ -f .env ]; then set -a; . ./.env; set +a; fi && export CHARON_EMERGENCY_TOKEN="${CHARON_EMERGENCY_TOKEN:-test-emergency-token-for-e2e-32chars}" && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false PLAYWRIGHT_SKIP_SECURITY_DEPS=1 npx playwright test --project=chromium tests/settings/no-html.spec.ts tests/settings/notifications-no-html.spec.ts tests/core/no-html-hardening.spec.ts tests/integration/no-html-regression.spec.ts`

### Results

| Browser | Status | Output Summary |
| --- | --- | --- |
| Firefox | PASS | **43 passed, 0 failed** |
| Chromium | PASS | **43 passed, 0 failed** |

### Conclusion

All four previously failing specs are green locally when executed in CI-like environment settings.
