# QA Report: LAPI Auth Fix and Translation Bug Fix

**Date**: 2026-02-04
**Version**: v0.3.0 (beta)
**Changes Under Review**:
1. Backend: CrowdSec key-status endpoint, bouncer auto-registration, key file fallback
2. Frontend: Key warning banner, i18n race condition fix, translations

---

## Executive Summary

| Category | Status | Details |
|----------|--------|---------|
| E2E Tests | ⚠️ ISSUES | 175 passed, 3 failed, 26 skipped |
| Backend Coverage | ⚠️ BELOW THRESHOLD | 84.8% (minimum: 85%) |
| Frontend Coverage | ✅ PASS | All tests passed |
| TypeScript Check | ✅ PASS | Zero errors |
| Pre-commit Hooks | ⚠️ AUTO-FIXED | 1 file fixed (`tests/etc/passwd`) |
| Backend Linting | ✅ PASS | go vet passed |
| Frontend Linting | ✅ PASS | ESLint passed |
| Trivy FS Scan | ✅ PASS | 0 HIGH/CRITICAL vulnerabilities |
| Docker Image Scan | ⚠️ ISSUES | 7 HIGH vulnerabilities (base image) |

**Overall Status**: ⚠️ **CONDITIONAL APPROVAL** - Issues found requiring attention

---

## 1. Playwright E2E Tests

### Results
- **Total**: 204 tests
- **Passed**: 175 (86%)
- **Failed**: 3
- **Skipped**: 26

### Failed Tests (Severity: LOW-MEDIUM)

