## QA Report — Import/Save Route Regression Test Suite

- Date: 2026-03-02
- Branch: `feature/beta-release` (HEAD `2f90d936`)
- Scope: Regression test coverage for import and save function routes
- Full report: [docs/reports/qa_report_import_save_regression.md](qa_report_import_save_regression.md)

## E2E Status

- Command status provided by current PR context:
   `npx playwright test --project=chromium --project=firefox --project=webkit tests/core/caddy-import`
- Result: `106 passed, 0 failed, 0 skipped`
- Gate: PASS

## Patch Report Status

- Command: `bash scripts/local-patch-report.sh`
- Artifacts:
   - `test-results/local-patch-report.md` (present)
   - `test-results/local-patch-report.json` (present)
- Result: PASS (artifacts generated)
- Notes:
   - Warning: overall patch coverage `81.7%` below advisory threshold `90.0%`
   - Warning: backend patch coverage `81.6%` below advisory threshold `85.0%`

## Backend Coverage

- Command: `.github/skills/scripts/skill-runner.sh test-backend-coverage`
- Result: PASS
- Metrics:
   - Statement coverage: `87.5%`
   - Line coverage: `87.7%`
   - Gate threshold observed in run: `87%`

## Frontend Coverage

- Command: `.github/skills/scripts/skill-runner.sh test-frontend-coverage`
- Result: FAIL
- Failure root cause:
   - Test timeout at `frontend/src/components/__tests__/ProxyHostForm.test.tsx:1419`
   - Failing test: `maps remote docker container to remote host and public port`
   - Error: `Test timed out in 5000ms`
- Coverage snapshot produced before failure:
   - Statements: `88.95%`
   - Lines: `89.62%`
   - Functions: `86.05%`
   - Branches: `81.3%`

## Typecheck

- Command: `npm --prefix frontend run type-check`
- Result: PASS

## Pre-commit

- Command: `pre-commit run --all-files`
- Result: PASS
- Notable hooks: `golangci-lint (Fast Linters - BLOCKING)`, `Frontend TypeScript Check`, `Frontend Lint (Fix)` all passed

## Security Scans

- Trivy filesystem scan:
   - Command: `.github/skills/scripts/skill-runner.sh security-scan-trivy`
   - Result: PASS
   - Critical/High findings: `0/0`

- Docker image scan:
   - Command: `.github/skills/scripts/skill-runner.sh security-scan-docker-image`
   - Result: PASS
   - Critical/High findings: `0/0`
   - Additional findings: `10 medium`, `3 low` (non-blocking)

## Remediation Required Before Merge

1. Stabilize the timed-out frontend test at `frontend/src/components/__tests__/ProxyHostForm.test.tsx:1419`.
2. Re-run `.github/skills/scripts/skill-runner.sh test-frontend-coverage` until the suite is fully green.
3. Optional quality improvement: raise patch coverage warnings (`81.7%` overall, `81.6%` backend) with targeted tests on uncovered changed lines from `test-results/local-patch-report.md`.

## Final Merge Recommendation

- Recommendation: **NO-GO**
- Reason: Required frontend coverage gate did not pass due to a deterministic test timeout.
