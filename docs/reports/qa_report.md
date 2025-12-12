# QA Security Report: WAF to Coraza Rename

**Date:** December 12, 2025
**Agent:** QA_Security
**Scope:** Frontend UI changes renaming "WAF (Coraza)" to "Coraza"

---

## Executive Summary

**Overall Status: ✅ PASS**

All tests pass after fixing test assertions to match the new UI. The rename from "WAF (Coraza)" to "Coraza" has been successfully implemented and verified.

---

## Test Results

### TypeScript Compilation

| Check | Status |
|-------|--------|
| `npm run type-check` | ✅ PASS |

**Output:** Clean compilation with no errors.

### Frontend Unit Tests

| Metric | Count |
|--------|-------|
| Test Files | 84 |
| Tests Passed | 728 |
| Tests Skipped | 2 |
| Tests Failed | 0 |
| Duration | ~61s |

**Initial Run:** 4 failures related to outdated test assertions
**After Fix:** All 728 tests passing

#### Issues Found and Fixed

1. **Security.test.tsx - Line 281**
   - **Issue:** Test expected card title `'WAF (Coraza)'` but UI shows `'Coraza'`
   - **Severity:** Low (test sync issue)
   - **Fix:** Updated assertion to expect `'Coraza'`

2. **Security.test.tsx - Lines 252-267 (WAF Controls describe block)**
   - **Issue:** Tests for `waf-mode-select` and `waf-ruleset-select` dropdowns that were removed from the Security page
   - **Severity:** Low (removed UI elements)
   - **Fix:** Removed the `WAF Controls` test suite as dropdowns are now on dedicated `/security/waf` page

### Lint Results

| Tool | Errors | Warnings |
|------|--------|----------|
| ESLint | 0 | 5 |

**Warnings (pre-existing, not related to this change):**

- `CrowdSecConfig.tsx:212` - React Hook useEffect missing dependencies
- `CrowdSecConfig.tsx:715` - Unexpected any type
- `CrowdSecConfig.spec.tsx:258,284,317` - Unexpected any types in tests

### Pre-commit Hooks

| Hook | Status |
|------|--------|
| Go Test Coverage (85.1%) | ✅ PASS |
| Go Vet | ✅ PASS |
| Check .version matches Git tag | ✅ PASS |
| Prevent large files not tracked by LFS | ✅ PASS |
| Prevent committing CodeQL DB artifacts | ✅ PASS |
| Prevent committing data/backups files | ✅ PASS |
| Frontend TypeScript Check | ✅ PASS |
| Frontend Lint (Fix) | ✅ PASS |

---

## File Verification

### Security.tsx (`frontend/src/pages/Security.tsx`)

| Check | Status | Details |
|-------|--------|---------|
| Card title shows "Coraza" | ✅ Verified | Line 320: `<h3>Coraza</h3>` |
| No "WAF (Coraza)" text in card title | ✅ Verified | Confirmed via grep search |
| Dropdowns removed from Security page | ✅ Verified | Controls moved to `/security/waf` config page |
| Internal API field names unchanged | ✅ Verified | `status.waf.enabled`, `toggle-waf` testid preserved for API compatibility |

### Layout.tsx (`frontend/src/components/Layout.tsx`)

| Check | Status | Details |
|-------|--------|---------|
| Navigation shows "Coraza" | ✅ Verified | Line 70: `{ name: 'Coraza', path: '/security/waf', icon: '🛡️' }` |

---

## Changes Made During QA

### Test File Update: Security.test.tsx

```diff
- describe('WAF Controls', () => {
-   it('should change WAF mode', async () => { ... })
-   it('should change WAF ruleset', async () => { ... })
- })
+ // Note: WAF Controls tests removed - dropdowns moved to dedicated WAF config page (/security/waf)

- expect(cardNames).toEqual(['CrowdSec', 'Access Control', 'WAF (Coraza)', 'Rate Limiting', 'Live Security Logs'])
+ expect(cardNames).toEqual(['CrowdSec', 'Access Control', 'Coraza', 'Rate Limiting', 'Live Security Logs'])
```

---

## Recommendations

1. **No blocking issues** - All changes are complete and verified.

2. **Pre-existing warnings** - Consider addressing the `@typescript-eslint/no-explicit-any` warnings in `CrowdSecConfig.tsx` and its test file in a future cleanup pass.

---

## Conclusion

The WAF to Coraza rename has been successfully implemented:

- ✅ UI displays "Coraza" in the Security dashboard card
- ✅ Navigation shows "Coraza" instead of "WAF"
- ✅ Dropdowns removed from main Security page (moved to dedicated config page)
- ✅ All 728 frontend tests pass
- ✅ TypeScript compiles without errors
- ✅ No new lint errors introduced
- ✅ All pre-commit hooks pass

**QA Approval:** ✅ Approved for merge
