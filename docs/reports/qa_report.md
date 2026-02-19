# QA/Security Validation Report - Current Branch

**Date:** 2026-02-19
**Repository:** `/projects/Charon`
**Workflow:** Required QA/security gate sequence (E2E-first, coverage, pre-commit, security scans)

## Gate Results (Latest Run)

| # | Gate | Command(s) | Status | Evidence / Artifacts |
|---|---|---|---|---|
| 1 | Determine E2E rebuild requirement + rebuild if needed | `git diff` context via changed files + Task `Docker: Rebuild E2E Environment` + `docker inspect` health check | **PASS** | Rebuild required because branch modifies runtime inputs (`backend/**`, `frontend/**`). Rebuild executed; `charon-e2e` is `healthy`. |
| 2a | Required Playwright targeted suite (changed area) | `PLAYWRIGHT_HTML_OPEN=never npx playwright test --config /projects/Charon/playwright.config.js /projects/Charon/tests/settings/notifications.spec.ts --project=firefox` | **FAIL** | `25 passed`, `2 failed` in `tests/settings/notifications.spec.ts`: (1) `should enable/disable provider` timeout due click interception/out-of-viewport on `security-notifications-enabled`; (2) `should show preview error for invalid template` matched hidden `Error` option instead of visible error feedback. |
| 2b | Security-project equivalent (firefox excludes security folder) | `PLAYWRIGHT_HTML_OPEN=never npx playwright test --config /projects/Charon/playwright.config.js /projects/Charon/tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts --project=security-tests` | **FAIL** | `20 passed`, `1 failed`: `should save general settings successfully` expected success toast (`toast-success`/status role) not visible within timeout. |
| 3 | Local patch coverage preflight + artifact verification | `bash /projects/Charon/scripts/local-patch-report.sh` + artifact `ls` verification | **FAIL (artifacts present)** | Script exits with `Error: frontend coverage input missing at /projects/Charon/frontend/coverage/lcov.info`. Required artifacts are still generated in warn/input-missing mode: `test-results/local-patch-report.md`, `test-results/local-patch-report.json`. |
| 4a | Backend coverage >=85% | `CHARON_ENCRYPTION_KEY=<generated> /projects/Charon/.github/skills/scripts/skill-runner.sh test-backend-coverage` | **PASS** | Backend coverage gate passed. Summary: statements `87.0%`, lines `87.3%` (min `85%`). |
| 4b | Frontend coverage >=85% | `/projects/Charon/.github/skills/scripts/skill-runner.sh test-frontend-coverage` | **FAIL** | Test run failed before gate completion: `src/pages/__tests__/Security.functional.test.tsx` test `should disable Notifications button when Cerberus is disabled` expects disabled button but button is enabled. |
| 5 | Frontend type-check | `cd /projects/Charon/frontend && npm run type-check` | **PASS** | `tsc --noEmit` completed without TypeScript errors. |
| 6 | Pre-commit all files | `pre-commit run --all-files` | **PASS** | Initial runs auto-fixed trailing whitespace (`docs/plans/current_spec.md`, then `docs/reports/qa_report.md`); subsequent rerun completed with all hooks passing. |
| 7a | Trivy filesystem scan | `/projects/Charon/.github/skills/scripts/skill-runner.sh security-scan-trivy` | **PASS** | Summary: `0` vulnerabilities, `0` secrets across scanned targets (`backend/go.mod`, `frontend/package-lock.json`, `package-lock.json`, `playwright/.auth/user.json`). |
| 7b | Docker image scan | Task `Security: Scan Docker Image (Local)` | **PASS** | Grype summary: `0 critical`, `0 high`, `9 medium`, `4 low` (total `13`). Output notes non-blocking medium/low only. |
| 7c | CodeQL Go (CI-aligned) | Task `Security: CodeQL Go Scan (CI-Aligned) [~60s]` | **PASS** | Completed, extraction parity OK (`compiled baseline=178, extracted=178`), SARIF generated: `codeql-results-go.sarif`. |
| 7d | CodeQL JS (CI-aligned) | `bash scripts/pre-commit-hooks/codeql-js-scan.sh` (from repo root after stale DB cleanup) | **PASS** | Completed; CodeQL scanned `346/346` JS/TS files. SARIF generated: `codeql-results-js.sarif`. |
| 7e | CodeQL findings gate | `pre-commit run --hook-stage manual codeql-check-findings --all-files` | **PASS** | Hook output: no HIGH/CRITICAL findings in Go or JS SARIF. |

## Exact Failing Commands + Root Cause

1. **Playwright notifications suite**
  Command: `PLAYWRIGHT_HTML_OPEN=never npx playwright test --config /projects/Charon/playwright.config.js /projects/Charon/tests/settings/notifications.spec.ts --project=firefox`
  Root cause: two failing assertions in suite (`setChecked` click interception/viewport issue; hidden text match in invalid-template error assertion).

2. **Playwright security-project suite**
  Command: `PLAYWRIGHT_HTML_OPEN=never npx playwright test --config /projects/Charon/playwright.config.js /projects/Charon/tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts --project=security-tests`
  Root cause: success toast not visible after settings save in `system-security-settings.spec.ts`.

3. **Local patch preflight**
  Command: `bash /projects/Charon/scripts/local-patch-report.sh`
  Root cause: required frontend LCOV input missing (`frontend/coverage/lcov.info`); report emitted in input-missing warning mode.

4. **Frontend coverage gate**
  Command: `/projects/Charon/.github/skills/scripts/skill-runner.sh test-frontend-coverage`
  Root cause: failing frontend test in `Security.functional.test.tsx` (`Notifications` button disable expectation no longer matches current UI behavior).

## Final Verdict

- **Overall Result: FAIL**
- **Blocking failed gates:** 2a, 2b, 3, 4b
- **Security scan status:** Trivy PASS, Docker image scan PASS (no high/critical), CodeQL Go/JS PASS, CodeQL findings gate PASS
