# Phase 5: TestDataManager Authentication Fix

> **Status**: Ready for Implementation
> **Created**: 2026-01-24
> **Estimated Effort**: M (Medium) - 8-12 hours
> **Priority**: P1 - Blocks user management test coverage
> **Tests to Enable**: 8 tests (user management CRUD operations)

---

## Executive Summary

The `TestDataManager` class uses an authenticated `APIRequestContext` that inherits cookies from the stored auth state. However, a **cookie domain mismatch** prevents those cookies from being sent when tests run against a non-localhost URL (e.g., Tailscale IP `100.98.12.109:8080`). This causes "Admin access required" (401/403) errors when `TestDataManager` attempts to create/delete test users.

**Solution**: Ensure consistent `localhost:8080` base URL throughout the authentication setup and test execution, and verify the cookie domain in stored authentication state matches the test target domain.

---

## Root Cause Analysis

### Current AUTH Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 1. auth.setup.ts runs                                                   │
│    - Creates admin user via /api/v1/setup                               │
│    - Logs in via /api/v1/auth/login                                     │
│    - Saves cookies to playwright/.auth/user.json                        │
│    - Cookie domain: depends on PLAYWRIGHT_BASE_URL or localhost:8080    │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 2. auth-fixtures.ts creates TestDataManager                             │
│    - Reads storageState from playwright/.auth/user.json                 │
│    - Creates APIRequestContext with baseURL                             │
│    - TestDataManager uses this context for API calls                    │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 3. TestDataManager.createUser() called                                  │
│    - POST /api/v1/users with authenticated context                      │
│    - ❌ If baseURL != cookie domain → cookies not sent                 │
│    - ❌ API returns 401 "Admin access required"                        │
└─────────────────────────────────────────────────────────────────────────┘
```

### Cookie Domain Mismatch Scenario

| Stage | URL/Domain | Cookies |
|-------|------------|---------|
| Auth Setup | `http://localhost:8080` | Cookie set for `localhost` |
| Browser Tests | `http://100.98.12.109:8080` | Cookies sent (browser follows redirects) |
| TestDataManager API | `http://100.98.12.109:8080` | ❌ Cookies NOT sent (domain mismatch) |

### Evidence from Code

**[tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts#L534-L537)**:
```typescript
// SKIP: TestDataManager authenticated context not working due to cookie domain mismatch.
// Auth setup creates cookies for 'localhost' but tests run against Tailscale IP (100.98.12.109).
// Cookies aren't sent cross-domain. Fix requires consistent PLAYWRIGHT_BASE_URL environment config.
test.skip('should update permission mode', async ({ page, testData }) => {
```

---

## Affected Tests

### 8 Tests Blocked by TestDataManager Auth Issue

| # | Test Name | Line | Uses `testData` | Skip Reason |
|---|-----------|------|-----------------|-------------|
| 1 | `should update permission mode` | 538 | ✅ | Cookie domain mismatch |
| 2 | `should enable/disable user` | 780 | ✅ | Cookie domain mismatch |
| 3 | `should show pending invite status` | 164 | ✅ | Complex flow + auth |
| 4 | `should open permissions modal` | 494 | ✅ | UI not implemented + auth |
| 5 | `should add permitted hosts` | 612 | ✅ | UI not implemented + auth |
| 6 | `should remove permitted hosts` | 669 | ✅ | Auth + lookup issues |
| 7 | `should save permission changes` | 725 | ✅ | UI not implemented + auth |
| 8 | `should delete user with confirmation` | 847 | ✅ | UI not implemented + auth |

**Note**: Some tests have dual blockers (auth + UI). Once auth is fixed, they may still require UI implementation. The "pure auth" tests are #1 and #2.

---

## Implementation Plan

### Phase 5.1: Consistent Base URL Configuration (2-3 hours)

#### Task 5.1.1: Update playwright.config.js

**File**: [playwright.config.js](../../playwright.config.js#L131-L140)

**Current** (lines 131-140):
```javascript
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('')`. */
    // CI sets PLAYWRIGHT_BASE_URL=http://localhost:8080
    // Local development can override via environment variable
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',  // Line 136

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',
  },
```

**Action**: The default is already `localhost:8080`, but we need to explicitly document that all test environments MUST use localhost for auth to work.

**Changes Required**:
1. Add validation comment
2. Consider adding runtime warning if non-localhost detected

**Proposed** (replace lines 131-140):
```javascript
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL Configuration
     *
     * CRITICAL: Authentication cookies are domain-scoped. The auth.setup.ts
     * stores cookies for the domain in this baseURL. TestDataManager and
     * browser tests must use the SAME domain for cookies to be sent.
     *
     * For local testing, always use http://localhost:8080 (not IP addresses).
     * CI sets PLAYWRIGHT_BASE_URL=http://localhost:8080 automatically.
     */
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',
  },

