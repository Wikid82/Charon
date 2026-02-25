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

---

## QA/Security Delta — Post-Hardening E2E Remediation Pass

- Date: 2026-02-25
- Scope: Post-hardening E2E remediation for authz restrictions, secret redaction behavior, setup/security guardrails, and settings endpoint protections.
- Final Status: **PASS FOR REMEDIATION SCOPE** (targeted hardening suites green; see non-scope blockers below).

### Commands Run

1. `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
2. `.github/skills/scripts/skill-runner.sh test-e2e-playwright`
3. `PLAYWRIGHT_HTML_OPEN=never npx playwright test tests/security tests/security-enforcement tests/settings --project=firefox`
4. `PLAYWRIGHT_HTML_OPEN=never npx playwright test tests/security tests/security-enforcement tests/settings --project=firefox` (post-fix rerun)
5. `PLAYWRIGHT_HTML_OPEN=never npx playwright test tests/settings/account-settings.spec.ts tests/settings/notifications-payload.spec.ts --project=firefox`
6. `bash scripts/local-patch-report.sh`
7. `.github/skills/scripts/skill-runner.sh test-backend-coverage`
8. `.github/skills/scripts/skill-runner.sh test-frontend-coverage`
9. `.github/skills/scripts/skill-runner.sh qa-precommit-all`
10. VS Code task: `Security: CodeQL Go Scan (CI-Aligned) [~60s]`
11. VS Code task: `Security: CodeQL JS Scan (CI-Aligned) [~90s]`
12. `pre-commit run --hook-stage manual codeql-go-scan --all-files`
13. `pre-commit run --hook-stage manual codeql-js-scan --all-files`
14. `pre-commit run --hook-stage manual codeql-check-findings --all-files`
15. `.github/skills/scripts/skill-runner.sh security-scan-trivy`
16. `.github/skills/scripts/skill-runner.sh security-scan-docker-image`

### Gate Results

| Gate | Status | Evidence |
| --- | --- | --- |
| E2E-first hardening verification | PASS (targeted) | Remediated files passed: `tests/settings/account-settings.spec.ts` and `tests/settings/notifications-payload.spec.ts` → **30/30 passed**. |
| Local patch preflight artifacts | PASS (WARN) | `test-results/local-patch-report.md` and `test-results/local-patch-report.json` generated; warning mode due patch coverage below configured threshold. |
| Backend coverage threshold | PASS | Coverage gate met (minimum **87%** required by local gate). |
| Frontend coverage threshold | PASS | Coverage summary: **Lines 88.92%**; gate PASS vs **87%** minimum. |
| Pre-commit all-files | PASS | `.github/skills/scripts/skill-runner.sh qa-precommit-all` passed all hooks. |
| CodeQL Go/JS + findings gate | PASS | Manual-stage scans executed and findings gate reports no security issues in Go/JS. |
| Trivy filesystem | PASS | `security-scan-trivy` completed with no reported issues at configured severities. |
| Docker image vulnerability gate | PASS | No blocking critical/high vulnerabilities; non-blocking medium/low remain tracked in generated artifacts. |
| GORM scanner | N/A | Not triggered: this remediation changed only E2E test files, not backend model/database scope. |

### Remediation Notes

1. Updated account settings E2E to reflect hardened API-key redaction behavior:
	- Assert masked display and absence of copy action for API key.
	- Assert regeneration success without expecting raw key disclosure.
2. Updated notifications payload E2E to reflect hardened endpoint protection and trusted-provider test dispatch model:
	- Added authenticated headers where protected endpoints are exercised.
	- Updated assertions to expect guardrail contract (`MISSING_PROVIDER_ID`) for untrusted direct dispatch payloads.

### Non-Scope Blockers (Observed in Broader Rerun)

- A broad `tests/settings` rerun still showed unrelated failures in:
  - `tests/settings/notifications.spec.ts` (event persistence reload timeout)
  - `tests/settings/smtp-settings.spec.ts` (reload timeout)
  - `tests/settings/user-management.spec.ts` (pending invite/reinvite timing)
- These were not introduced by this remediation and were outside the hardening-failure set addressed here.

### Recommendation

- Continue with a separate stability pass for the remaining non-scope settings suite timeouts.
- For this post-hardening remediation objective, proceed with the current changes.

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

---

## Deep Security Audit — Huntarr-Style Hardening (Charon)

- Date: 2026-02-25
- Scope: Full backend/API/runtime/CI posture against Huntarr-style failure modes and self-hosted hardening requirements
- Constraint honored: `docs/plans/current_spec.md` was not modified
- Verdict: **FAIL (P0 findings present)**

### Executive Summary

Charon has strong baseline controls (JWT auth middleware, setup lockout, non-root container runtime, emergency token constant-time verification, and active CI security gates), but this audit found critical gaps in authorization boundaries and secret exposure behavior. The most severe risks are: (1) security-control mutation endpoints accessible to any authenticated user in multiple handlers, (2) import preview/status endpoints exposed without auth middleware and without admin checks, and (3) sensitive values returned in generic settings/profile/invite responses. One container-image vulnerability (HIGH) is also present in `usr/bin/caddy`.

### Commands Executed

1. `shell: Security: CodeQL All (CI-Aligned)`
2. `shell: Security: CodeQL Go Scan (CI-Aligned) [~60s]`
3. `shell: Security: CodeQL JS Scan (CI-Aligned) [~90s]`
4. `python3` SARIF summary (`codeql-results-go.sarif`, `codeql-results-js.sarif`, `codeql-results-javascript.sarif`)
5. `pre-commit run codeql-check-findings --all-files` (hook not registered locally; see blockers)
6. `.github/skills/scripts/skill-runner.sh security-scan-trivy vuln,secret,misconfig json > trivy-report.json` (misconfig scanner panic; see blockers)
7. `docker run ... aquasec/trivy:latest fs --scanners vuln,secret ... --format json > vuln-results.json`
8. `docker run ... aquasec/trivy:latest image ... charon:local > trivy-image-report.json`
9. `./scripts/scan-gorm-security.sh --check`
10. `pre-commit run --all-files`

### Gate Results

| Gate | Status | Evidence |
| --- | --- | --- |
| CodeQL (Go + JS SARIF artifacts) | PASS | `codeql-results-go.sarif`, `codeql-results-js.sarif`, `codeql-results-javascript.sarif` all contained `0` results. |
| Trivy filesystem (actionable scope: vuln+secret) | PASS | `vuln-results.json` reported `0` CRITICAL/HIGH findings after excluding local caches. |
| Trivy image scan (`charon:local`) | **FAIL** | `trivy-image-report.json`: `1` HIGH vulnerability (`CVE-2026-25793`) in `usr/bin/caddy` (`github.com/slackhq/nebula v1.9.7`). |
| GORM security gate (`--check`) | PASS | `0` CRITICAL/HIGH/MEDIUM; `2` INFO only. |
| Pre-commit full gate | PASS | `pre-commit run --all-files` passed all configured hooks. |

### Findings

| ID | Severity | Category | CWE / OWASP | Evidence | Impact | Exploitability | Remediation |
| --- | --- | --- | --- | --- | --- | --- | --- |
| F-001 | **Critical** | Broken authorization on security mutation endpoints | CWE-862 / OWASP A01 | `backend/internal/api/routes/routes.go` exposes `/api/v1/security/config`, `/security/breakglass/generate`, `/security/decisions`, `/security/rulesets*` under authenticated routes; corresponding handlers in `backend/internal/api/handlers/security_handler.go` (`UpdateConfig`, `GenerateBreakGlass`, `CreateDecision`, `UpsertRuleSet`, `DeleteRuleSet`) do not enforce admin role. | Any authenticated non-admin can alter core security controls, generate break-glass token material, and tamper with decision/ruleset state. | High (single authenticated request path). | Enforce admin authorization at route-level or handler-level for all security-mutating endpoints; add deny-by-default middleware tests for all `/security/*` mutators. |
| F-002 | **High** | Unauthenticated import status/preview exposure | CWE-200 + CWE-306 / OWASP A01 + A04 | `backend/internal/api/routes/routes.go` registers import handlers via `RegisterImportHandler`; `backend/internal/api/routes/routes.go` `RegisterImportHandler()` mounts `/api/v1/import/*` without auth middleware. In `backend/internal/api/handlers/import_handler.go`, `GetStatus` and `GetPreview` lack `requireAdmin` checks and can return `caddyfile_content`. | Potential disclosure of infrastructure hostnames/routes/config snippets to unauthenticated users. | Medium-High (network-accessible management endpoint). | Move import routes into protected/admin group; require admin check in `GetStatus` and `GetPreview`; redact/remove raw `caddyfile_content` from API responses. |
| F-003 | **High** | Secret disclosure in API responses | CWE-200 / OWASP A02 + A01 | `backend/internal/api/handlers/settings_handler.go` `GetSettings()` returns full key/value map; `backend/internal/services/mail_service.go` persists `smtp_password` in settings. `backend/internal/api/handlers/user_handler.go` returns `api_key` in profile/regenerate responses and `invite_token` in invite/create/resend flows. | Secrets and account takeover tokens can leak through UI/API, logs, browser storage, and support channels. | Medium (requires authenticated access for some paths; invite token leak is high-risk in admin workflows). | Introduce server-side secret redaction policy: write-only secret fields, one-time reveal tokens, and masked settings API; remove raw invite/API key returns except explicit one-time secure exchange endpoints with re-auth. |
| F-004 | **Medium** | Dangerous operation controls incomplete | CWE-285 / OWASP A01 | High-impact admin operations (security toggles, user role/user deletion pathways) do not consistently require re-auth/step-up confirmation; audit exists in places but not uniformly enforced with confirmation challenge. | Increases blast radius of stolen session or accidental clicks for destructive operations. | Medium. | Add re-auth (password/TOTP) for dangerous operations and explicit confirmation tokens with short TTL; enforce audit record parity for every security mutation endpoint. |
| F-005 | **Medium** | Secure-by-default network exposure posture | CWE-1327 / OWASP A05 | `backend/cmd/api/main.go` starts HTTP server on `:<HTTPPort>` (all interfaces). Emergency server defaults are safer, but management API default bind remains broad in self-hosted deployments. | Expanded attack surface if deployment network controls are weak/misconfigured. | Medium (environment dependent). | Default management bind to loopback/private interface and require explicit opt-in for public exposure; document hardened reverse-proxy-only deployment mode. |
| F-006 | **Medium** | Container image dependency vulnerability | CWE-1104 / OWASP A06 | `trivy-image-report.json`: `HIGH CVE-2026-25793` in `usr/bin/caddy` (`github.com/slackhq/nebula v1.9.7`) in `charon:local`. | Potential exposure via vulnerable transitive component in runtime image. | Medium (depends on exploit preconditions). | Rebuild with patched Caddy base/version; pin and verify fixed digest; keep image scan as blocking CI gate for CRITICAL/HIGH. |

### Setup-Mode Re-entry Assessment

- **Pass**: `backend/internal/api/handlers/user_handler.go` blocks setup when user count is greater than zero (`Setup already completed`).
- Residual risk: concurrent first-run race conditions are still theoretically possible if multiple setup requests arrive before first transaction commits.

### Charon Safety Contract (Current State)

| Invariant | Status | Notes |
| --- | --- | --- |
| No state-changing endpoint without strict authz | **FAIL** | Security mutators and import preview/status gaps violate deny-by-default authorization expectations. |
| No raw secrets in API/logs/diagnostics | **FAIL** | Generic settings/profile/invite responses include sensitive values/tokens. |
| Secure-by-default management exposure | **PARTIAL** | Emergency server defaults safer; main API bind remains broad by default. |
| Dangerous operations require re-auth + audit | **PARTIAL** | Audit is present in parts; step-up re-auth/confirmation is inconsistent. |
| Setup mode is one-way lockout after initialization | **PASS** | Setup endpoint rejects execution when users already exist. |

### Prioritized Remediation Plan

**P0 (block release / immediate):**

1. Enforce admin authz on all `/security/*` mutation endpoints (`UpdateConfig`, `GenerateBreakGlass`, `CreateDecision`, `UpsertRuleSet`, `DeleteRuleSet`, and any equivalent mutators).
2. Move all import endpoints behind authenticated admin middleware; add explicit admin checks to `GetStatus`/`GetPreview`.
3. Remove raw secret/token disclosure from settings/profile/invite APIs; implement write-only and masked read semantics.

**P1 (next sprint):**

1. Add step-up re-auth for dangerous operations (security toggles, user deletion/role changes, break-glass token generation).
2. Add explicit confirmation challenge for destructive actions with short-lived confirmation tokens.
3. Resolve image CVE by upgrading/pinning patched Caddy dependency and re-scan.

**P2 (hardening backlog):**

1. Tighten default bind posture for management API.
2. Add startup race protection for first-run setup path.
3. Expand documentation redaction standards for tokenized URLs and support artifacts.

### CI Tripwires (Required Enhancements)

1. **Route-auth crawler test (new):** enumerate all API routes and fail CI when any state-changing route (`POST/PUT/PATCH/DELETE`) is not protected by auth + role policy.
2. **Secret exposure contract tests:** assert sensitive keys (`smtp_password`, API keys, invite tokens, provider tokens) are never returned by generic read APIs.
3. **Security mutator RBAC tests:** negative tests for non-admin callers on all `/security/*` mutators.
4. **Image vulnerability gate:** fail build on CRITICAL/HIGH vulnerabilities unless explicit waiver with expiry exists.
5. **Trivy misconfig stability gate:** pin Trivy version or disable known-crashing parser path until upstream fix; keep scanner reliability monitored.

### Blockers / Tooling Notes

- `pre-commit run codeql-check-findings --all-files` failed locally because hook id is not registered in current pre-commit stage.
- Trivy `misconfig` scanner path crashed with a nil-pointer panic in Ansible parser during full filesystem scan; workaround used (`vuln,secret`) for actionable gate execution.

### Final DoD / Security Gate Decision

- **Overall Security Gate:** **FAIL** (due to unresolved P0 findings F-001/F-002/F-003 and one HIGH image vulnerability F-006).
- **If this code were Huntarr, would we call it safe now?** **No** — not until P0 authorization and secret-exposure issues are remediated and re-validated.

### Remediation Update (2026-02-25)

- Scope: P0 backend remediations from this audit were implemented in a single change set; `docs/plans/current_spec.md` remained untouched.

**F-001 — Security mutator authorization:**

- Added explicit admin checks in security mutator handlers (`UpdateConfig`, `GenerateBreakGlass`, `CreateDecision`, `UpsertRuleSet`, `DeleteRuleSet`, `ReloadGeoIP`, `LookupGeoIP`, `AddWAFExclusion`, `DeleteWAFExclusion`).
- Updated security route wiring so mutation endpoints are mounted under admin-protected route groups.
- Added/updated negative RBAC tests to verify non-admin callers receive `403` for security mutators.

**F-002 — Import endpoint protection:**

- Updated import route registration to require authenticated admin middleware for `/api/v1/import/*` endpoints.
- Added admin enforcement in `GetStatus` and `GetPreview` handlers.
- Added/updated route tests to verify unauthenticated and non-admin access is blocked.

**F-003 — Secret/token exposure prevention:**

- Updated settings read behavior to mask sensitive values and return metadata flags instead of raw secret values.
- Removed raw `api_key` and invite token disclosure from profile/regenerate/invite responses; responses now return masked/redacted values and metadata.
- Updated handler tests to enforce non-disclosure response contracts.

**Validation executed for this remediation update:**

- `go test ./internal/api/handlers -run 'SecurityHandler|ImportHandler|SettingsHandler|UserHandler'` ✅
- `go test ./internal/api/routes` ✅

**Residual gate status after this remediation update:**

- P0 backend findings F-001/F-002/F-003 are addressed in code and covered by updated tests.
- Image vulnerability finding F-006 remains open until runtime image dependency update and re-scan.
