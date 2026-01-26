# Security Module Testing Infrastructure - QA Audit Report

**Date:** 2026-01-25
**Auditor:** GitHub Copilot (Claude Opus 4.5)
**Status:** ISSUES FOUND - REQUIRES FIXES BEFORE MERGE

## Executive Summary

The security module testing infrastructure implementation has **critical design issues** that prevent the tests from working correctly. The core problem is that ACL enforcement blocks all API calls including the teardown mechanism, creating a deadlock scenario.

---

## 1. TypeScript Validation

| Check | Status | Notes |
|-------|--------|-------|
| `npm run type-check` (frontend) | ✅ PASS | No TypeScript errors |
| Tests use Playwright's built-in TS | ✅ OK | Tests rely on Playwright's TypeScript support |

**Details:** TypeScript compilation passed without errors.

---

## 2. ESLint

| Check | Status | Notes |
|-------|--------|-------|
| ESLint on test files | ⚠️ SKIPPED | Tests directory not covered by ESLint config |

**Details:** The root `eslint.config.js` only covers `frontend/**` files. Test files in `/tests/` are intentionally excluded. This is acceptable since tests use Playwright's type checking, but consider adding ESLint coverage for test files in future.

---

## 3. Playwright Config Validation

| Check | Status | Notes |
|-------|--------|-------|
| Project dependencies | ❌ FIXED | Browser projects cannot depend on teardown |
| Teardown relationship | ✅ CORRECT | security-tests → security-teardown |
| Duplicate project names | ✅ PASS | No duplicates |

**Issue Found & Fixed:**
```javascript
// BEFORE (Invalid - caused error):
dependencies: ['setup', 'security-teardown'],

// AFTER (Fixed):
dependencies: ['setup', 'security-tests'],
```

Browser projects cannot depend on teardown projects directly. They now depend on `security-tests` which has its own teardown.

---

## 4. Security Tests Execution

| Metric | Count |
|--------|-------|
| Total tests | 24 |
| Passed | 5 |
| Failed | 19 |
| Skipped | 0 |

### Passing Tests (5)
- ✅ security-headers-enforcement: X-Content-Type-Options
- ✅ security-headers-enforcement: X-Frame-Options
- ✅ security-headers-enforcement: HSTS behavior
- ✅ security-headers-enforcement: CSP verification
- ✅ acl-enforcement: error response format (partial)

### Failing Tests (19) - Root Cause: ACL Blocking

All failures share the same root cause:
```
Error: Failed to get security status: 403 {"error":"Blocked by access control list"}
```

---

## 5. Critical Design Issue Identified

### The Deadlock Problem

```
1. Security tests enable ACL in beforeAll()
2. ACL starts blocking ALL requests (even authenticated)
3. Test tries to make API calls → 403 Blocked
4. Test fails
5. afterAll() teardown tries to disable ACL via API
6. API call is blocked by ACL → teardown fails
7. Next test run: ACL still enabled → catastrophic failure
```

### Why This Happens

The ACL implementation blocks requests **before** authentication is checked. This means:
- Even authenticated requests get blocked
- The API-based teardown cannot disable ACL
- The global-setup emergency reset runs before auth, so it can't access protected endpoints either

### Impact

- **Tests fail repeatedly** once ACL is enabled
- **Manual intervention required** (database modification)
- **Browser tests blocked** if security tests run first
- **CI/CD pipeline failure** until manually fixed

---

## 6. Teardown Verification

| Check | Status | Notes |
|-------|--------|-------|
| Security modules disabled after tests | ❌ FAIL | ACL blocks the teardown API calls |
| `/api/v1/security/status` accessible | ❌ FAIL | Returns 403 when ACL enabled |

**Verification Attempted:**
```bash
$ curl http://localhost:8080/api/v1/security/status
{"error":"Blocked by access control list"}
```

**Manual Fix Required:**
```bash
docker exec charon-playwright sqlite3 /app/data/charon.db \
  "DELETE FROM settings WHERE key='security.acl.enabled';"
docker restart charon-playwright
```

---

## 7. Pre-commit Hooks

