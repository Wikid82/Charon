# QA/Security Validation Report - Flaky Certificate Test Fix

**Date:** 2026-02-19
**Scope:** Validation/audit gates for flaky-test fix in certificate handler/service paths.

## Gate Summary

| Gate | Status | Evidence |
|---|---|---|
| 1) Playwright E2E certificates gate | **PASS** | Task: `Test: E2E Playwright (FireFox) - Core: Certificates`; status file: `test-results/.last-run.json` (`passed`) |
| 1b) Durable flaky artifacts in `test-results/flaky/` | **PASS** | `cert-list-stability.jsonl`, `cert-list-race.jsonl`, `cert-db-setup-ordering.jsonl`, `cert-handler-regression.jsonl` |
| 2) Local patch preflight artifacts present | **PASS (warn-mode)** | `test-results/local-patch-report.md`, `test-results/local-patch-report.json` |
| 3) Backend coverage gate >=85% | **PASS** | `test-backend-coverage` rerun with valid `CHARON_ENCRYPTION_KEY`; line coverage `87.3%`, statements `87.0%` |
| 4) Pre-commit all files | **PASS** | Task: `Lint: Pre-commit (All Files)` -> all hooks passed |
| 5a) Trivy filesystem scan | **PASS** | Task: `Security: Trivy Scan` -> 0 vulnerabilities, 0 secrets |
| 5b) Docker image scan | **FAIL** | Task: `Security: Scan Docker Image (Local)` -> 1 High, 9 Medium, 1 Low |
| 5c) CodeQL Go CI-aligned | **PASS** | Task: `Security: CodeQL Go Scan (CI-Aligned) [~60s]` completed |
| 5d) CodeQL JS CI-aligned | **PASS** | Task: `Security: CodeQL JS Scan (CI-Aligned) [~90s]` completed |
| 5e) CodeQL high/critical findings gate | **PASS** | `pre-commit run --hook-stage manual codeql-check-findings --all-files` |
| 6) Lint/type checks relevant to scope | **PASS** | `Lint: Staticcheck (Fast)` passed; `Lint: TypeScript Check` passed |
| 7) Flaky loop thresholds from plan | **PASS** | stability=100, race=30, dbordering=50, raceWarnings=0, noSuchTable=0 |

## Detailed Evidence

### 1) Playwright Certificates Gate

- Executed task: `Test: E2E Playwright (FireFox) - Core: Certificates`
- Base URL: `http://127.0.0.1:8080`
- Result marker: `test-results/.last-run.json`:

```json
{
  "status": "passed",
  "failedTests": []
}
```

### 2) Local Patch Preflight

- Executed task: `Test: Local Patch Report`
- Artifacts exist:
  - `test-results/local-patch-report.md`
  - `test-results/local-patch-report.json`
- Current mode: `warn`
- Warning recorded: missing frontend coverage input (`frontend/coverage/lcov.info`)

### 3) Backend Coverage

- Task invocation failed initially due missing `CHARON_ENCRYPTION_KEY`.
- Rerun with valid env key:
  - `CHARON_ENCRYPTION_KEY="$(openssl rand -base64 32)" .github/skills/scripts/skill-runner.sh test-backend-coverage`
- Final result:
  - `Coverage gate: PASS`
  - `Line coverage: 87.3%`
  - `Statement coverage: 87.0%`

### 4) Pre-commit

- Executed task: `Lint: Pre-commit (All Files)`
- Result: all configured hooks passed, including:
  - yaml checks
  - shellcheck
  - actionlint
  - go vet
  - golangci-lint fast
  - frontend type check and lint fix hooks

### 5) Security Scans

#### Trivy Filesystem

- Executed task: `Security: Trivy Scan`
- Summary:
  - `backend/go.mod`: 0 vulnerabilities
  - `frontend/package-lock.json`: 0 vulnerabilities
  - `package-lock.json`: 0 vulnerabilities
  - secrets: 0

#### Docker Image Scan (Grype via skill)

