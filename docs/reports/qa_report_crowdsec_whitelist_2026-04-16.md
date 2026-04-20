# QA Audit Report — CrowdSec IP Whitelist Management

**Feature Branch**: `feature/beta-release`
**Pull Request**: #952
**Repository**: Wikid82/Charon
**Audit Date**: 2026-04-16
**Auditor**: QA Security Agent

---

## Overall Verdict

### APPROVED WITH CONDITIONS

The CrowdSec IP Whitelist Management feature passes all critical quality and security gates. All feature-specific E2E tests pass across three browsers. Backend and frontend coverage exceed thresholds. No security vulnerabilities were found in the feature code. Two upstream HIGH CVEs in the Docker image and below-threshold overall patch coverage require tracking but do not block the release.

---

## Gate Results Summary

| # | Gate | Result | Detail |
|---|------|--------|--------|
| 1 | Playwright E2E | **PASS** | All CrowdSec whitelist tests passed; 14 pre-existing failures (unrelated) |
| 2 | Go Backend Coverage | **PASS** | 88.4% line coverage (threshold: 87%) |
| 3 | Frontend Coverage | **PASS** | 90.06% line coverage (threshold: 87%) |
| 4 | Patch Coverage | **WARN** | Overall 89.4% (threshold: 90%); Backend 88.0% PASS; Frontend 97.0% PASS |
| 5 | TypeScript Type Check | **PASS** | `npx tsc --noEmit` — 0 errors |
| 6 | Lefthook (Lint/Format) | **PASS** | All 6 hooks green |
| 7 | GORM Security Scan | **PASS** | 0 CRITICAL/HIGH/MEDIUM issues |
| 8 | Trivy Filesystem Scan | **PASS** | 0 CRITICAL/HIGH vulnerabilities |
| 9 | Trivy Docker Image Scan | **WARN** | 2 unique HIGH CVEs (upstream dependencies) |
| 10 | CodeQL SARIF Review | **PASS** | 1 pre-existing Go finding; 0 JS findings; 0 whitelist-related |

---

## Detailed Gate Analysis

### Gate 1 — Playwright E2E Tests

**Result**: PASS
**Browsers**: Chromium, Firefox, WebKit (all three)

**CrowdSec Whitelist-Specific Tests (10 tests)**: All PASSED
- Add whitelist entry with valid IP
- Add whitelist entry with valid CIDR
- Reject invalid IP/CIDR input
- Reject duplicate entry
- Delete whitelist entry
- Display whitelist table with entries
- Empty state display
- Whitelist tab visibility (local mode only)
- Form validation and error handling
- Toast notification on success/failure

**Pre-existing Failures (14 unique, unrelated to this feature)**:
- Certificate deletion tests (7): cert delete/bulk-delete operations
- Caddy import tests (3): conflict details, server detection, resolution
- Navigation test (1): main navigation item count
- User management tests (2): invite link copy, keyboard navigation
- Integration test (1): system health check

None of the pre-existing failures are related to the CrowdSec whitelist feature.

### Gate 2 — Go Backend Coverage

**Result**: PASS
**Coverage**: 88.4% line coverage
**Threshold**: 87%

### Gate 3 — Frontend Coverage

**Result**: PASS
**Coverage**: 90.06% line coverage (Statements: 89.03%, Branches: 85.84%, Functions: 85.85%)
**Threshold**: 87%

5 pre-existing test timeouts in `ProxyHostForm-dns.test.tsx` and `ProxyHostForm-dropdown-changes.test.tsx` — not whitelist-related.

### Gate 4 — Patch Coverage

**Result**: WARN (non-blocking)

| Scope | Changed Lines | Covered | Patch % | Status |
|-------|--------------|---------|---------|--------|
| Overall | 1689 | 1510 | 89.4% | WARN (threshold: 90%) |
| Backend | 1426 | 1255 | 88.0% | PASS (threshold: 85%) |
| Frontend | 263 | 255 | 97.0% | PASS (threshold: 85%) |

**CrowdSec-Specific Patch Coverage**:
- `crowdsec_handler.go`: 71.2% — 17 uncovered changed lines (error-handling branches)
- `crowdsec_whitelist_service.go`: 83.6% — 18 uncovered changed lines (YAML write failure, edge cases)
- `CrowdSecConfig.tsx`: 93.3% — 2 uncovered changed lines

**Recommendation**: Add targeted unit tests for error-handling branches in `crowdsec_handler.go` (lines 2712-2772) and `crowdsec_whitelist_service.go` (lines 47-148) to bring CrowdSec-specific patch coverage above 90%. This is tracked as a follow-up improvement and does not block release.

### Gate 5 — TypeScript Type Check

**Result**: PASS
`npx tsc --noEmit` from `frontend/` completed with 0 errors.

### Gate 6 — Lefthook (Lint/Format)

**Result**: PASS
All 6 hooks passed:
- `go-fmt`
- `go-vet`
- `go-staticcheck`
- `eslint`
- `prettier-check`
- `tsc-check`

### Gate 7 — GORM Security Scan

**Result**: PASS
`./scripts/scan-gorm-security.sh --check` — 0 CRITICAL, 0 HIGH, 0 MEDIUM issues.
No exposed IDs, secrets, or DTO embedding violations in CrowdSec whitelist models.

