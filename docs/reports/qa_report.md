# QA/Security DoD Audit Report — Issue #929

Date: 2026-04-21
Repository: /projects/Charon
Branch: feature/beta-release
Scope assessed: backend/internal/services/docker_service.go (SA1019 fix) and mandatory DoD QA/Security gates

## Final Recommendation

FAIL

Reason: DoD is blocked by failing mandatory gates (Playwright E2E first, frontend coverage, pre-commit version check).

## Gate Summary

| # | DoD Gate | Status | Notes |
|---|---|---|---|
| 1 | Playwright E2E first | FAIL | Mandatory e2e rebuild repeatedly failed (`docker build ... context canceled`); fallback Firefox run failed auth (`tests/auth.setup.ts` login 401) |
| 2 | GORM security scan (conditional) | PASS (proactive run) | Scope does not touch models/migrations; scanner still executed: 0 Critical, 0 High, 0 Medium, 2 Info |
| 3a | Backend coverage | PASS | `scripts/go-test-coverage.sh`: 91.6% statements, artifact `backend/coverage.txt` |
| 3b | Frontend coverage | FAIL | `scripts/frontend-test-coverage.sh`: tests pass but coverage run exits non-zero due `ENOENT frontend/coverage/.tmp/coverage-151.json` |
| 4 | Local patch coverage report | PASS | `scripts/local-patch-report.sh` exit 0; artifacts generated; patch report shows 0 changed lines / 100% |
| 5 | Frontend type check | PASS | `npm run type-check` (`tsc --noEmit`) passes |
| 6 | Pre-commit hooks | FAIL | `lefthook run pre-commit --all-files` fails at `check-version-match` (.version v0.21.0 vs latest tag v0.27.0) |
| 7a | Trivy filesystem scan | PASS | `security-scan-trivy`: 0 vulnerabilities, 0 secrets findings |
| 7b | Docker image scan | PASS | `security-scan-docker-image`: 0 Critical, 0 High, 4 Medium |
| 7c | CodeQL Go + JS | PASS | Go: 0 errors; JS: 0 errors; no blocking findings |
| 8 | Lint/build checks | PASS | `make lint-fast` 0 issues, `go build ./...` pass, `npm run build` pass, `go vet ./...` pass |

## Explicit SA1019 Verification

Status: RESOLVED

Evidence:
- `lefthook run pre-commit --all-files` output contains no `SA1019` occurrences.
- `golangci-lint-fast` passes with 0 issues.
- Root lint gate `make lint-fast` passes with 0 issues.

Conclusion: The deprecation warning SA1019 for the Issue #929 fix scope is no longer present in current lint/static analysis output.

## Detailed Evidence

### 1) Playwright E2E first

Execution path:
- Attempted mandatory rebuild: `/projects/Charon/.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean --no-cache`
- Result: repeated Docker build cancellation (`failed to solve: Canceled: context canceled`)
- Fallback run against available container:
  - `npx playwright test -c /projects/Charon/playwright.config.js --project=firefox`
  - Then with explicit base URL: `PLAYWRIGHT_BASE_URL=http://127.0.0.1:8787 ...`
  - Result: 1 failed, 722 not run
  - Failing spec: `tests/auth.setup.ts` (`authenticate`), error `Login failed: 401 - {"error":"invalid credentials"}`

Gate disposition: FAIL (mandatory first gate not passing).

### 2) GORM scan (conditional)

Execution:
- `./scripts/scan-gorm-security.sh --check`

Result:
- Exit code 0
- Critical 0, High 0, Medium 0, Info 2
- Info items are index recommendations in `backend/internal/models/user.go` (non-blocking).

Gate disposition: PASS (proactively executed for audit completeness).

### 3) Coverage gates

Backend:
- Command: `bash scripts/go-test-coverage.sh`
- Result: PASS
- Coverage: 91.6%
- Artifact: `backend/coverage.txt`

Frontend:
- Command: `bash scripts/frontend-test-coverage.sh`
- Test execution summary: 147 files passed, 5 skipped; 2035 tests passed, 90 skipped
- Final result: FAIL due unhandled rejection / filesystem error:
  - `ENOENT: no such file or directory, open '/projects/Charon/frontend/coverage/.tmp/coverage-151.json'`
- Command exit code: 1
- Artifact exists but gate is failing due non-zero exit.

Gate disposition: Backend PASS, Frontend FAIL.

### 4) Local patch coverage report

Execution:
- `bash scripts/local-patch-report.sh`

Result:
- Exit code 0
- Artifacts present:
  - `test-results/local-patch-report.md`
  - `test-results/local-patch-report.json`
- Patch summary: 0 changed lines, reported 100%

Gate disposition: PASS.

### 5) Frontend type check

Execution:
- `cd frontend && npm run type-check`

Result:
- PASS (`tsc --noEmit` exit 0)

### 6) Pre-commit hooks

Execution:
- `cd backend && lefthook run pre-commit --all-files`

Result:
- FAIL at `check-version-match`
- Failure detail: `.version` reports v0.21.0 while latest tag is v0.27.0
- Important: no SA1019 in output; fast lint/static checks are green

Gate disposition: FAIL.

### 7) Security scans

Trivy filesystem:
- Command: `/projects/Charon/.github/skills/scripts/skill-runner.sh security-scan-trivy`
- Summary table reports zero vulnerabilities and zero secrets findings for scanned manifests/text targets
- Result: PASS

Docker image scan:
- Command: `/projects/Charon/.github/skills/scripts/skill-runner.sh security-scan-docker-image`
- Result: PASS
- Vulnerability summary: Critical 0, High 0, Medium 4, Low 0

CodeQL Go:
- Command: `/projects/Charon/.github/skills/scripts/skill-runner.sh security-scan-codeql go summary`
- Result: PASS (0 errors, 0 warnings, 0 notes)

CodeQL JavaScript:
- Command: `/projects/Charon/.github/skills/scripts/skill-runner.sh security-scan-codeql javascript summary`
- Result: PASS (0 errors, 0 warnings, 0 notes)

Gotify token exposure review:
- No active tokenized URLs or secret tokens were found in generated scan outputs during this rerun.

### 8) Lint/build checks

Execution:
- `make lint-fast` (repo root) -> PASS (0 issues)
- `cd backend && go build ./...` -> PASS
- `cd frontend && npm run build` -> PASS
- `cd backend && go vet ./...` -> PASS

Gate disposition: PASS.

## Remaining Non-Blocking Issues

1. GORM scanner reports 2 informational index recommendations in `backend/internal/models/user.go` (performance-oriented, not security-blocking).
2. Docker image scan still contains 4 Medium vulnerabilities (no Critical/High).
3. Frontend test output contains warning noise (React key warning, CrowdSec query undefined warnings) but these are not current blocking gates by themselves.

## Blocking Issues

1. Playwright E2E-first gate failing:
   - e2e rebuild repeatedly canceled during Docker build, and fallback Firefox auth setup fails with 401.
2. Frontend coverage gate failing:
   - coverage post-processing aborts with missing `.tmp` coverage file (ENOENT).
3. Pre-commit gate failing:
   - `check-version-match` mismatch between `.version` and latest git tag.

## Decision

Overall DoD decision for Issue #929: FAIL

Promotion recommendation: Do not approve/merge as DoD-complete until the three blocking gates above are remediated and rerun successfully.