#### Task 5.1.2: Verify docker-compose.e2e.yml Port Binding

**File**: [.docker/compose/docker-compose.e2e.yml](../../.docker/compose/docker-compose.e2e.yml#L17-L18)

**Current** (lines 17-18):
```yaml
ports:
  - "8080:8080"    # Management UI (Charon)
```

**Status**: ✅ Already correct - binds to `0.0.0.0:8080` which is accessible as `localhost:8080`.

**No changes required**.

#### Task 5.1.3: Update Environment Documentation

**File**: Create or update `.env.example`

**Add**:
```bash
# Playwright E2E Testing
# CRITICAL: Use localhost (not IP address) for cookie authentication to work
PLAYWRIGHT_BASE_URL=http://localhost:8080
```

---

### Phase 5.2: Verify Cookie Domain in Auth State (2-3 hours)

#### Task 5.2.1: Add Cookie Domain Validation to auth.setup.ts

**File**: [tests/auth.setup.ts](../../tests/auth.setup.ts)

**Current** (lines 78-79):
```typescript
  await request.storageState({ path: STORAGE_STATE });
  console.log(`Auth state saved to ${STORAGE_STATE}`);
```

**Changes Required**:

1. **Add import at top of file** (after line 2) - imports cannot be inside functions:
```typescript
import { test as setup, expect } from '@bgotink/playwright-coverage';
import { STORAGE_STATE } from './constants';
import { readFileSync } from 'fs';  // <-- ADD THIS LINE
```

2. **Add validation after saving** (after line 79) - with defensive null checks and try/catch:
```typescript
  await request.storageState({ path: STORAGE_STATE });
  console.log(`Auth state saved to ${STORAGE_STATE}`);

  // Step 5: Verify cookie domain matches expected base URL
  try {
    const savedState = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8'));
    const cookies = savedState.cookies || [];
    const authCookie = cookies.find((c: { name: string }) => c.name === 'auth_token');

    if (authCookie?.domain && baseURL) {
      const expectedHost = new URL(baseURL).hostname;
      if (authCookie.domain !== expectedHost && authCookie.domain !== `.${expectedHost}`) {
        console.warn(`⚠️ Cookie domain mismatch: cookie domain "${authCookie.domain}" does not match baseURL host "${expectedHost}"`);
        console.warn('TestDataManager API calls may fail with 401. Ensure PLAYWRIGHT_BASE_URL uses localhost.');
      } else {
        console.log(`✅ Cookie domain "${authCookie.domain}" matches baseURL host "${expectedHost}"`);
      }
    }
  } catch (err) {
    console.warn('⚠️ Could not validate cookie domain:', err instanceof Error ? err.message : err);
  }
```

#### Task 5.2.2: Add Defensive Check in auth-fixtures.ts

**File**: [tests/fixtures/auth-fixtures.ts](../../tests/fixtures/auth-fixtures.ts#L67-L97)

**Current** (lines 67-97):
```typescript
  testData: async ({ baseURL }, use, testInfo) => {
    // Defensive check: Verify auth state file exists (created by auth.setup.ts)
    if (!existsSync(STORAGE_STATE)) {
      throw new Error(
        `Auth state file not found at ${STORAGE_STATE}. ` +
          'Ensure auth.setup has run first. Check that dependencies: ["setup"] is configured.'
      );
    }

    // Create an authenticated API request context using stored auth state
    // ... rest of fixture
```

**Changes Required**:

1. **Update existing import on line 27** to include `readFileSync`:
```typescript
// Current (line 27):
import { existsSync } from 'fs';

// Change to:
import { existsSync, readFileSync } from 'fs';
```

2. **Add domain validation after defensive check** (insert after line 75) - with null checks and try/catch:
```typescript
    // Defensive check: Verify auth state file exists (created by auth.setup.ts)
    if (!existsSync(STORAGE_STATE)) {
      throw new Error(
        `Auth state file not found at ${STORAGE_STATE}. ` +
          'Ensure auth.setup has run first. Check that dependencies: ["setup"] is configured.'
      );
    }

    // Validate cookie domain matches baseURL to catch configuration issues early
    try {
      const savedState = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8'));
      const cookies = savedState.cookies || [];
      const authCookie = cookies.find((c: { name: string }) => c.name === 'auth_token');

      if (authCookie?.domain && baseURL) {
        const expectedHost = new URL(baseURL).hostname;
        const cookieDomain = authCookie.domain.replace(/^\./, ''); // Remove leading dot

        if (cookieDomain !== expectedHost) {
          console.warn(
            `⚠️ TestDataManager: Cookie domain mismatch detected!\n` +
            `   Cookie domain: "${authCookie.domain}"\n` +
            `   Base URL host: "${expectedHost}"\n` +
            `   API calls will likely fail with 401/403.\n` +
            `   Fix: Set PLAYWRIGHT_BASE_URL=http://localhost:8080 in your environment.`
          );
        }
      }
    } catch (err) {
      console.warn('⚠️ Could not validate cookie domain:', err instanceof Error ? err.message : err);
    }

    // Create an authenticated API request context using stored auth state
    // ... rest unchanged
