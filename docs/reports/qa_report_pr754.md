# QA Report — PR #754: Enable and Test Gotify and Custom Webhook Notifications

**Branch:** `feature/beta-release`
**Date:** 2026-02-25
**Auditor:** QA Security Agent

---

## Summary

| # | Check | Result | Details |
|---|-------|--------|---------|
| 1 | Local Patch Coverage Preflight | **WARN** | 79.5% overall (threshold 90%), 78.3% backend (threshold 85%) — advisory only |
| 2 | Backend Coverage ≥ 85% | **PASS** | 87.0% statement / 87.3% line (threshold 87%) |
| 3 | Frontend Coverage ≥ 85% | **PASS** | 88.21% statement / 88.97% line (threshold 85%) |
| 4 | TypeScript Type Check | **PASS** | Zero errors |
| 5 | Pre-commit Hooks | **PASS** | All 15 hooks passed |
| 6a | Trivy Filesystem Scan | **PASS** | 0 CRITICAL/HIGH in project code (findings only in Go module cache) |
| 6b | Docker Image Scan | **WARN** | 1 HIGH in Caddy transitive dep (CVE-2026-25793, nebula v1.9.7 → fixed 1.10.3) |
| 6c | CodeQL (Go + JavaScript) | **PASS** | 0 errors, 0 warnings across both languages |
| 7 | GORM Security Scan | **PASS** | 0 CRITICAL/HIGH (2 INFO suggestions: missing indexes on UserPermittedHost) |
| 8 | Go Vulnerability Check | **PASS** | No vulnerabilities found |

---

## Detailed Findings

### 1. Local Patch Coverage Preflight

- **Status:** WARN (advisory, not blocking per policy)
- Overall patch coverage: **79.5%** (threshold: 90%)
- Backend patch coverage: **78.3%** (threshold: 85%)
- Artifacts generated but `test-results/` directory was not persisted at repo root
- **Action:** Consider adding targeted tests for uncovered changed lines in notification service/handler

### 2. Backend Unit Test Coverage

- **Status:** PASS
- Statement coverage: **87.0%**
- Line coverage: **87.3%**
- All tests passed (0 failures)

### 3. Frontend Unit Test Coverage

- **Status:** PASS
- Statement coverage: **88.21%**
- Branch coverage: **80.58%**
- Function coverage: **85.20%**
- Line coverage: **88.97%**
- All tests passed (0 failures)
- Coverage files generated: `lcov.info`, `coverage-summary.json`, `coverage-final.json`

### 4. TypeScript Type Check

- **Status:** PASS
- `tsc --noEmit` completed with zero errors

### 5. Pre-commit Hooks

- **Status:** PASS
- All hooks passed:
  - fix end of files
  - trim trailing whitespace
  - check yaml
  - check for added large files
  - shellcheck
  - actionlint (GitHub Actions)
  - dockerfile validation
  - Go Vet
  - golangci-lint (Fast Linters - BLOCKING)
  - Check .version matches latest Git tag
  - Prevent large files not tracked by LFS
  - Prevent committing CodeQL DB artifacts
  - Prevent committing data/backups files
  - Frontend TypeScript Check
  - Frontend Lint (Fix)

### 6a. Trivy Filesystem Scan

- **Status:** PASS
- Scanned `backend/` and `frontend/` directories: **0 CRITICAL, 0 HIGH**
- Full workspace scan found 3 CRITICAL + 14 HIGH across Go module cache dependencies (not project code)
- Trivy misconfig scanner crashed (known Trivy bug in ansible parser — nil pointer dereference in `discovery.go:82`). Vuln scanner completed successfully.

### 6b. Docker Image Scan

- **Status:** WARN (not blocking — upstream dependency)
- Image: `charon:local`
- **1 HIGH finding:**
  - **CVE-2026-25793** — `github.com/slackhq/nebula` v1.9.7 (in `usr/bin/caddy` binary)
  - Description: Blocklist evasion via ECDSA Signature Malleability
  - Fixed in: v1.10.3
  - Impact: Caddy transitive dependency, not Charon code
- **Remediation:** Upgrade Caddy to a version that pulls nebula ≥ 1.10.3 when available

### 6c. CodeQL Scans

- **Status:** PASS
- **Go:** 0 errors, 0 warnings
- **JavaScript:** 0 errors, 0 warnings (347/347 files scanned)
- SARIF outputs: `codeql-results-go.sarif`, `codeql-results-javascript.sarif`

### 7. GORM Security Scan

- **Status:** PASS
- Scanned: 41 Go files (2207 lines), 2 seconds
- **0 CRITICAL, 0 HIGH, 0 MEDIUM**
- 2 INFO suggestions:
  - `backend/internal/models/user.go:109` — `UserPermittedHost.UserID` missing index
  - `backend/internal/models/user.go:110` — `UserPermittedHost.ProxyHostID` missing index

### 8. Go Vulnerability Check

- **Status:** PASS
- `govulncheck ./...` — No vulnerabilities found

---

## Gotify Token Security Review

- No Gotify tokens found in logs, test artifacts, or API examples
- No tokenized URL query parameters exposed in diagnostics or output
- Token handling follows `json:"-"` pattern (verified via `HasToken` computed field approach in PR)

---

## Recommendation

### GO / NO-GO: **GO** (conditional)

All blocking gates pass. Two advisory warnings exist:

1. **Patch coverage** (79.5% overall, 78.3% backend) is below advisory thresholds but not a blocking gate per current policy
2. **Docker image** has 1 HIGH CVE in Caddy's transitive dependency (nebula) — upstream fix required, not actionable in Charon code

**Conditions:**
- Track nebula CVE-2026-25793 remediation as a follow-up issue when a Caddy update incorporates the fix
- Consider adding targeted tests for uncovered changed lines in notification service/handler to improve patch coverage