| Hook | Status |
|------|--------|
| fix end of files | ✅ PASS |
| trim trailing whitespace | ✅ PASS |
| check yaml | ✅ PASS |
| check for added large files | ✅ PASS |
| dockerfile validation | ✅ PASS |
| Go Vet | ✅ PASS |
| golangci-lint | ✅ PASS |
| .version matches Git tag | ✅ PASS |
| check-lfs-large-files | ✅ PASS |
| block-codeql-db-commits | ✅ PASS |
| block-data-backups-commit | ✅ PASS |
| Frontend TypeScript Check | ✅ PASS |
| Frontend Lint (Fix) | ✅ PASS |

**All pre-commit hooks passed.**

---

## 8. Files Modified/Created Validation

| File | Status | Notes |
|------|--------|-------|
| `tests/global-setup.ts` | ✅ EXISTS | Emergency reset implemented |
| `playwright.config.js` | ✅ FIXED | Browser dependencies corrected |
| `tests/security-teardown.setup.ts` | ✅ EXISTS | Teardown implemented (but ineffective) |
| `tests/security-enforcement/*.spec.ts` | ✅ EXISTS | 6 test files created |
| `tests/utils/security-helpers.ts` | ✅ EXISTS | Helper functions implemented |

---

## 9. Recommendations

### Critical (MUST FIX before merge)

1. **Implement non-API ACL bypass mechanism**
   - Add a direct database reset in global-setup
   - Use `docker exec` or environment variable to bypass ACL for test auth
   - Consider a test-only endpoint that bypasses ACL

2. **Modify security test design**
   - Don't actually enable ACL in E2E tests if it blocks API
   - Mock ACL responses instead of testing real blocking
   - Or create an ACL whitelist for test runner IP

3. **Add environment variable ACL bypass**
   ```go
   // In ACL middleware
   if os.Getenv("CHARON_TEST_MODE") == "true" {
       // Skip ACL enforcement
   }
   ```

### Important

4. **Add ESLint coverage for test files**
   - Extend `eslint.config.js` to include `tests/**`

5. **Add database-level teardown**
   ```typescript
   // In security-teardown.setup.ts
   await exec(`docker exec charon-playwright sqlite3 /app/data/charon.db
     "UPDATE settings SET value='false' WHERE key='security.acl.enabled';"`);
   ```

6. **Document the ACL blocking behavior**
   - ACL blocks BEFORE authentication
   - This is security-conscious but impacts testability

### Nice to Have

7. **Add health check before tests**
   - Verify ACL is disabled before running security tests
   - Fail fast with clear error message

8. **Add CI/CD recovery script**
   - Auto-reset ACL if tests fail catastrophically

---

## 10. Test Count Summary

| Category | Expected | Actual | Status |
|----------|----------|--------|--------|
| ACL Enforcement | 5 | 1 pass, 4 fail | ❌ |
| WAF Enforcement | 4 | 0 pass, 4 fail | ❌ |
| CrowdSec Enforcement | 3 | 0 pass, 3 fail | ❌ |
| Rate Limit Enforcement | 3 | 0 pass, 3 fail | ❌ |
| Security Headers | 4 | 4 pass | ✅ |
| Combined Enforcement | 5 | 0 pass, 5 fail | ❌ |
| **TOTAL** | **24** | **5 pass, 19 fail** | ❌ |

---

## Conclusion

The security module testing infrastructure has a **fundamental design flaw**: ACL enforcement blocks the teardown mechanism, creating unrecoverable test failures. The implementation is technically sound but operationally broken.

**Recommendation:** Do NOT merge this PR until Issue #9 (ACL bypass mechanism) is resolved. The security-headers-enforcement tests can be merged separately as they work correctly.

---

## Appendix: Files Changed

```
tests/
├── global-setup.ts                     # Modified - emergency reset
├── security-teardown.setup.ts          # New - teardown project
├── security-enforcement/
│   ├── acl-enforcement.spec.ts         # New - 5 tests
│   ├── waf-enforcement.spec.ts         # New - 4 tests
│   ├── crowdsec-enforcement.spec.ts    # New - 3 tests
│   ├── rate-limit-enforcement.spec.ts  # New - 3 tests
│   ├── security-headers-enforcement.spec.ts  # New - 4 tests (WORKING)
│   └── combined-enforcement.spec.ts    # New - 5 tests
└── utils/
    └── security-helpers.ts             # New - helper functions

playwright.config.js                     # Modified - added projects
```
