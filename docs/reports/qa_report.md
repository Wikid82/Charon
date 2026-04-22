# QA/Security DoD Audit Report — Issue #929

Date: 2026-04-21
Repository: /projects/Charon
Branch: feature/beta-release
Scope assessed: DoD revalidation after recent fixes (E2E-first, frontend coverage, pre-commit/version gate, SA1019, Trivy CVE check)

## Final Recommendation

FAIL

Reason: Two mandatory gates are still failing in current rerun evidence:
- Playwright E2E-first gate
- Frontend coverage gate

Pre-commit/version-check is now passing.

## Gate Summary

| # | DoD Gate | Status | Notes |
|---|---|---|---|
| 1 | Playwright E2E first | FAIL | Healthy container path confirmed (`charon-e2e Up ... (healthy)`), auth setup passes, but accessibility suite still has 1 failing test (security headers page axe timeout) |
| 2 | Frontend coverage | FAIL | `scripts/frontend-test-coverage.sh` still ends with unhandled `ENOENT` on `frontend/coverage/.tmp/coverage-132.json` |
| 3 | Pre-commit hooks + version check | PASS | `lefthook run pre-commit --all-files` passes; `check-version-match` passes (`.version` matches latest tag `v0.27.0`) |
| 4 | SA1019 reconfirmation | PASS | `golangci-lint run ./... --enable-only staticcheck` reports `0 issues`; no `SA1019` occurrences |
| 5 | Trivy FS status (CVE-2026-34040) | PASS (not detected) | Current FS scan (`trivy fs --scanners vuln .`) exits 0 with no CVE hit; `CVE-2026-34040` not present in available Trivy artifacts |

## Detailed Evidence

### 1) Playwright E2E-first gate (revalidated)

Execution evidence:
- Container health:
  - `docker ps --filter name=charon-e2e --format '{{.Names}} {{.Status}}'`
  - Output: `charon-e2e Up 35 minutes (healthy)`
- Auth setup:
  - `PLAYWRIGHT_HTML_OPEN=never npx playwright test --project=firefox tests/auth.setup.ts -g "authenticate"`
  - Result: `1 passed`
  - Evidence: `Login successful`
- Accessibility rerun:
  - `PLAYWRIGHT_HTML_OPEN=never npx playwright test --project=firefox -g "accessibility"`
  - Result: `1 failed, 2 skipped, 64 passed`
  - Failing test:
    - `tests/a11y/security.a11y.spec.ts:21:5`
    - `Accessibility: Security › security headers page has no critical a11y violations`
  - Failure detail: `Test timeout of 90000ms exceeded` during axe analyze step.

Gate disposition: FAIL.

### 2) Frontend coverage gate (revalidated)

Execution:
- `bash scripts/frontend-test-coverage.sh`

Result:
- Coverage run still fails with unhandled rejection.
- Blocking error remains present:
  - `Error: ENOENT: no such file or directory, open '/projects/Charon/frontend/coverage/.tmp/coverage-132.json'`
- Run summary before abort:
  - `Test Files 128 passed | 5 skipped (187)`
  - `Tests 1918 passed | 90 skipped (2008)`

Additional state:
- `frontend/coverage/lcov.info` and `frontend/coverage/coverage-summary.json` can exist despite gate failure, but command-level DoD gate remains FAIL due non-zero termination path from unhandled ENOENT.

Gate disposition: FAIL.

### 3) Pre-commit hooks + version-check gate (revalidated)

Execution:
- `lefthook run pre-commit --all-files`
- `bash ./scripts/check-version-match-tag.sh`

Result:
- Pre-commit summary shows all required hooks completed successfully, including:
  - `check-version-match`
  - `golangci-lint-fast`
  - `frontend-type-check`
  - `frontend-lint`
  - `semgrep`
- Version check output:
  - `OK: .version matches latest Git tag v0.27.0`

Gate disposition: PASS.

### 4) SA1019 reconfirmation

Execution:
- `cd backend && golangci-lint run ./... --enable-only staticcheck`

Result:
- Output: `0 issues.`
- Additional grep for `SA1019`: no matches.

Conclusion: SA1019 remains resolved.

### 5) Trivy FS reconfirmation for CVE-2026-34040

Execution:
- `trivy fs --scanners vuln .`

Result:
- Exit status: `0`
- Output indicates scan completed with:
  - `Number of language-specific files num=0`
- CVE lookup:
  - No `CVE-2026-34040` match found in available Trivy JSON artifacts (`vuln-results.json`, `trivy-image-report.json`).

Conclusion: CVE-2026-34040 not detected in current FS scan context.

## Local Patch Report Artifact Check

Execution:
- `bash /projects/Charon/scripts/local-patch-report.sh`

Result:
- Generated successfully in warn mode.
- Artifacts verified:
  - `/projects/Charon/test-results/local-patch-report.md`
  - `/projects/Charon/test-results/local-patch-report.json`

## Blocking Issues

1. Playwright E2E accessibility suite has one failing security headers test (axe timeout).
2. Frontend coverage command still fails with ENOENT under `frontend/coverage/.tmp`.

## Decision

Overall DoD decision for Issue #929: FAIL

Promotion recommendation: keep blocked until both failing mandatory gates are green on rerun.