- Executed task: `Security: Scan Docker Image (Local)`
- Artifacts generated:
  - `sbom.cyclonedx.json`
  - `grype-results.json`
  - `grype-results.sarif`
- Summary from `grype-results.json`:
  - High: 1
  - Medium: 9
  - Low: 1
  - Critical: 0

#### CodeQL

- Go CI-aligned task completed and generated `codeql-results-go.sarif`.
- JS CI-aligned task completed and generated `codeql-results-js.sarif`.
- Manual findings gate:
  - `pre-commit run --hook-stage manual codeql-check-findings --all-files`
  - result: no HIGH/CRITICAL findings in Go or JS.

### 6) Linting/Type Checks

- `Lint: Staticcheck (Fast)` -> `0 issues`
- `Lint: TypeScript Check` -> `tsc --noEmit` passed

### 7) Flaky-Specific Loop Artifacts and Thresholds

- Artifacts in `test-results/flaky/`:
  - `cert-list-stability.jsonl`
  - `cert-list-race.jsonl`
  - `cert-db-setup-ordering.jsonl`
  - `cert-handler-regression.jsonl`

- Measured thresholds:
  - `stability=100` (expected 100)
  - `race=30` (expected 30)
  - `dbordering=50` (expected 50)
  - `raceWarnings=0`
  - `noSuchTable=0`

## Filesystem vs Image Findings Comparison

- Filesystem scan (Trivy): **0 vulnerabilities**.
- Image scan (Grype): **11 vulnerabilities**.
- **Additional image-only vulnerabilities:** 11

Image-only findings:

| Severity | ID | Package | Version | Fix |
|---|---|---|---|---|
| High | GHSA-69x3-g4r3-p962 | github.com/slackhq/nebula | v1.9.7 | 1.10.3 |
| Medium | CVE-2025-60876 | busybox | 1.37.0-r30 | N/A |
| Medium | CVE-2025-60876 | busybox-binsh | 1.37.0-r30 | N/A |
| Medium | CVE-2025-60876 | busybox-extras | 1.37.0-r30 | N/A |
| Medium | CVE-2025-60876 | ssl_client | 1.37.0-r30 | N/A |
| Medium | CVE-2025-14819 | curl | 8.17.0-r1 | N/A |
| Medium | CVE-2025-13034 | curl | 8.17.0-r1 | N/A |
| Medium | CVE-2025-14524 | curl | 8.17.0-r1 | N/A |
| Medium | CVE-2025-15079 | curl | 8.17.0-r1 | N/A |
| Medium | CVE-2025-14017 | curl | 8.17.0-r1 | N/A |
| Low | CVE-2025-15224 | curl | 8.17.0-r1 | N/A |

## Failed Gates and Remediation

### Failed Gate: Security Docker Image Scan

- Failing evidence: image scan task ended with non-zero exit due vulnerability policy (`1 High`).
- Primary blocker: `GHSA-69x3-g4r3-p962` in `github.com/slackhq/nebula@v1.9.7` (fix `1.10.3`).

Recommended remediation:

1. Update dependency chain to a version resolving `nebula >= 1.10.3` (or update parent component that pins it).
2. Rebuild image and rerun:
   - `Security: Scan Docker Image (Local)`
   - `Security: Trivy Scan`
3. If immediate upgrade is not feasible, document/renew security exception with review date and compensating controls.

### Warning (Non-blocking for requested artifact-presence check): Local Patch Preflight

- Current warning: missing frontend coverage input `frontend/coverage/lcov.info`.
- Artifacts are present and valid for preflight evidence.

Recommended remediation:

1. Generate frontend coverage (`test-frontend-coverage`) to populate `frontend/coverage/lcov.info`.
2. Re-run `Test: Local Patch Report` to remove warn-mode status.

## Final Verdict

- **Overall QA/Security Result: FAIL** (blocked by Docker image security gate).
- All non-image gates requested for flaky-fix validation passed or produced required artifacts.