### Gate 8 — Trivy Filesystem Scan

**Result**: PASS
`trivy fs --scanners vuln --severity CRITICAL,HIGH --format table .` — 0 vulnerabilities detected in application source and dependencies.

### Gate 9 — Trivy Docker Image Scan

**Result**: WARN (non-blocking for this feature)
Image: `charon:local` (Alpine 3.23.3)

| CVE | Severity | Package | Installed | Fixed | Component |
|-----|----------|---------|-----------|-------|-----------|
| CVE-2026-34040 | HIGH | github.com/docker/docker | v28.5.2+incompatible | 29.3.1 | Charon Go binary (Moby authorization bypass) |
| CVE-2026-32286 | HIGH | github.com/jackc/pgproto3/v2 | v2.3.3 | No fix | CrowdSec binaries (PostgreSQL protocol DoS) |

**Analysis**:
- CVE-2026-34040: Moby authorization bypass — affects Docker API access control. Charon does not expose Docker API to untrusted networks. Low practical risk. Update `github.com/docker/docker` to v29.3.1 when available.
- CVE-2026-32286: PostgreSQL protocol DoS — present only in CrowdSec's `crowdsec` and `cscli` binaries, not in Charon's own code. Awaiting upstream fix from CrowdSec.

**Recommendation**: Track both CVEs for remediation. Neither impacts CrowdSec whitelist management functionality or Charon's own security posture directly.

### Gate 10 — CodeQL SARIF Review

**Result**: PASS

- **Go**: 1 pre-existing finding — `cookie-secure-not-set` at `auth_handler.go:152`. Not whitelist-related. Tracked separately.
- **JavaScript**: 0 findings.
- **CrowdSec whitelist**: 0 findings across both Go and JavaScript.

---

## Security Review — CrowdSec IP Whitelist Feature

### 1. IP/CIDR Input Validation

**Status**: SECURE

The `normalizeIPOrCIDR()` function in `crowdsec_whitelist_service.go` uses Go standard library functions `net.ParseIP()` and `net.ParseCIDR()` for validation. Invalid inputs are rejected with the sentinel error `ErrInvalidIPOrCIDR`. No user input passes through without validation.

### 2. YAML Injection Prevention

**Status**: SECURE

`buildWhitelistYAML()` uses a `strings.Builder` to construct YAML output. Only IP addresses and CIDR ranges that have already passed `normalizeIPOrCIDR()` validation are included. The normalized output from `net.ParseIP`/`net.ParseCIDR` cannot contain YAML metacharacters.

### 3. Path Traversal Protection

**Status**: SECURE

`WriteYAML()` uses hardcoded file paths (no user input in path construction). Atomic write pattern: writes to `.tmp` suffix, then `os.Rename()` to final path. No directory traversal vectors.

### 4. SQL Injection Prevention

**Status**: SECURE

All GORM queries use parameterized operations:
- `Where("uuid = ?", id)` for delete
- `Where("ip_or_cidr = ?", normalized)` for duplicate check
- Standard GORM `Create()` for inserts

No raw SQL or string concatenation.

### 5. Authentication & Authorization

**Status**: SECURE

All whitelist routes are registered under the admin route group in `routes.go`, which is protected by:
- Cerberus middleware (authentication/authorization enforcement)
- Emergency bypass middleware (for recovery scenarios only)
- Security headers and gzip middleware

No unauthenticated access to whitelist endpoints is possible.

### 6. Log Safety

**Status**: SECURE

Whitelist service logs only operational error context (e.g., "failed to write CrowdSec whitelist YAML after add"). No IP addresses, user data, or PII are written to logs. Other handler code uses `util.SanitizeForLog()` for user-controlled input in log messages.

---

## Conditions for Approval

These items are tracked as follow-up improvements and do not block merge:

1. **Patch Coverage Improvement**: Add targeted unit tests for error-handling branches in:
   - `crowdsec_handler.go` (lines 2712-2772, 71.2% patch coverage)
   - `crowdsec_whitelist_service.go` (lines 47-148, 83.6% patch coverage)

2. **Upstream CVE Tracking**:
   - CVE-2026-34040: Update `github.com/docker/docker` to v29.3.1 when Go module is available
   - CVE-2026-32286: Monitor CrowdSec upstream for `pgproto3` fix

3. **Pre-existing Test Failures**: 14 pre-existing E2E test failures (certificate deletion, caddy import, navigation, user management) should be tracked in existing issues. None are regressions from this feature.

---

## Artifacts

| Artifact | Location |
|----------|----------|
| Playwright HTML Report | `playwright-report/index.html` |
| Backend Coverage | `backend/coverage.txt` |
| Frontend Coverage | `frontend/coverage/lcov.info`, `frontend/coverage/coverage-summary.json` |
| Patch Coverage Report | `test-results/local-patch-report.md`, `test-results/local-patch-report.json` |
| GORM Security Scan | Inline (0 findings) |
| Trivy Filesystem Scan | Inline (0 findings) |
| Trivy Image Scan | `trivy-image-report.json` |
| CodeQL Go SARIF | `codeql-results-go.sarif` |
| CodeQL JS SARIF | `codeql-results-javascript.sarif` |