```

---

### Phase 5.3: Update Test Skip Comments (1-2 hours)

#### Task 5.3.1: Update Skipped Tests with Clear Instructions

For tests that will remain skipped until auth is verified working, update comments:

**File**: [tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts)

**Pattern** - Change from:
```typescript
// SKIP: TestDataManager authenticated context not working due to cookie domain mismatch.
// Auth setup creates cookies for 'localhost' but tests run against Tailscale IP (100.98.12.109).
// Cookies aren't sent cross-domain. Fix requires consistent PLAYWRIGHT_BASE_URL environment config.
test.skip('should update permission mode', ...
```

**To conditional skip** (after fix is implemented):
```typescript
// TestDataManager auth fix: Remove skip once Phase 5 is complete and
// PLAYWRIGHT_BASE_URL is consistently set to http://localhost:8080
test('should update permission mode', ...
```

For tests 1 and 2 (pure auth blockers), remove skip entirely after validation.

---

### Phase 5.4: Re-enable Tests and Validate (2-3 hours)

#### Task 5.4.1: Create Validation Script

**File**: Create `scripts/validate-e2e-auth.sh`

```bash
#!/bin/bash
# Validates E2E authentication setup for TestDataManager

set -eo pipefail

echo "=== E2E Authentication Validation ==="

# Check 0: Verify required dependencies
if ! command -v jq &> /dev/null; then
  echo "❌ jq is required but not installed."
  echo "   Install with: brew install jq (macOS) or apt-get install jq (Linux)"
  exit 1
fi
echo "✅ jq is installed"

# Check 1: Verify PLAYWRIGHT_BASE_URL uses localhost
if [[ -n "$PLAYWRIGHT_BASE_URL" && "$PLAYWRIGHT_BASE_URL" != *"localhost"* ]]; then
  echo "❌ PLAYWRIGHT_BASE_URL ($PLAYWRIGHT_BASE_URL) does not use localhost"
  echo "   Fix: export PLAYWRIGHT_BASE_URL=http://localhost:8080"
  exit 1
fi
echo "✅ PLAYWRIGHT_BASE_URL is localhost or unset (defaults to localhost)"

# Check 2: Verify Docker container is running
if ! docker ps | grep -q charon-e2e; then
  echo "⚠️ charon-e2e container not running. Starting..."
  docker compose -f .docker/compose/docker-compose.e2e.yml up -d
  echo "Waiting for container health..."
  sleep 10
fi
echo "✅ charon-e2e container is running"

# Check 3: Verify API is accessible at localhost:8080
if ! curl -sf http://localhost:8080/api/v1/health > /dev/null; then
  echo "❌ API not accessible at http://localhost:8080"
  exit 1
fi
echo "✅ API accessible at localhost:8080"

# Check 4: Run auth setup and verify cookie domain
echo ""
echo "Running auth setup..."
if ! npx playwright test --project=setup; then
  echo "❌ Auth setup failed"
  exit 1
fi

# Check 5: Verify stored cookie domain
AUTH_FILE="playwright/.auth/user.json"
if [[ -f "$AUTH_FILE" ]]; then
  COOKIE_DOMAIN=$(jq -r '.cookies[] | select(.name=="auth_token") | .domain // empty' "$AUTH_FILE" 2>/dev/null || echo "")
  if [[ -z "$COOKIE_DOMAIN" ]]; then
    echo "❌ No auth_token cookie found in $AUTH_FILE"
    exit 1
  elif [[ "$COOKIE_DOMAIN" == "localhost" || "$COOKIE_DOMAIN" == ".localhost" ]]; then
    echo "✅ Auth cookie domain is localhost"
  else
    echo "❌ Auth cookie domain is '$COOKIE_DOMAIN' (expected 'localhost')"
    exit 1
  fi
else
  echo "❌ Auth state file not found at $AUTH_FILE"
  exit 1
fi

echo ""
echo "=== All validation checks passed ==="
echo "You can now run the user management tests:"
echo "  npx playwright test tests/settings/user-management.spec.ts --project=chromium"
```

#### Task 5.4.2: Test Execution Commands

```bash
# 1. Start E2E environment
docker compose -f .docker/compose/docker-compose.e2e.yml up -d

# 2. Verify health
curl http://localhost:8080/api/v1/health

# 3. Run auth setup only
npx playwright test --project=setup

# 4. Inspect stored auth state
cat playwright/.auth/user.json | jq '.cookies[] | {name, domain, path}'

# 5. Run previously skipped tests
npx playwright test tests/settings/user-management.spec.ts --project=chromium \
  --grep "should update permission mode|should enable/disable user"

# 6. Run all user management tests
npx playwright test tests/settings/user-management.spec.ts --project=chromium
```

---

## File Changes Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `playwright.config.js` | Modify | Add documentation comments about cookie domain requirement |
| `tests/auth.setup.ts` | Modify | Add cookie domain validation after saving state |
| `tests/fixtures/auth-fixtures.ts` | Modify | Add domain mismatch warning in testData fixture |
| `tests/settings/user-management.spec.ts` | Modify | Remove skip from 2 pure-auth tests, update comments on others |
| `scripts/validate-e2e-auth.sh` | Create | Validation script for auth setup |
| `.env.example` | Modify | Add PLAYWRIGHT_BASE_URL documentation |

---

## Dependencies and Blockers

### No External Dependencies
- All changes are within the test infrastructure
- No backend changes required
- No frontend changes required

### Related Work
- Tests 3-8 have **additional UI blockers** (permissions button, delete button, etc.)
- Those tests will remain skipped until Phase 6 (User Management UI) is complete
- This phase unblocks **2 tests immediately** and sets foundation for remaining 6

---

## Success Criteria

| Metric | Before | After |
|--------|--------|-------|
| Tests immediately passing | 0 | 2 |
| Tests unblocked (pending UI) | 0 | 6 |
| Cookie domain validation | None | Automatic warning |
| Documentation | Sparse | Clear setup guide |

### Acceptance Tests

1. ✅ `npx playwright test --grep "should update permission mode" --project=chromium` passes
2. ✅ `npx playwright test --grep "should enable/disable user" --project=chromium` passes
3. ✅ Running tests against non-localhost URL shows clear warning message
4. ✅ `scripts/validate-e2e-auth.sh` passes with exit code 0
5. ✅ No 401/403 errors when TestDataManager creates users

---

## Implementation Assignments

### Backend_Dev Tasks
None - no backend changes required

### Frontend_Dev Tasks

1. **Task F5.1**: Update [playwright.config.js](../../playwright.config.js#L131-L140)
   - Add documentation comments about cookie domain
   - Time: 15 minutes

2. **Task F5.2**: Update [tests/auth.setup.ts](../../tests/auth.setup.ts#L1-5,L78-79)
   - Add cookie domain validation
   - Time: 30 minutes

3. **Task F5.3**: Update [tests/fixtures/auth-fixtures.ts](../../tests/fixtures/auth-fixtures.ts#L27,L67-L97)
   - Add domain mismatch warning
   - Add `readFileSync` import
   - Time: 30 minutes

4. **Task F5.4**: Create `scripts/validate-e2e-auth.sh`
   - Create validation script
   - Make executable: `chmod +x scripts/validate-e2e-auth.sh`
   - Time: 20 minutes

5. **Task F5.5**: Update [tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts)
   - Remove `test.skip` from lines 538 and 780
   - Update comments on other skipped tests
   - Time: 30 minutes

6. **Task F5.6**: Validate
   - Run validation script
   - Run the 2 enabled tests
   - Verify no regressions in other tests
   - Time: 1 hour

---

## Rollback Plan

If the fix causes issues:

1. Revert test file changes (re-add `test.skip`)
2. Keep validation/warning code (it only logs, doesn't fail)
3. Document any newly discovered issues

---

## References

- [Skipped Tests Remediation Plan](skipped-tests-remediation.md) - Phase 5 section
- [Playwright storageState docs](https://playwright.dev/docs/api/class-browsercontext#browser-context-storage-state)
- [HTTP Cookie domain scope](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#domain)
- [Testing Instructions](../../.github/instructions/testing.instructions.md) - E2E section

---

## Change Log

| Date | Author | Change |
|------|--------|--------|
| 2026-01-24 | Planning Agent | Initial plan created |
| 2026-01-24 | Planning Agent | CRITICAL FIXES: Fixed line numbers (baseURL@L136, storageState@L78-79), moved imports to file top, added null checks (`authCookie?.domain && baseURL`), wrapped validation in try/catch, added jq check + `set -eo pipefail` to script, updated auth-fixtures.ts import pattern |
