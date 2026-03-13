# QA/Security Audit Report — Post-Remediation

**Date**: 2026-03-13
**Scope**: Full audit after Telegram/Slack notification remediation + zlib CVE fix
**Auditor**: QA Security Agent

---

## Gate Summary

| # | Gate | Result | Details |
|---|------|--------|---------|
| 1 | Local Patch Coverage Preflight | **PASS** | 92.3% overall (threshold: 90%) |
| 2 | Backend Unit Tests & Coverage | **PASS** | 88.1% line coverage, 0 failures |
| 3 | Frontend Unit Tests & Coverage | **PASS** | 89.73% line coverage, 0 failures |
| 4 | TypeScript Type Check | **PASS** | 0 errors |
| 5 | Pre-commit Hooks (Lefthook) | **PASS** | All 6 hooks passed |
| 6 | Trivy Filesystem Scan | **PASS** | 0 vulnerabilities, 0 secrets |
| 7 | Docker Image Scan | **PASS** (with accepted risk) | 0 Critical, 2 High (unfixable) |
| 8 | CodeQL (Go + JavaScript) | **PASS** | 0 errors, 0 warnings |
| 9 | Backend Linting (golangci-lint) | **PASS** (pre-existing) | 53 issues (all pre-existing, non-blocking) |
| 10 | GORM Security Scan | **PASS** | 0 issues (2 info-only suggestions) |
| 11 | Gotify Token Review | **PASS** | No tokens found in artifacts |

**Overall Verdict: PASS — All blocking gates cleared.**

---

## 1. Local Patch Coverage Preflight

- **Artifacts**: `test-results/local-patch-report.md`, `test-results/local-patch-report.json` — both verified
- **Overall Patch Coverage**: 92.3% (52 changed lines, 48 covered)
- **Backend Patch Coverage**: 92.3%
- **Frontend Patch Coverage**: 100.0% (0 changed lines)
- **Uncovered Lines**: 4 lines in `notification_service.go` (L462-463, L466-467) — dead code paths for Slack error formatting, accepted per remediation decision

## 2. Backend Unit Tests & Coverage

- **Test Result**: All packages passed, 0 failures
- **Statement Coverage**: 87.9%
- **Line Coverage**: 88.1% (gate: ≥87%)
- **Gate**: PASS

## 3. Frontend Unit Tests & Coverage

- **Test Result**: All 33 test suites passed
- **Statements**: 89.01%
- **Branches**: 81.21%
- **Functions**: 86.18%
- **Lines**: 89.73% (gate: ≥87%)
- **Gate**: PASS

## 4. TypeScript Type Check

- **Command**: `tsc --noEmit`
- **Result**: 0 errors
- **Gate**: PASS

## 5. Pre-commit Hooks (Lefthook)

All hooks passed (12.19s):
- check-yaml
- actionlint
- dockerfile-check
- end-of-file-fixer
- trailing-whitespace
- shellcheck

## 6. Trivy Filesystem Scan

| Target | Type | Vulnerabilities | Secrets |
|--------|------|-----------------|---------|
| backend/go.mod | gomod | 0 | — |
| frontend/package-lock.json | npm | 0 | — |
| package-lock.json | npm | 0 | — |
| playwright/.auth/user.json | text | — | 0 |

**Gate**: PASS — Zero issues

## 7. Docker Image Scan (Grype via SBOM)

### zlib CVE-2026-27171 Verification

| Package | Previous Version | Current Version | CVE Status |
|---------|-----------------|-----------------|------------|
| zlib | 1.3.1-r2 | **1.3.2-r0** | **FIXED** |

**CVE-2026-27171 is confirmed resolved.** Zero zlib-related vulnerabilities in scan results.

### Vulnerability Summary

| Severity | Count |
|----------|-------|
| Critical | 0 |
| High | 2 |
| Medium | 12 |
| Low | 3 |
| **Total** | **17** |

### High Severity (2) — No Fix Available

| CVE | Package | Version | CVSS | Status |
|-----|---------|---------|------|--------|
| CVE-2025-69650 | binutils | 2.45.1-r0 | 7.5 | No fix available — double free in readelf |
| CVE-2025-69649 | binutils | 2.45.1-r0 | 7.5 | No fix available — null pointer deref in readelf |

**Risk Acceptance**: Both `binutils` CVEs affect `readelf` processing of crafted ELF binaries. Charon does not process user-supplied ELF files; `binutils` is present as a build-time dependency in the Alpine image. Risk is accepted as non-exploitable in production context. Will be resolved when Alpine releases updated `binutils` package.

### Medium Severity (12)

| CVE | Package | Description |
|-----|---------|-------------|
| CVE-2025-13034 | curl 8.17.0-r1 | No upstream fix |
| CVE-2025-14017 | curl 8.17.0-r1 | No upstream fix |
| CVE-2025-14524 | curl 8.17.0-r1 | No upstream fix |
| CVE-2025-14819 | curl 8.17.0-r1 | No upstream fix |
| CVE-2025-15079 | curl 8.17.0-r1 | No upstream fix |
| CVE-2025-60876 | busybox 1.37.0-r30 | Affects busybox, busybox-binsh, busybox-extras, ssl_client (4 instances) |
| CVE-2025-69644 | binutils 2.45.1-r0 | No upstream fix |
| CVE-2025-69651 | binutils 2.45.1-r0 | No upstream fix |
| CVE-2025-69652 | binutils 2.45.1-r0 | No upstream fix |

### Low Severity (3)

| CVE | Package | Fix Available |
|-----|---------|---------------|
| CVE-2025-15224 | curl 8.17.0-r1 | None |
| GHSA-fw7p-63qq-7hpr | filippo.io/edwards25519 v1.1.0 | Fixed in v1.1.1 (2 instances) |

## 8. CodeQL Scans

| Language | Errors | Warnings | Notes | Files Scanned |
|----------|--------|----------|-------|---------------|
| Go | 0 | 0 | 0 | Full backend |
| JavaScript | 0 | 0 | 0 | 354/354 files |

**Gate**: PASS

## 9. Backend Linting (golangci-lint)

- **Total Issues**: 53 (all pre-existing)
  - gocritic: 50 (style suggestions)
  - gosec: 2 (G203 HTML template, G306 file permissions in test)
  - bodyclose: 1
- **Net New Issues from Remediation**: 0
- **Gate**: PASS (non-blocking, pre-existing)

## 10. GORM Security Scan

- Scanned 41 Go files (2253 lines)
- 0 Critical, 0 High, 0 Medium issues
- 2 informational suggestions only
- **Gate**: PASS

## 11. Gotify Token Review

- Scanned: grype-results.json, grype-results.sarif, sbom.cyclonedx.json, trivy reports
- No Gotify tokens or `?token=` query strings found
- **Gate**: PASS

---

## Remediation Confirmation

All 4 blockers from the previous audit are resolved:

1. **Slack unit test coverage**: 7 new tests covering 11 of 15 uncovered lines (4 accepted as dead code) — verified via 92.3% patch coverage
2. **CVE-2026-27171 (zlib)**: Fixed via `apk upgrade --no-cache zlib` in Dockerfile runtime stage — confirmed zlib 1.3.2-r0 in image, 0 zlib CVEs remaining
3. **E2E notification tests**: All 160 tests passing across Chromium/Firefox/WebKit (verified in prior run)
4. **Container rebuild**: Image rebuilt with zlib fix, scan confirms resolution