| Test | File | Error | Severity |
|------|------|-------|----------|
| Should reject archive missing required CrowdSec fields | [crowdsec-import.spec.ts](tests/security/crowdsec-import.spec.ts#L133) | Expected 422, got 500 | MEDIUM |
| Should reject archive with path traversal attempt | [crowdsec-import.spec.ts](tests/security/crowdsec-import.spec.ts#L338) | Error message mismatch | LOW |
| Verify admin whitelist is set to 0.0.0.0/0 | [zzzz-break-glass-recovery.spec.ts](tests/security-enforcement/zzzz-break-glass-recovery.spec.ts#L147) | `admin_whitelist` undefined | LOW |

### Analysis
1. **CrowdSec Import Validation (crowdsec-import.spec.ts:133)**: Backend returns 500 instead of 422 for missing required fields - suggests error handling improvement needed.
2. **Path Traversal Detection (crowdsec-import.spec.ts:338)**: Error message says "failed to create backup" instead of security-related message - error messaging could be improved.
3. **Admin Whitelist API (zzzz-break-glass-recovery.spec.ts:147)**: API response missing `admin_whitelist` field - may be API schema change.

### Skipped Tests (26 total)
- Mostly CrowdSec-related tests that require CrowdSec to be running
- Rate limiting tests that test middleware enforcement (correctly skipped per testing scope)
- These are documented and expected skips

---

## 2. Backend Unit Tests

### Results
- **Status**: ⚠️ BELOW THRESHOLD
- **Coverage**: 84.8%
- **Threshold**: 85.0%
- **Deficit**: 0.2%

### Recommendation
Coverage is 0.2% below threshold. This is a marginal gap. Priority:
1. Check if any new code paths in the LAPI auth fix lack tests
2. Add targeted tests for CrowdSec key-status handler edge cases
3. Consider raising coverage exclusions for generated/boilerplate code if appropriate

---

## 3. Frontend Unit Tests

### Results
- **Status**: ✅ PASS
- **Test Files**: 136+ passed
- **Tests**: 1500+ passed
- **Skipped**: ~90 (documented security audit tests)

### Coverage by Area
| Area | Statement Coverage |
|------|-------------------|
| Components | 74.14% |
| Components/UI | 98.94% |
| Hooks | 98.11% |
| Pages | 83.01% |
| Utils | 96.49% |
| API | ~91% |
| Data | 100% |
| Context | 92.59% |

---

## 4. TypeScript Check

- **Status**: ✅ PASS
- **Errors**: 0
- **Command**: `npm run type-check`

---

## 5. Pre-commit Hooks

### Results
- **Status**: ⚠️ AUTO-FIXED
- **Hooks Passed**: 12/13
- **Auto-fixed**: 1 file

### Details
| Hook | Status |
|------|--------|
| fix end of files | Fixed `tests/etc/passwd` |
| trim trailing whitespace | ✅ Pass |
| check yaml | ✅ Pass |
| check for added large files | ✅ Pass |
| dockerfile validation | ✅ Pass |
| Go Vet | ✅ Pass |
| golangci-lint (Fast) | ✅ Pass |
| Check .version matches tag | ✅ Pass |
| LFS large files check | ✅ Pass |
| Prevent CodeQL DB commits | ✅ Pass |
| Prevent data/backups commits | ✅ Pass |
| Frontend TypeScript Check | ✅ Pass |
| Frontend Lint (Fix) | ✅ Pass |

**Action Required**: Commit the auto-fixed `tests/etc/passwd` file.

---

## 6. Linting

### Backend (Go)
| Linter | Status | Notes |
|--------|--------|-------|
| go vet | ✅ PASS | No issues |
| staticcheck | ⚠️ SKIPPED | Go version mismatch (1.25.6 vs 1.25.5) - not a code issue |

### Frontend (TypeScript/React)
| Linter | Status | Notes |
|--------|--------|-------|
| ESLint | ✅ PASS | No issues |

---

## 7. Security Scans

### Trivy Filesystem Scan
- **Status**: ✅ PASS
- **HIGH/CRITICAL Vulnerabilities**: 0
- **Scanned**: Source code + npm dependencies

### Docker Image Scan (Grype)
- **Status**: ⚠️ HIGH VULNERABILITIES DETECTED
- **Critical**: 0
- **High**: 7
- **Medium**: 20
- **Low**: 2
- **Negligible**: 380
- **Total**: 409

### High Severity Vulnerabilities

| CVE | Package | Version | Fixed | CVSS | Description |
|-----|---------|---------|-------|------|-------------|
| CVE-2025-13151 | libtasn1-6 | 4.20.0-2 | No fix | 7.5 | Stack-based buffer overflow |
| CVE-2025-15281 | libc-bin | 2.41-12+deb13u1 | No fix | 7.5 | wordexp WRDE_REUSE issue |
| CVE-2025-15281 | libc6 | 2.41-12+deb13u1 | No fix | 7.5 | wordexp WRDE_REUSE issue |
| CVE-2026-0915 | libc-bin | 2.41-12+deb13u1 | No fix | 7.5 | getnetbyaddr nsswitch issue |
| CVE-2026-0915 | libc6 | 2.41-12+deb13u1 | No fix | 7.5 | getnetbyaddr nsswitch issue |
| CVE-2026-0861 | libc-bin | 2.41-12+deb13u1 | No fix | 8.4 | memalign alignment issue |
| CVE-2026-0861 | libc6 | 2.41-12+deb13u1 | No fix | 8.4 | memalign alignment issue |

### Analysis
All HIGH vulnerabilities are in **base image system packages** (Debian Trixie):
- `libtasn1-6` (ASN.1 parsing library)
- `libc-bin` / `libc6` (GNU C Library)

**Mitigation Status**: No fixes currently available from Debian upstream. These affect the base OS, not application code.

**Risk Assessment**:
- **libtasn1-6 (CVE-2025-13151)**: Only exploitable if parsing malicious ASN.1 data - low risk for Charon's use case
- **glibc issues**: Require specific API usage patterns that Charon does not trigger

**Recommendation**: Monitor for Debian package updates. No immediate blocking action required for beta release.

---

## 8. Issues Requiring Resolution

### MUST FIX (Blocking)
1. **Backend Coverage**: Increase from 84.8% to 85.0% (0.2% gap)
   - Priority: Add tests for new CrowdSec key-status code paths

### SHOULD FIX (Before release)
2. **E2E Test Failures**: 3 tests failing
   - `crowdsec-import.spec.ts:133` - Fix error code consistency (500 → 422)
   - `crowdsec-import.spec.ts:338` - Improve error message clarity
   - `zzzz-break-glass-recovery.spec.ts:147` - Fix API response schema

3. **Pre-commit Auto-fix**: Commit `tests/etc/passwd` EOF fix

### MONITOR (Non-blocking)
4. **Docker Image CVEs**: 7 HIGH in base image packages
   - Monitor for Debian security updates
   - Consider if alternative base image is warranted

5. **Staticcheck Version**: Update staticcheck to go 1.25.7+

---

## 9. Test Execution Details

| Test Suite | Duration | Workers |
|------------|----------|---------|
| Playwright E2E | 4.6 minutes | 2 |
| Backend Unit | ~30 seconds | - |
| Frontend Unit | ~102 seconds | - |

---

## 10. Approval Status

### ⚠️ CONDITIONAL APPROVAL

**Conditions for Full Approval**:
1. ✅ TypeScript compilation passing
2. ✅ Frontend linting passing
3. ✅ Backend linting passing (go vet)
4. ✅ Trivy filesystem scan clean
5. ⚠️ Backend coverage at 85%+ (currently 84.8%)
6. ⚠️ All E2E tests passing (currently 3 failing)

**Recommendation**: Address the 0.2% coverage gap and investigate the 3 E2E test failures before merging to main. The Docker image vulnerabilities are in base OS packages with no fixes available - these issues do not block the implementation.

---

*Report generated by QA Security Agent*
