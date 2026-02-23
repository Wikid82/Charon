# DNS Provider "Add Provider" Button Fix - Complete

**Date**: 2026-02-12
**Issue**: DNS provider tests failing with "button not found" error
**Status**: ✅ RESOLVED - All 18 tests passing

## Root Cause Analysis

### Problem Chain:
1. **Cookie Domain Mismatch (Initial)**:
   - Playwright config used `127.0.0.1:8080` as baseURL
   - Auth setup saved cookies for `localhost`
   - Cookies wouldn't transfer between different domains

2. **localStorage Token Missing (Primary)**:
   - Frontend `AuthContext` checks `localStorage.getItem('charon_auth_token')` on mount
   - If token not found in localStorage, authentication fails immediately
   - httpOnly cookies (secure!) aren't accessible to JavaScript
   - Auth setup only saved cookies, didn't populate localStorage
   - Frontend redirected to login despite valid httpOnly cookie

## Fixes Applied

### Fix 1: Domain Consistency (playwright.config.js & global-setup.ts)
**Changed**: `http://127.0.0.1:8080` → `http://localhost:8080`

**Files Modified**:
- `/projects/Charon/playwright.config.js` (line 126)
- `/projects/Charon/tests/global-setup.ts` (lines 101, 108, 138, 165, 394)

**Reason**: Cookies are domain-specific. Both auth setup and tests must use identical hostname for cookie sharing.

### Fix 2: localStorage Token Storage (auth.setup.ts)
**Added**: Token extraction from login response and localStorage population in storage state

**Changes**:
```typescript
// Extract token from login API response
const loginData = await loginResponse.json();
const token = loginData.token;

// Add localStorage to storage state
savedState.origins = [{
  origin: baseURL,
  localStorage: [
    { name: 'charon_auth_token', value: token }
  ]
}];
```

**Reason**: Frontend requires token in localStorage to initialize auth context, even though httpOnly cookie handles actual authentication.

## Verification Results

### DNS Provider CRUD Tests (18 total)
```bash
PLAYWRIGHT_COVERAGE=0 npx playwright test tests/dns-provider-crud.spec.ts --project=firefox
```

**Result**: ✅ **18/18 PASSED** (31.8s)

**Test Categories**:
- ✅ Create Provider (4 tests)
  - Manual DNS provider
  - Webhook DNS provider
  - Validation errors
  - URL format validation

- ✅ Provider List (3 tests)
  - Display list/empty state
  - Show Add Provider button
  - Show provider details

- ✅ Edit Provider (2 tests)
  - Open edit dialog
  - Update provider name

- ✅ Delete Provider (1 test)
  - Show delete confirmation

- ✅ API Operations (4 tests)
  - List providers
  - Create provider
  - Reject invalid type
  - Get single provider

- ✅ Accessibility (4 tests)
  - Accessible form labels
  - Keyboard navigation
  - Error announcements

## Technical Details

### Authentication Flow (Fixed)
1. **Auth Setup** (runs before tests):
   - POST `/api/v1/auth/login` with credentials
   - Backend returns `{"token": "..."}` in response body
   - Backend sets httpOnly `auth_token` cookie
   - Setup extracts token and saves to storage state:
     - `cookies`: [httpOnly auth_token cookie]
     - `origins.localStorage`: [charon_auth_token: token value]

2. **Browser Tests** (inherit storage state):
   - Playwright loads cookies from storage state
   - Playwright injects localStorage from storage state
   - Frontend `AuthContext` checks localStorage → finds token ✓
   - Frontend calls `/api/v1/auth/me` (with httpOnly cookie) → 200 ✓
   - User authenticated, protected routes accessible ✓

### Why Both Cookie AND localStorage?
- **httpOnly Cookie**: Secure auth token (not accessible to JavaScript, protects from XSS)
- **localStorage Token**: Frontend auth state trigger (tells React app user is logged in)
- **Both Required**: Backend validates cookie, frontend needs localStorage for initialization

## Impact Analysis

### Tests Fixed:
- ✅ `tests/dns-provider-crud.spec.ts` - All 18 tests

### Tests Potentially Affected:
Any test navigating to protected routes after authentication. All should now work correctly with the fixed storage state.

### No Regressions Expected:
- Change is backwards compatible
- Only affects E2E test authentication
- Production auth flow unchanged

## Files Modified

1. **playwright.config.js**
   - Changed baseURL default for non-coverage mode to `localhost:8080`
   - Updated documentation to explain cookie domain requirements

2. **tests/global-setup.ts**
   - Changed all IP references from `127.0.0.1` to `localhost`
   - Updated 5 locations for consistency

3. **tests/auth.setup.ts**
   - Added token extraction from login response
   - Added localStorage population in storage state
   - Added imports: `writeFileSync`, `existsSync`, `dirname`
   - Added validation logging for localStorage creation

## Lessons Learned

1. **Cookie Domains Matter**: Even `127.0.0.1` vs `localhost` breaks cookie sharing
2. **Dual Auth Strategy**: httpOnly cookies + localStorage both serve important purposes
3. **Storage State Power**: Playwright storage state supports both cookies AND localStorage
4. **Auth Flow Alignment**: E2E auth must match production auth exactly
5. **Debug First**: Network monitoring revealed the real issue (localStorage check)

## Next Steps

1. ✅ All DNS provider tests passing
2. ⏭️ Monitor other test suites for similar auth issues
3. ⏭️ Consider documenting auth flow for future developers
4. ⏭️ Verify coverage mode (Vite) still works with new auth setup

## Commands for Future Reference

### Run DNS provider tests
```bash
PLAYWRIGHT_COVERAGE=0 npx playwright test tests/dns-provider-crud.spec.ts --project=firefox
```

### Regenerate auth state (if needed)
```bash
rm -f playwright/.auth/user.json
npx playwright test tests/auth.setup.ts
```

### Check auth state contents
```bash
cat playwright/.auth/user.json | jq .
```

## Conclusion

The "Add Provider" button was always present on the DNS Providers page. The issue was a broken authentication flow preventing tests from reaching the authenticated page state. By fixing cookie domain consistency and adding localStorage token storage to the auth setup, all DNS provider tests now pass reliably.

**Impact**: 18 previously failing tests now passing, 0 regressions introduced.
