# QA/Security Audit Report — PR-1

Date: 2026-02-18
Scope: PR-1 in `docs/plans/current_spec.md` (high-risk findings only)

## Audit Scope and Target Findings

PR-1 target findings:
- `go/log-injection`
- `go/cookie-secure-not-set`
- `js/regex/missing-regexp-anchor`
- `js/insecure-temporary-file`

PR-1 touched areas (from plan/status artifacts):
- Backend handlers/services/middleware/security modules listed in `docs/reports/pr1_backend_impl_status.md`
- Frontend/test files listed in `docs/reports/pr1_frontend_impl_status.md`

## Definition of Done Gate Results (Ordered)

| Gate | Command/Method | Result | Status |
|---|---|---|---|
| 0. E2E env readiness (prereq) | Task: `Docker: Rebuild E2E Environment` | Container rebuilt and healthy (`charon-e2e`) | PASS |
| 1. Playwright E2E first (targeted touched suites) | `npx playwright test --project=firefox tests/tasks/import-caddyfile.spec.ts tests/security-enforcement/zzz-caddy-imports/caddy-import-cross-browser.spec.ts` | `20 failed`, `1 passed` (root error: `Failed to create user: {"error":"Authorization header required"}` from `tests/utils/TestDataManager.ts:494`) | FAIL |
| 1b. Cross-browser touched suite explicit run | `npx playwright test tests/security-enforcement/zzz-caddy-imports/caddy-import-cross-browser.spec.ts --project=chromium --project=firefox --project=webkit` | `Error: No tests found` for this invocation | FAIL |
| 2. Local patch coverage preflight (first attempt, in-order) | `bash scripts/local-patch-report.sh` | Failed: missing `frontend/coverage/lcov.info` | FAIL |
| 2b. Local patch coverage preflight (rerun after coverage) | `bash scripts/local-patch-report.sh` | Output said generated + warnings (`overall 85.2% < 90`, backend `84.7% < 85`) but artifacts not found in workspace (`test-results/local-patch-report.{md,json}` absent) | FAIL |
| 3. CodeQL Go (CI-aligned) | Task: `Security: CodeQL Go Scan (CI-Aligned) [~60s]` | Completed; SARIF produced (`codeql-results-go.sarif`) | PASS |
| 3b. CodeQL JS (CI-aligned) | Task: `Security: CodeQL JS Scan (CI-Aligned) [~90s]` | Completed; SARIF produced (`codeql-results-js.sarif`) | PASS |
| 3c. CodeQL blocking findings gate | `pre-commit run --hook-stage manual codeql-check-findings --all-files` | Passed (no blocking security issues in go/js) | PASS |
| 4. Pre-commit all-files | `pre-commit run --all-files` | All hooks passed | PASS |
| 5. Backend coverage suite | `.github/skills/scripts/skill-runner.sh test-backend-coverage` (with `.env` loaded) | Coverage gate met (`line 87.0%`), but test suite failed (`TestSetSecureCookie_*` failures) | FAIL |
| 6. Frontend coverage suite | `.github/skills/scripts/skill-runner.sh test-frontend-coverage` | Passed; line coverage `88.57%` | PASS |
| 7. Frontend type-check | `cd frontend && npm run type-check` | Passed (`tsc --noEmit`) | PASS |
| 8. Trivy filesystem scan | `.github/skills/scripts/skill-runner.sh security-scan-trivy` | Passed (no vuln/secret findings in scanned targets) | PASS |
| 9. Docker image security scan | Task: `Security: Scan Docker Image (Local)` | Failed due `1 High` vulnerability: `GHSA-69x3-g4r3-p962` in `github.com/slackhq/nebula@v1.9.7` (fixed `1.10.3`) | FAIL |
| 10. Go vulnerability check (additional) | Task: `Security: Go Vulnerability Check` | No vulnerabilities found | PASS |

## PR-1 Security Finding Remediation Verification

Verification source: latest CI-aligned SARIF outputs + `jq` rule counts on `.runs[0].results[].ruleId`.

- `go/log-injection`: `0`
- `go/cookie-secure-not-set`: `0`
- `js/regex/missing-regexp-anchor`: `0`
- `js/insecure-temporary-file`: `0`

Result: **Target PR-1 CodeQL findings are remediated in current local scan outputs.**

## Blockers and Impact

1. **Targeted E2E gate failing**
   - Blocker: test data bootstrap unauthorized (`Authorization header required`) in import suite.
   - Impact: cannot claim PR-1 behavioral regression safety in affected user workflow.

2. **Cross-browser touched suite not runnable in current invocation**
   - Blocker: `No tests found` when executing `caddy-import-cross-browser.spec.ts` directly.
   - Impact: required touched-suite validation is incomplete for that file.

3. **Patch preflight artifact inconsistency**
   - Blocker: script reports generated artifacts, but files are absent in workspace.
   - Impact: required evidence artifacts are missing; changed-line coverage visibility is not auditable.

4. **Backend coverage suite has failing tests**
   - Blocker: multiple `TestSetSecureCookie_*` failures.
   - Impact: backend gate fails despite acceptable aggregate coverage.

5. **Docker image scan high vulnerability**
   - Blocker: `GHSA-69x3-g4r3-p962` high severity in image SBOM.
   - Impact: security release gate blocked.

6. **Trivy MCP adapter invocation failure (tooling path)**
   - Blocker: direct MCP call `mcp_trivy_mcp_scan_filesystem` returned `MPC -32603: failed to scan project`.
   - Impact: scanner execution had to fall back to repository skill runner; filesystem scan result is still available, but MCP-path reliability should be investigated.

## Prioritized Remediation Plan (Owner-Mapped)

1. **P0 — Fix E2E auth bootstrap regression**
   Owner: **Backend Dev + QA/E2E**
   - Restore/align authorization expectations for user-creation path used by `TestDataManager.createUser`.
   - Re-run targeted E2E for `tests/tasks/import-caddyfile.spec.ts` until green.

2. **P0 — Resolve backend failing tests (`TestSetSecureCookie_*`)**
   Owner: **Backend Dev**
   - Reconcile cookie security behavior vs test expectations (localhost/forwarded host/scheme cases).
   - Update implementation/tests only after confirming intended security policy.

3. **P0 — Remediate high image vulnerability (`GHSA-69x3-g4r3-p962`)**
   Owner: **DevOps + Backend Dev**
   - Upgrade `github.com/slackhq/nebula` to fixed version (`>=1.10.3`) and rebuild image.
   - Re-run image scan and confirm `Critical=0`, `High=0`.

4. **P1 — Make cross-browser touched suite executable in CI/local targeted mode**
   Owner: **QA/E2E**
   - Verify Playwright config grep/match filters for `@cross-browser` suite and ensure discoverability.
   - Re-run suite across `chromium/firefox/webkit` and capture pass evidence.

5. **P1 — Fix local patch preflight artifact emission path/evidence**
   Owner: **DevOps + QA Tooling**
   - Ensure `scripts/local-patch-report.sh` reliably writes `test-results/local-patch-report.md` and `.json`.
   - Validate artifact existence post-run and fail fast if missing.

## Final Verdict

**FAIL**

Rationale:
- PR-1 target CodeQL security findings are cleared (good), but multiple Definition of Done gates are still failing (E2E targeted suites, backend coverage test pass, patch preflight artifact evidence, and Docker image high vulnerability). PR-1 is not releasable under current QA/Security gate policy.
