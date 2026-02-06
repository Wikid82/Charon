# QA & Security Report

**Date:** 2026-02-06
**Status:** 🔴 FAILED
**Evaluator:** GitHub Copilot (QA Security Mode)

## Executive Summary

The codebase currently **fails** to meet the strict QA thresholds defined for the project. While the backend binaries are secure, the frontend falls short of coverage targets, contains type errors, and leaks sensitive tokens in test artifacts.

| Check | Status | Details |
| :--- | :--- | :--- |
| **Frontend Coverage** | 🔴 FAIL | **87.25%** (Threshold: 87.5%) |
| **Type Check** | 🔴 FAIL | **13 Errors** in test files |
| **Pre-commit** | 🔴 FAIL | Blocked by Type Check & Formatting |
| **Filesystem Security** | 🟠 WARN | Secrets in Playwright artifacts |
| **Container Security** | 🟠 WARN | 2 HIGH CVEs in Base Image (Debian) |

---

## 1. Frontend Quality

### Coverage Gap
**Target:** 87.5% | **Actual:** 87.25% (Statements)

The following files have 0% or low coverage and require immediate attention:
- `src/api/client.ts`
- `src/components/CrowdSecKeyWarning.tsx`
- `src/locales/*`

**Metrics:**
- **Statements**: 87.25% (❌ < 87.5%)
- **Branches**: 78.87%
- **Functions**: 84.32%
- **Lines**: 87.25%

### Type Safety Violations
**Total Errors:** 13
**Primary Cause:** Outdated mock objects in tests missing fields added to the source types.

**Key Failures:**
1.  **`AccessListForm.test.tsx`**: `ProxyHost` mock missing `meta` field.
2.  **`DNSProviderForm.test.tsx`**: Unused variables (`fireEvent`, `within`).
3.  **`CrowdSecConfig.test.tsx`**: Type mismatch on `id` (expected number, got string).
4.  **`metrics.test.ts`**: Missing `uuid` in mock metrics object.

**Remediation:**
Run `npm run type-check` in `frontend/` and update test mocks to match strict `tsconfig.json`.

---

## 2. Security Findings

### Container Security (`charon:local`)
**Base Image:** Debian 13.3 (Trixie)
**Binaries:** All Go binaries (`charon`, `caddy`, `crowdsec`) are **CLEAN**.

**Vulnerabilities:**
| Library | CVE | Severity | Description |
| :--- | :--- | :--- | :--- |
| `libc-bin` / `libc6` | CVE-2026-0861 | **HIGH** | Integer overflow in memalign (Heap Corruption) |

*Action*: Inspect upstream Debian updates or consider switching to `distroless` or `alpine` if Debian patches are delayed.

### Filesystem Scan (Trivy)

**🔴 ACTION REQUIRED: Secrets Detected**
- **File:** `playwright/.auth/user.json`
- **Finding:** Generic High Entropy Secret (JWT Token)
- **Remediation:** Add `playwright/.auth/` to `.gitignore` and `.trivyignore`. Revoke token if used in production environment.

**⚪ Low Priority: CodeQL Artifacts**
- Multiple vulnerabilities detected in `backend/codeql-custom-queries-go/ql/test/`. These are test fixtures and can be safely ignored.

---

## 3. Pre-commit Status
The `pre-commit` hook pipeline failed.
- **Hook:** `frontend-type-check`
- **Result:** Failed (Exit code 2)
- **Hook:** `trailing-whitespace`
- **Result:** Failed (Files were modified/fixed automatically)

## 4. Recommendations

1.  **Immediate Fix**: Update `frontend/src/setupTests.ts` or individual test files to fix the 13 Type Errors.
2.  **Coverage Push**: Add a single unit test for `CrowdSecKeyWarning.tsx` to push coverage over 87.5%.
3.  **Security Hygiene**:
    - Add `playwright/.auth/` to `.gitignore`.
    - Run `trivy clean --all`.
4.  **CI/CD**: Do not merge PRs until Frontend Coverage > 87.5% and Type Check passes.
