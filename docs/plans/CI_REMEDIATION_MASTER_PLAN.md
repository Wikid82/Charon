# CI Remediation Master Plan

**Status:** 🔴 **BLOCKED** - CI failures preventing releases
**Created:** February 12, 2026
**Last Updated:** February 12, 2026
**Priority:** CRITICAL (P0)

---

## Status Overview

**Target:** 100% Pass Rate (0 failures)
**Current:** 98.3% Pass Rate (36 failures total)
**Blockers:** 8 security + 28 Chromium E2E

### Progress Tracker

- [ ] **Phase 1:** Security Fixes (8 items) - **PRIORITY 0** - Est. 7-10 hours
- [ ] **Phase 2:** High-Impact E2E (17 items) - **PRIORITY 1** - Est. 7-10 hours
- [ ] **Phase 3:** Medium-Impact E2E (6 items) - **PRIORITY 2** - Est. 3-5 hours
- [ ] **Phase 4:** Low-Impact E2E (5 items) - **PRIORITY 3** - Est. 2-3 hours
- [ ] **Phase 5:** Final Validation & CI Approval - **MANDATORY** - Est. 2-3 hours

**Current Phase:** Phase 1 - Security Fixes
**Estimated Total Time:** 21-31 hours
**Target Completion:** Within 4-5 business days (split across team)

---

## Phase 1: Security Fixes (PRIORITY 0)

### Overview
**Total Items:** 8 (4 ACL API endpoints + 4 broken imports)
**Current Pass Rate:** 94.2% (65/69 tests passing)
**Target:** 100% (69/69 tests passing)
**Owner:** Backend Dev (API) + Frontend Dev (Imports)
**Status:** 🔴 Not Started

---

#### Task 1.1: Fix ACL Security Status Endpoint

**File:** `backend/internal/routes/security.go`
**Issue:** `GET /api/v1/security/status` returns 404
**Tests Failing:** 2 tests in `tests/security-enforcement/acl-enforcement.spec.ts`
**Owner:** Backend Dev
**Priority:** HIGH
**Estimated Time:** 2 hours

**Root Cause:**
API endpoint missing or not exposed. Frontend ACL UI tests pass (22/22), but API enforcement tests fail because the backend endpoint doesn't exist.

**Implementation Steps:**

1. **Create route handler** in `backend/internal/routes/security.go`:
   ```go
   func GetSecurityStatus(c *gin.Context) {
       // Retrieve current security module states from config
       status := map[string]interface{}{
           "cerberus":  map[string]bool{"enabled": getCerberusEnabled()},
           "acl":       map[string]interface{}{"enabled": getACLEnabled(), "mode": getACLMode()},
           "waf":       map[string]bool{"enabled": getWAFEnabled()},
           "rateLimit": map[string]bool{"enabled": getRateLimitEnabled()},
           "crowdsec":  map[string]interface{}{"enabled": getCrowdSecEnabled(), "mode": getCrowdSecMode()},
       }
       c.JSON(200, status)
   }
   ```

2. **Register route** in router setup:
   ```go
   authorized.GET("/security/status", GetSecurityStatus)
   ```

3. **Add authentication middleware** (already required by `authorized` group)

4. **Write unit tests** in `backend/internal/routes/security_test.go`

**Validation Command:**
```bash
# Run the 2 failing tests
npx playwright test tests/security-enforcement/acl-enforcement.spec.ts --project=chromium --grep "should verify ACL is enabled"
npx playwright test tests/security-enforcement/acl-enforcement.spec.ts --project=chromium --grep "should return security status"
```

**Acceptance Criteria:**
- [ ] API endpoint returns 200 status code
- [ ] JSON response contains all security module states (cerberus, acl, waf, rateLimit, crowdsec)
- [ ] Response includes ACL mode ("allow" or "deny")
- [ ] Authentication middleware enforced (401 without valid token)
- [ ] 2 ACL enforcement tests pass
- [ ] No new test failures introduced
- [ ] Backend unit tests written and passing

---

#### Task 1.2: Fix ACL Access Lists Endpoint

**File:** `backend/internal/routes/access_lists.go`
**Issue:** `GET /api/v1/access-lists` returns 404
**Tests Failing:** 2 tests in `tests/security-enforcement/acl-enforcement.spec.ts`
**Owner:** Backend Dev
**Priority:** HIGH
**Estimated Time:** 2 hours

**Root Cause:**
API endpoint missing. Tests expect to list access lists and test IP addresses against ACL rules, but endpoint doesn't exist.

**Implementation Steps:**

1. **Create route handler** in `backend/internal/routes/access_lists.go`:
   ```go
   func GetAccessLists(c *gin.Context) {
       // Query database for ACL entries
       var accessLists []models.AccessList
       result := db.Find(&accessLists)
       if result.Error != nil {
           c.JSON(500, gin.H{"error": "Failed to fetch access lists"})
           return
       }
       c.JSON(200, accessLists)
   }
   ```

2. **Register route** in router setup:
   ```go
   authorized.GET("/access-lists", GetAccessLists)
   ```

3. **Add optional filtering** by proxy_host_id (query param)

4. **Write unit tests** in `backend/internal/routes/access_lists_test.go`

**Validation Command:**
```bash
# Run the 2 failing tests
npx playwright test tests/security-enforcement/acl-enforcement.spec.ts --project=chromium --grep "should list access lists when ACL enabled"
npx playwright test tests/security-enforcement/acl-enforcement.spec.ts --project=chromium --grep "should test IP against access list"
```

**Acceptance Criteria:**
- [ ] API endpoint returns 200 status code
- [ ] JSON response is array of access list objects
- [ ] Each object includes: id, name, mode, ips, proxy_hosts
- [ ] Empty array returned when no ACLs exist (not 404)
- [ ] Authentication middleware enforced
- [ ] 2 ACL enforcement tests pass
- [ ] No new test failures introduced
- [ ] Backend unit tests written and passing

---

#### Task 1.3: Fix ACL Test IP Endpoint (Optional)

**File:** `backend/internal/routes/access_lists.go`
**Issue:** `POST /api/v1/access-lists/:id/test` may be needed for IP testing
**Tests Potentially Needing This:** Part of "test IP against access list" test
**Owner:** Backend Dev
**Priority:** MEDIUM
**Estimated Time:** 1 hour

**Note:** This may not be a separate endpoint - the test might just be checking if GET /access-lists works. Investigate Task 1.2 first to determine if this is needed.

**Implementation Steps (if needed):**

1. **Create route handler**:
   ```go
   func TestIPAgainstACL(c *gin.Context) {
       aclID := c.Param("id")
       var req struct {
           IP string `json:"ip" binding:"required"`
       }
       if err := c.ShouldBindJSON(&req); err != nil {
           c.JSON(400, gin.H{"error": "Invalid IP format"})
           return
       }

       // Test IP against ACL rules using CIDR matching
       allowed, reason := testIPAgainstACL(aclID, req.IP)
       c.JSON(200, gin.H{"allowed": allowed, "reason": reason})
   }
   ```

2. **Implement CIDR matching logic** for IP testing

**Validation Command:**
```bash
# Run after Task 1.2 to see if this is needed
npx playwright test tests/security-enforcement/acl-enforcement.spec.ts --project=chromium --grep "should test IP against access list"
```

**Acceptance Criteria:**
- [ ] Determine if endpoint is actually needed (may be covered by Task 1.2)
- [ ] If needed: Endpoint validates IP format (400 for invalid)
- [ ] If needed: Returns allow/deny result with reason
- [ ] Test passes without this endpoint, OR endpoint implemented if required

---

#### Task 1.4: Fix Broken Import Paths in zzz-caddy-imports

**Files:**
- `tests/security-enforcement/zzz-caddy-imports/caddy-import-cross-browser.spec.ts`
- `tests/security-enforcement/zzz-caddy-imports/caddy-import-firefox.spec.ts`
- `tests/security-enforcement/zzz-caddy-imports/caddy-import-gaps.spec.ts`
- `tests/security-enforcement/zzz-caddy-imports/caddy-import-webkit.spec.ts`

**Issue:** All 4 files import `from '../fixtures/auth-fixtures'` (wrong path)
**Owner:** Frontend Dev / QA
**Priority:** MEDIUM
**Estimated Time:** 0.5 hours (30 minutes)

**Root Cause:**
Import paths are missing one level. Files are in `tests/security-enforcement/zzz-caddy-imports/`, but fixtures are in `tests/fixtures/`, requiring `../../fixtures/` instead of `../fixtures/`.

**Implementation Steps:**

1. **Fix import paths** in all 4 files:
   ```diff
   - import { test, expect, loginUser } from '../fixtures/auth-fixtures';
   + import { test, expect, loginUser } from '../../fixtures/auth-fixtures';
   ```

2. **Verify import resolution** (files should load without errors)

3. **Run tests** to ensure no new failures introduced

**Validation Command:**
```bash
# Run all 4 caddy-import tests
npx playwright test tests/security-enforcement/zzz-caddy-imports/ --project=chromium
```

**Acceptance Criteria:**
- [ ] All 4 files have corrected import paths to `../../fixtures/auth-fixtures`
- [ ] TypeScript compilation successful (no import errors)
- [ ] Tests run without import resolution errors
- [ ] No new test failures introduced by path fixes
- [ ] Clean `npm run type-check` output

---

### Phase 1 Summary

**Total Tasks:** 4
**Total Estimated Time:** 5.5-7 hours
**Critical Path:** Tasks 1.1 → 1.2 (API endpoints) must complete before Task 1.4 (imports) can be fully validated

**Phase 1 Validation Command:**
```bash
# Run all security tests to verify 100% pass rate
npx playwright test tests/security/ tests/security-enforcement/ --project=chromium

# Expected: 69/69 tests passing (100%)
```

**Phase 1 Exit Criteria:**
- [ ] All 4 ACL API endpoint tests passing
- [ ] All 4 caddy-import tests running without import errors
- [ ] Total security test pass rate: 100% (69/69)
- [ ] No new failures introduced in other test suites
- [ ] Backend unit tests passing for new API endpoints
- [ ] Git commit: `fix(security): implement missing ACL API endpoints + fix import paths`

---

## Phase 2: High-Impact E2E (PRIORITY 1)

### Overview
**Total Failures:** 17 (7 + 5 + 5)
**Categories:** User Lifecycle (7) + Multi-Component Workflows (5) + Data Consistency (5)
**Impact:** CRITICAL - Security, Authentication, Core CRUD Operations
**Owner:** Playwright Dev + QA Engineer
**Status:** 🔴 Not Started

---

#### Task 2.1: Settings - User Lifecycle (7 failures)

**File:** `tests/core/settings-user-lifecycle.spec.ts` (assumed path)
**Browser:** Chromium only (Firefox/WebKit: 0 failures ✅)
**Impact:** CRITICAL - Security, Authentication, Authorization, Audit Logging
**Owner:** Playwright Dev
**Estimated Time:** 3 hours

**Root Cause Hypothesis:**
Browser-specific timing issues. Chromium's faster JavaScript execution may trigger race conditions in authentication state, session management, or permission checks that don't occur in Firefox/WebKit.

**Investigation Steps:**

1. **Run headed to observe behavior:**
   ```bash
   npx playwright test tests/core/settings-user-lifecycle.spec.ts --project=chromium --headed
   ```

2. **Generate trace for analysis:**
   ```bash
   npx playwright test tests/core/settings-user-lifecycle.spec.ts --project=chromium --trace on
   ```

3. **Compare timing vs Firefox** (which has 0 failures):
   ```bash
   npx playwright test tests/core/settings-user-lifecycle.spec.ts --project=firefox --headed
   ```

4. **Check for common patterns:**
   - Authentication state not fully propagated before assertions
   - Session cookies not set before navigation
   - Permission checks executing before role assignment completes
   - Audit log writes not flushed before reads

**Failing Tests (7):**

1. **Deleted user cannot login**
   - Expected: 401 or login failure
   - May need explicit wait for user deletion to propagate to auth middleware

2. **Session persistence after logout and re-login**
   - Expected: New session created, old session invalidated
   - May need `page.waitForLoadState('networkidle')` after logout

3. **Users see only their own data**
   - Expected: User A cannot see User B's resources
   - May need explicit wait after user creation before data isolation check

4. **User cannot promote self to admin**
   - Expected: 403 Forbidden when non-admin tries role escalation
   - May need explicit wait for permission check API call

5. **Permissions apply immediately on user refresh**
   - Expected: Role change → refresh → new permissions active
   - May need explicit wait for role update to propagate to session

6. **Permissions propagate from creation to resource access**
   - Expected: New user → assigned role → can access allowed resources
   - May need explicit wait after role assignment before resource access

7. **Audit log records user lifecycle events**
   - Expected: User create/update/delete events in audit log
   - May need explicit wait for async audit log write to complete

**Likely Fix Pattern:**
Add explicit waits after state-changing operations:
```typescript
// After user deletion
await page.waitForResponse(resp => resp.url().includes('/api/v1/users') && resp.status() === 200);
await page.waitForTimeout(500); // Allow propagation in Chromium

// After role assignment
await page.waitForResponse(resp => resp.url().includes('/api/v1/users') && resp.request().method() === 'PUT');
await page.context().storageState(); // Ensure session updated
```

**Validation Command:**
```bash
# Run all 7 tests
npx playwright test tests/core/settings-user-lifecycle.spec.ts --project=chromium

# Expected: 7/7 passing
```

**Acceptance Criteria:**
- [ ] All 7 tests pass in Chromium
- [ ] 0 failures remain in Firefox/WebKit (no regressions)
- [ ] No test timeout increases beyond 15s per test
- [ ] Fix applied consistently across all 7 tests (same pattern)
- [ ] Trace analysis confirms timing issues resolved

---

#### Task 2.2: Core - Multi-Component Workflows (5 failures)

**File:** `tests/core/multi-component-workflows.spec.ts`
**Browser:** Chromium only (Firefox/WebKit: 0 failures ✅)
**Impact:** HIGH - Security Module Integration, User Permissions, Backup/Restore
**Owner:** Playwright Dev
**Estimated Time:** 2 hours

**Root Cause Hypothesis:**
Complex test scenarios involving multiple async operations (security module toggles, resource creation, permission checks) are timing-sensitive in Chromium.

**Investigation Steps:**

1. **Run headed with debug:**
   ```bash
   npx playwright test tests/core/multi-component-workflows.spec.ts --project=chromium --headed --debug
   ```

2. **Check previous baseline notes:**
   - Previous failures showed 8.8-8.9s timeouts
   - May need timeout increases or better synchronization

3. **Validate security module state propagation:**
   - Ensure `waitForSecurityModuleEnabled()` helper is used
   - Check Caddy reload completion before assertions

**Failing Tests (5):**

1. **WAF enforcement applies to newly created proxy**
   - Expected: Create proxy → enable WAF → proxy blocked by WAF
   - May need wait for Caddy reload after WAF enable

2. **User with proxy creation role can create and manage proxies**
   - Expected: Role assigned → can create proxy → can manage proxy
   - May need explicit wait for permission propagation

3. **Backup restore recovers deleted user data**
   - Expected: Backup → delete data → restore → data recovered
   - May need explicit wait for backup completion before restore

4. **Security modules apply to subsequently created resources**
   - Expected: Enable ACL → create proxy → ACL enforced on proxy
   - May need wait for security module activation before resource creation

5. **Security enforced even on previously created resources**
   - Expected: Create proxy → enable ACL → ACL enforced on existing proxy
   - May need wait for Caddy reload to apply rules to existing resources

**Likely Fix Pattern:**
Add explicit waits for async security operations:
```typescript
// After security module toggle
await waitForSecurityModuleEnabled(page, 'waf', true);
await page.waitForTimeout(1000); // Caddy reload + propagation

// After backup operation
await page.waitForResponse(resp => resp.url().includes('/api/v1/backup') && resp.status() === 200);
await page.waitForTimeout(500); // Ensure file written
```

**Validation Command:**
```bash
# Run all 5 tests
npx playwright test tests/core/multi-component-workflows.spec.ts --project=chromium

# Expected: 5/5 passing
```

**Acceptance Criteria:**
- [ ] All 5 tests pass in Chromium
- [ ] 0 failures remain in Firefox/WebKit (no regressions)
- [ ] Security module state checked before assertions
- [ ] Caddy reload completion verified before enforcement checks
- [ ] No timeout increases beyond 30s per test (complex workflows)

---

#### Task 2.3: Core - Data Consistency (5 failures)

**File:** `tests/core/data-consistency.spec.ts`
**Browser:** Chromium only (Firefox/WebKit: 0 failures ✅)
**Impact:** HIGH - Core CRUD Operations, API/UI Synchronization
**Owner:** Playwright Dev
**Estimated Time:** 2 hours

**Root Cause Hypothesis:**
Data synchronization delays between API operations and UI updates. Chromium may render UI faster than Firefox, causing assertions to execute before data fully propagated.

**Investigation Steps:**

1. **Run headed to observe data propagation:**
   ```bash
   npx playwright test tests/core/data-consistency.spec.ts --project=chromium --headed
   ```

2. **Check previous baseline notes:**
   - Previous failures showed 90s timeout on validation test
   - Likely needs better data synchronization waits

3. **Validate API/UI sync pattern:**
   - Ensure `waitForLoadState('networkidle')` used after mutations
   - Check for explicit waits after CRUD operations

**Failing Tests (5):**

1. **Pagination and sorting produce consistent results**
   - Expected: Sort order and page boundaries match across requests
   - May need explicit wait for table render after sort/pagination change

2. **Client-side and server-side validation consistent**
   - Expected: Both UI and API reject invalid data with same messages
   - May need explicit wait for server validation response

3. **Data stored via API is readable via UI**
   - Expected: POST /api/v1/resource → refresh UI → see new data
   - May need explicit wait for UI data refresh after API mutation

4. **Data deleted via UI is removed from API**
   - Expected: Delete in UI → GET /api/v1/resource → 404
   - May need explicit wait for deletion propagation

5. **Real-time events reflect partial data updates**
   - Expected: WebSocket events show incremental changes
   - May need explicit wait for WebSocket message receipt

**Likely Fix Pattern:**
Add explicit waits for data synchronization:
```typescript
// After API mutation
await page.waitForResponse(resp => resp.url().includes('/api/v1/') && resp.request().method() === 'POST');
await page.reload({ waitUntil: 'networkidle' });

// After UI mutation
await page.waitForLoadState('networkidle');
await page.waitForResponse(resp => resp.url().includes('/api/v1/') && resp.request().method() === 'DELETE');
```

**Validation Command:**
```bash
# Run all 5 tests
npx playwright test tests/core/data-consistency.spec.ts --project=chromium

# Expected: 5/5 passing
```

**Acceptance Criteria:**
- [ ] All 5 tests pass in Chromium
- [ ] 0 failures remain in Firefox/WebKit (no regressions)
- [ ] Network idle state checked before assertions
- [ ] API/UI synchronization verified with explicit waits
- [ ] No timeout increases beyond 30s per test

---

### Phase 2 Summary

**Total Tasks:** 3 (covering 17 test failures)
**Total Estimated Time:** 7 hours
**Critical Path:** All tasks can run in parallel if multiple devs available

**Phase 2 Validation Command:**
```bash
# Run all high-impact tests
npx playwright test tests/core/settings-user-lifecycle.spec.ts --project=chromium
npx playwright test tests/core/multi-component-workflows.spec.ts --project=chromium
npx playwright test tests/core/data-consistency.spec.ts --project=chromium

# Expected: 17/17 tests passing
```

**Phase 2 Exit Criteria:**
- [ ] All 17 high-impact tests passing in Chromium
- [ ] Firefox/WebKit remain at 0 failures (no regressions)
- [ ] Root cause analysis documented for each category
- [ ] Common timing pattern identified and fix applied consistently
- [ ] Git commit: `fix(e2e): resolve Chromium timing issues in user lifecycle, workflows, and data consistency`

---

## Phase 3: Medium-Impact E2E (PRIORITY 2)

### Overview
**Total Failures:** 6 (2 + 2 + 2)
**Categories:** User Management (2) + Modal Dropdowns (2) + Certificates (2)
**Impact:** MEDIUM - User Workflows, Certificate Display
**Owner:** Playwright Dev + Frontend Dev
**Status:** 🔴 Not Started

---

#### Task 3.1: Settings - User Management (2 failures)

**File:** `tests/settings/user-management.spec.ts`
**Browser:** Chromium only
**Impact:** MEDIUM - User Invitation Workflows
**Owner:** Playwright Dev
**Estimated Time:** 1 hour

**Failing Tests (2):**

1. **User should copy invite link**
   - Expected: Copy button copies invite URL to clipboard
   - May need clipboard permission or different clipboard API in Chromium

2. **User should remove permitted hosts**
   - Expected: Remove host from user permissions → host no longer accessible
   - May need explicit wait for permission update

**Investigation:**
```bash
npx playwright test tests/settings/user-management.spec.ts --project=chromium --grep "copy invite link|remove permitted hosts"
```

**Likely Fix:**
Clipboard API may differ in Chromium:
```typescript
// Use Playwright's clipboard API instead of browser's
const clipboardText = await page.evaluate(() => navigator.clipboard.readText());
// Or grant clipboard permission explicitly
await context.grantPermissions(['clipboard-read', 'clipboard-write']);
```

**Validation Command:**
```bash
npx playwright test tests/settings/user-management.spec.ts --project=chromium --grep "copy invite link|remove permitted hosts"
```

**Acceptance Criteria:**
- [ ] Both tests pass in Chromium
- [ ] Clipboard operations work without manual permission grant
- [ ] No regressions in Firefox/WebKit

---

#### Task 3.2: Modal - Dropdown Triage (2 failures)

**File:** `tests/modal-dropdown-triage.spec.ts`
**Browser:** Chromium only
**Impact:** MEDIUM - User Workflows (Invite, Proxy Creation)
**Owner:** Frontend Dev
**Estimated Time:** 1 hour

**Failing Tests (2):**

1. **InviteUserModal Role Dropdown**
   - Expected: Role dropdown opens and allows selection
   - May need role-based locator fix from DNS provider work

2. **ProxyHostForm ACL Dropdown**
   - Expected: ACL dropdown opens and allows selection
   - May need role-based locator fix from DNS provider work

**Known Issue:**
This is part of the dropdown triage effort completed for DNS providers. Same fix pattern should apply.

**Investigation:**
```bash
npx playwright test tests/modal-dropdown-triage.spec.ts --project=chromium
```

**Likely Fix:**
Apply role-based locators:
```typescript
// Before (brittle)
await page.locator('#role-dropdown').click();

// After (robust)
await page.getByRole('combobox', { name: 'Role' }).click();
await page.getByRole('option', { name: 'admin' }).click();
```

**Validation Command:**
```bash
npx playwright test tests/modal-dropdown-triage.spec.ts --project=chromium
```

**Acceptance Criteria:**
- [ ] Both dropdown tests pass in Chromium
- [ ] Locators use `getByRole('combobox')` instead of CSS selectors
- [ ] No regressions in Firefox/WebKit

---

#### Task 3.3: Core - Certificates SSL (2 failures)

**File:** `tests/core/certificates.spec.ts`
**Browser:** Chromium only
**Impact:** MEDIUM - Certificate Visibility
**Owner:** Playwright Dev
**Estimated Time:** 1 hour

**Failing Tests (2):**

1. **Display certificate domain in table**
   - Expected: Certificate list shows domain name column
   - May need explicit wait for table render in Chromium

2. **Display certificate issuer**
   - Expected: Certificate list shows issuer column (Let's Encrypt, etc.)
   - May need explicit wait for API data to populate columns

**Investigation:**
```bash
npx playwright test tests/core/certificates.spec.ts --project=chromium --grep "Display certificate"
```

**Likely Fix:**
Add explicit wait for table data:
```typescript
// Wait for certificate data API response
await page.waitForResponse(resp => resp.url().includes('/api/v1/certificates'));

// Wait for table to render
await page.locator('table tbody tr').first().waitFor({ state: 'visible' });

// Then assert column presence
await expect(page.locator('th:has-text("Domain")')).toBeVisible();
```

**Validation Command:**
```bash
npx playwright test tests/core/certificates.spec.ts --project=chromium --grep "Display certificate"
```

**Acceptance Criteria:**
- [ ] Both certificate display tests pass in Chromium
- [ ] Table columns render correctly after API data loads
- [ ] No regressions in Firefox/WebKit

---

### Phase 3 Summary

**Total Tasks:** 3 (covering 6 test failures)
**Total Estimated Time:** 3 hours
**Critical Path:** All tasks can run in parallel

**Phase 3 Validation Command:**
```bash
# Run all medium-impact tests
npx playwright test tests/settings/user-management.spec.ts --project=chromium --grep "copy invite link|remove permitted hosts"
npx playwright test tests/modal-dropdown-triage.spec.ts --project=chromium
npx playwright test tests/core/certificates.spec.ts --project=chromium --grep "Display certificate"

# Expected: 6/6 tests passing
```

**Phase 3 Exit Criteria:**
- [ ] All 6 medium-impact tests passing in Chromium
- [ ] Firefox/WebKit remain at 0 failures
- [ ] Dropdown locators use robust role-based selectors
- [ ] Git commit: `fix(e2e): resolve user management, dropdown, and certificate display issues`

---

## Phase 4: Low-Impact E2E (PRIORITY 3)

### Overview
**Total Failures:** 5 (2 + 2 + 1)
**Categories:** Authentication (2) + Admin Onboarding (2) + Navigation (1)
**Impact:** LOW - Edge Cases, Mobile UI
**Owner:** Playwright Dev
**Status:** 🔴 Not Started

---

#### Task 4.1: Core - Authentication (2 failures)

**File:** `tests/core/authentication.spec.ts`
**Browser:** Chromium only
**Impact:** LOW - Error Handling Edge Cases
**Owner:** Playwright Dev
**Estimated Time:** 1 hour

**Failing Tests (2):**

1. **Redirect with error message and redirect to login page**
   - Expected: Invalid session → error message → redirect to login
   - May need explicit wait for redirect or error message element

2. **Force login when session expires**
   - Expected: Expired session → forced logout → redirect to login
   - May need explicit wait for session expiration check

**Investigation:**
```bash
npx playwright test tests/core/authentication.spec.ts --project=chromium --grep "Redirect with error|Force login"
```

**Validation Command:**
```bash
npx playwright test tests/core/authentication.spec.ts --project=chromium --grep "Redirect with error|Force login"
```

**Acceptance Criteria:**
- [ ] Both authentication edge case tests pass
- [ ] No regressions in Firefox/WebKit

---

#### Task 4.2: Core - Admin Onboarding (2 failures)

**File:** `tests/core/admin-onboarding.spec.ts`
**Browser:** Chromium only
**Impact:** LOW - First-time Setup Workflow
**Owner:** Playwright Dev
**Estimated Time:** 1 hour

**Failing Tests (2):**

1. **Setup Logout clears session**
   - Expected: First-time admin setup → logout → session cleared
   - May need explicit wait for session clear

2. **First login after logout successful**
   - Expected: Setup → logout → login again → successful
   - May need explicit wait for login redirect after logout

**Investigation:**
```bash
npx playwright test tests/core/admin-onboarding.spec.ts --project=chromium --grep "Setup Logout|First login after logout"
```

**Validation Command:**
```bash
npx playwright test tests/core/admin-onboarding.spec.ts --project=chromium --grep "Setup Logout|First login after logout"
```

**Acceptance Criteria:**
- [ ] Both admin onboarding tests pass
- [ ] Session management correct during first-time setup
- [ ] No regressions in Firefox/WebKit

---

#### Task 4.3: Core - Navigation (1 failure)

**File:** `tests/core/navigation.spec.ts`
**Browser:** Chromium only
**Impact:** LOW - Mobile UI Interaction
**Owner:** Playwright Dev
**Estimated Time:** 0.5 hours (30 minutes)

**Failing Test (1):**

1. **Responsive Navigation should toggle mobile menu**
   - Expected: Small viewport → hamburger menu → click → menu opens
   - May need explicit viewport size or mobile emulation in Chromium

**Investigation:**
```bash
npx playwright test tests/core/navigation.spec.ts --project=chromium --grep "toggle mobile menu"
```

**Likely Fix:**
Ensure viewport explicitly set for mobile:
```typescript
await page.setViewportSize({ width: 375, height: 667 }); // iPhone SE
await page.getByRole('button', { name: 'Toggle menu' }).click();
await expect(page.locator('nav.mobile-menu')).toBeVisible();
```

**Validation Command:**
```bash
npx playwright test tests/core/navigation.spec.ts --project=chromium --grep "toggle mobile menu"
```

**Acceptance Criteria:**
- [ ] Mobile menu toggle test passes in Chromium
- [ ] Viewport size explicitly set for mobile tests
- [ ] No regressions in Firefox/WebKit

---

### Phase 4 Summary

**Total Tasks:** 3 (covering 5 test failures)
**Total Estimated Time:** 2.5 hours
**Critical Path:** All tasks can run in parallel

**Phase 4 Validation Command:**
```bash
# Run all low-impact tests
npx playwright test tests/core/authentication.spec.ts --project=chromium --grep "Redirect with error|Force login"
npx playwright test tests/core/admin-onboarding.spec.ts --project=chromium --grep "Setup Logout|First login after logout"
npx playwright test tests/core/navigation.spec.ts --project=chromium --grep "toggle mobile menu"

# Expected: 5/5 tests passing
```

**Phase 4 Exit Criteria:**
- [ ] All 5 low-impact tests passing in Chromium
- [ ] Firefox/WebKit remain at 0 failures
- [ ] Authentication and onboarding edge cases handled
- [ ] Git commit: `fix(e2e): resolve authentication, onboarding, and navigation edge cases`

---

## Phase 5: Final Validation & CI Approval

### Overview
**Status:** 🔴 Not Started
**Owner:** QA Lead + CI/CD Engineer
**Estimated Time:** 2-3 hours
**Prerequisite:** Phases 1-4 complete with 0 failures

---

### Pre-Merge Validation Checklist (MANDATORY)

#### 1. E2E Playwright Tests
```bash
# Run full suite across all browsers
npx playwright test --project=firefox --project=chromium --project=webkit
```

**Expected Result:** 1624/1624 passing (100%)

**Acceptance Criteria:**
- [ ] Firefox: 0 failures (542/542 passing)
- [ ] Chromium: 0 failures (540/540 passing) - **was 28 failures**
- [ ] WebKit: 0 failures (542/542 passing)
- [ ] No test skips (`test.skip()` = 0)
- [ ] No test timeouts (all tests < 30s)
- [ ] Trace generated for any flaky tests

---

#### 2. Backend Coverage
```bash
# Run backend tests with coverage
scripts/go-test-coverage.sh
```

**Expected Result:** ≥85% coverage with 100% patch coverage

**Acceptance Criteria:**
- [ ] Overall coverage ≥85%
- [ ] Patch coverage = 100% (all modified lines covered)
- [ ] No coverage regressions from previous run
- [ ] All Go unit tests passing
- [ ] `go test ./...` exits with code 0

---

#### 3. Frontend Coverage
```bash
# Run frontend tests with coverage
scripts/frontend-test-coverage.sh
```

**Expected Result:** ≥85% coverage with 100% patch coverage

**Acceptance Criteria:**
- [ ] Overall coverage ≥85%
- [ ] Patch coverage = 100% (all modified lines covered)
- [ ] No coverage regressions from previous run
- [ ] All Vitest unit tests passing
- [ ] `npm test` exits with code 0

---

#### 4. Type Safety
```bash
# TypeScript type checking
npm run type-check
```

**Expected Result:** 0 TypeScript errors

**Acceptance Criteria:**
- [ ] `tsc --noEmit` exits with code 0
- [ ] No `@ts-ignore` or `@ts-expect-error` added
- [ ] All import paths resolve correctly
- [ ] No implicit `any` types introduced

---

#### 5. Pre-commit Hooks
```bash
# Run all pre-commit hooks
pre-commit run --all-files
```

**Expected Result:** All hooks passing

**Acceptance Criteria:**
- [ ] Linting (ESLint, golangci-lint) passes
- [ ] Formatting (Prettier, gofmt) passes
- [ ] Security scans pass (no new issues)
- [ ] GORM security scanner passes (manual stage)
- [ ] All hooks exit with code 0

---

#### 6. Security Scans

**Trivy Docker Image Scan:**
```bash
.github/skills/scripts/skill-runner.sh security-scan-docker-image
```

**Expected Result:** 0 CRITICAL/HIGH vulnerabilities

**CodeQL Scan:**
```bash
.github/skills/scripts/skill-runner.sh security-scan-codeql
```

**Expected Result:** 0 alerts (Critical/High/Medium)

**Acceptance Criteria:**
- [ ] Trivy: 0 CRITICAL vulnerabilities
- [ ] Trivy: 0 HIGH vulnerabilities
- [ ] CodeQL Go: 0 alerts
- [ ] CodeQL JavaScript: 0 alerts
- [ ] SBOM generated and verified
- [ ] All security workflows pass in CI

---

#### 7. CI Workflows (GitHub Actions)

**Required Workflows:**
- [ ] **E2E Tests** - All browsers passing
- [ ] **Go Tests** - Coverage ≥85%, patch 100%
- [ ] **Frontend Tests** - Coverage ≥85%, patch 100%
- [ ] **Security Scans** - Trivy + CodeQL clean
- [ ] **Codecov** - Patch coverage 100%
- [ ] **Build** - Docker image builds successfully
- [ ] **Lint** - All linters passing

**Validation:**
```bash
# Trigger all workflows by pushing to PR branch
git push origin fix/ci-remediation

# Monitor CI status at:
# https://github.com/<org>/<repo>/actions
```

**Acceptance Criteria:**
- [ ] All CI workflows show green checkmarks
- [ ] No workflow failures or cancellations
- [ ] Codecov comment shows patch coverage 100%
- [ ] No new security alerts introduced
- [ ] Build time < 15 minutes (performance check)

---

### Success Criteria Summary

✅ **All checkboxes above must be checked before PR approval**

**Numbers:**
- E2E: 1624/1624 passing (100%) ← was 1592/1620 (98.3%)
- Backend: ≥85% coverage, 100% patch
- Frontend: ≥85% coverage, 100% patch
- Security: 0 CRITICAL/HIGH vulnerabilities
- CI: 7/7 workflows passing

**Quality Gates:**
- [ ] No test skips, no failures, no compromises
- [ ] No security vulnerabilities introduced
- [ ] No coverage regressions
- [ ] No type errors
- [ ] All linters passing

**Ready to Merge:**
- [ ] PR approved by 2+ reviewers
- [ ] All conversations resolved
- [ ] Branch up-to-date with main
- [ ] Squash commits with descriptive message
- [ ] Merge to main → Trigger release pipeline

---

## Quick Reference: Test Commands by Category

### Security Tests
```bash
# All security tests (Phase 1 validation)
npx playwright test tests/security/ tests/security-enforcement/ --project=chromium

# ACL enforcement only (Task 1.1 + 1.2)
npx playwright test tests/security-enforcement/acl-enforcement.spec.ts --project=chromium

# Broken imports only (Task 1.4)
npx playwright test tests/security-enforcement/zzz-caddy-imports/ --project=chromium
```

### E2E Tests by Priority
```bash
# High-Impact (Phase 2 - 17 tests)
npx playwright test tests/core/settings-user-lifecycle.spec.ts --project=chromium
npx playwright test tests/core/multi-component-workflows.spec.ts --project=chromium
npx playwright test tests/core/data-consistency.spec.ts --project=chromium

# Medium-Impact (Phase 3 - 6 tests)
npx playwright test tests/settings/user-management.spec.ts --project=chromium --grep "copy invite link|remove permitted hosts"
npx playwright test tests/modal-dropdown-triage.spec.ts --project=chromium
npx playwright test tests/core/certificates.spec.ts --project=chromium --grep "Display certificate"

# Low-Impact (Phase 4 - 5 tests)
npx playwright test tests/core/authentication.spec.ts --project=chromium --grep "Redirect with error|Force login"
npx playwright test tests/core/admin-onboarding.spec.ts --project=chromium --grep "Setup Logout|First login after logout"
npx playwright test tests/core/navigation.spec.ts --project=chromium --grep "toggle mobile menu"
```

### Debug Commands
```bash
# Headed mode (watch test in browser)
npx playwright test [test-file] --project=chromium --headed

# Debug mode (step through with inspector)
npx playwright test [test-file] --project=chromium --debug

# Generate trace (for later analysis)
npx playwright test [test-file] --project=chromium --trace on

# View trace file
npx playwright show-trace trace.zip
```

### Full Validation (Phase 5)
```bash
# E2E all browsers
npx playwright test --project=firefox --project=chromium --project=webkit

# Backend coverage
scripts/go-test-coverage.sh

# Frontend coverage
scripts/frontend-test-coverage.sh

# Type check
npm run type-check

# Pre-commit
pre-commit run --all-files

# Security scans
.github/skills/scripts/skill-runner.sh security-scan-docker-image
.github/skills/scripts/skill-runner.sh security-scan-codeql
```

---

## Delegation Matrix

| Phase | Task | Owner | Est. Time | Status | Dependencies |
|-------|------|-------|-----------|--------|--------------|
| **1.1** | ACL Security Status API | Backend Dev | 2h | 🔴 Not Started | None |
| **1.2** | ACL Access Lists API | Backend Dev | 2h | 🔴 Not Started | None |
| **1.3** | ACL Test IP API (Optional) | Backend Dev | 1h | 🔴 Not Started | Task 1.2 |
| **1.4** | Fix Broken Import Paths | Frontend Dev | 0.5h | 🔴 Not Started | None |
| **2.1** | User Lifecycle Tests | Playwright Dev | 3h | 🔴 Not Started | Phase 1 Complete |
| **2.2** | Multi-Component Workflows | Playwright Dev | 2h | 🔴 Not Started | Phase 1 Complete |
| **2.3** | Data Consistency Tests | Playwright Dev | 2h | 🔴 Not Started | Phase 1 Complete |
| **3.1** | User Management Tests | Playwright Dev | 1h | 🔴 Not Started | Phase 2 Complete |
| **3.2** | Modal Dropdown Tests | Frontend Dev | 1h | 🔴 Not Started | Phase 2 Complete |
| **3.3** | Certificate Display Tests | Playwright Dev | 1h | 🔴 Not Started | Phase 2 Complete |
| **4.1** | Authentication Edge Cases | Playwright Dev | 1h | 🔴 Not Started | Phase 3 Complete |
| **4.2** | Admin Onboarding Tests | Playwright Dev | 1h | 🔴 Not Started | Phase 3 Complete |
| **4.3** | Navigation Mobile Test | Playwright Dev | 0.5h | 🔴 Not Started | Phase 3 Complete |
| **5.0** | Final Validation & CI | QA Lead | 2-3h | 🔴 Not Started | Phases 1-4 Complete |

**Total Estimated Time:** 21-23 hours
**Critical Path:** Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5

### Team Resource Allocation

**Backend Dev (5.5 hours):**
- Task 1.1: ACL Security Status API (2h)
- Task 1.2: ACL Access Lists API (2h)
- Task 1.3: ACL Test IP API (1h - optional)
- Task 1.4: Code review for frontend import fixes (0.5h)

**Frontend Dev (1.5 hours):**
- Task 1.4: Fix Broken Import Paths (0.5h)
- Task 3.2: Modal Dropdown Tests (1h)

**Playwright Dev (11 hours):**
- Task 2.1: User Lifecycle Tests (3h)
- Task 2.2: Multi-Component Workflows (2h)
- Task 2.3: Data Consistency Tests (2h)
- Task 3.1: User Management Tests (1h)
- Task 3.3: Certificate Display Tests (1h)
- Task 4.1: Authentication Edge Cases (1h)
- Task 4.2: Admin Onboarding Tests (1h)
- Task 4.3: Navigation Mobile Test (0.5h)

**QA Lead (3 hours):**
- Phase 5: Final Validation & CI (2-3h)
- Cross-browser testing validation (included above)
- CI workflow monitoring (included above)

### Parallel Execution Strategy

**Day 1-2: Phase 1 (Security Fixes)**
- Backend Dev: Tasks 1.1 + 1.2 + 1.3 (parallel)
- Frontend Dev: Task 1.4 (parallel with backend)
- **Blocker:** Must complete before Phase 2 starts

**Day 2-3: Phase 2 (High-Impact E2E)**
- Playwright Dev: Tasks 2.1 + 2.2 + 2.3 (serial recommended for pattern identification)
- **Blocker:** Must complete before Phase 3 starts

**Day 3-4: Phase 3 (Medium-Impact E2E)**
- Playwright Dev: Task 3.1 + 3.3 (parallel)
- Frontend Dev: Task 3.2 (parallel)
- **Blocker:** Must complete before Phase 4 starts

**Day 4: Phase 4 (Low-Impact E2E)**
- Playwright Dev: Tasks 4.1 + 4.2 + 4.3 (serial or parallel)

**Day 4-5: Phase 5 (Final Validation)**
- QA Lead: Full validation suite
- All Devs: Fix any regressions discovered

---

## Risk Assessment & Mitigation

| Risk | Severity | Likelihood | Mitigation Strategy | Contingency Plan |
|------|----------|------------|---------------------|------------------|
| **Phase 1 API changes break existing frontend** | HIGH | MEDIUM | Verify frontend ACL UI (22 tests) still passes after API implementation | Rollback API, implement with feature flag |
| **Chromium timing fixes cause Firefox/WebKit failures** | HIGH | LOW | Run full test suite after each fix; validate no regressions | Revert timing changes, use browser-specific waits |
| **Phase 2 fixes take longer than estimated** | MEDIUM | HIGH | Start with Task 2.1 (highest impact); identify common pattern early | Extend timeline by 1-2 days, deprioritize Phase 4 |
| **CI fails after all local tests pass** | MEDIUM | MEDIUM | Test in CI environment before final merge; use CI timeout multipliers | Debug in CI logs, add CI-specific waits |
| **New test failures introduced during fixes** | MEDIUM | MEDIUM | Run full suite after each phase; use git bisect to identify regression | Revert breaking commit, apply fix more surgically |
| **Phase 5 validation discovers edge cases** | LOW | MEDIUM | Thorough testing at each phase; don't skip intermediate validation | Add Phase 6 for edge case fixes, extend timeline by 1 day |
| **Team capacity insufficient for timeline** | MEDIUM | LOW | Parallelize tasks where possible; prioritize critical path | Deprioritize Phase 4 (low-impact), focus on Phases 1-3 first |

---

## Success Metrics & KPIs

### Before Remediation (Baseline)
- **E2E Pass Rate:** 98.3% (1592/1620)
- **Security Pass Rate:** 94.2% (65/69)
- **Chromium Failures:** 28
- **Firefox Failures:** 0
- **WebKit Failures:** 0
- **CI Status:** 🔴 BLOCKED

### After Remediation (Target)
- **E2E Pass Rate:** 100% (1624/1624) ← +32 passing
- **Security Pass Rate:** 100% (69/69) ← +4 passing
- **Chromium Failures:** 0 ← -28 failures
- **Firefox Failures:** 0 ← maintained
- **WebKit Failures:** 0 ← maintained
- **CI Status:** ✅ PASSING

### Improvement Metrics
- **Failure Reduction:** 36 → 0 (100% reduction)
- **Pass Rate Improvement:** +1.7% (98.3% → 100%)
- **Tests Fixed:** 36 tests
- **New Backend APIs:** 2 endpoints
- **Code Quality:** 100% patch coverage maintained

---

## Communication & Reporting

### Daily Standup Updates (Required)

**Format:**
```
**CI Remediation Status - [Date]**
- Current Phase: [X]
- Tasks Completed Today: [List]
- Tests Fixed: [X/36]
- Blockers: [None / List]
- Next 24h Plan: [Tasks]
- ETA to Phase 5: [X days]
```

### Phase Completion Reports (Required)

**Format:**
```
**Phase [X] Complete - [Date]**
✅ Tasks Completed: [List with times]
✅ Tests Fixed: [X]
✅ Pass Rate: [%]
⚠️ Issues Encountered: [None / List with resolutions]
📊 Time Actual vs Estimated: [Xh vs Yh]
➡️ Next Phase: [Name - Starting [Date]]
```

### Final Report (Required at Phase 5)

**Format:**
```
**CI Remediation Complete - [Date]**
✅ All 36 failures resolved
✅ 100% E2E pass rate achieved
✅ CI unblocked - ready to release
📊 Total Time: [Xh] (Est: 21-31h)
📊 Tests Fixed Breakdown:
   - Security: 8
   - High-Impact E2E: 17
   - Medium-Impact E2E: 6
   - Low-Impact E2E: 5
🎉 Ready for PR merge and release!
```

---

## Appendix: Related Documentation

### Source Documents
- [Security Test Suite Remediation Plan](security_suite_remediation.md) - 8 security issues
- [E2E Baseline Fresh Run](../../E2E_BASELINE_FRESH_2026-02-12.md) - 28 Chromium failures

### Testing Documentation
- [Testing Instructions](../../.github/instructions/testing.instructions.md) - Test execution protocols
- [Playwright TypeScript Instructions](../../.github/instructions/playwright-typescript.instructions.md) - Test writing guidelines

### Architecture Documentation
- [Architecture](../../ARCHITECTURE.md) - System architecture overview
- [Contributing](../../CONTRIBUTING.md) - Development guidelines

### Test Files Referenced
- `tests/security-enforcement/acl-enforcement.spec.ts` - 4 API failures
- `tests/security-enforcement/zzz-caddy-imports/*.spec.ts` - 4 broken imports
- `tests/core/settings-user-lifecycle.spec.ts` - 7 Chromium failures
- `tests/core/multi-component-workflows.spec.ts` - 5 Chromium failures
- `tests/core/data-consistency.spec.ts` - 5 Chromium failures
- `tests/settings/user-management.spec.ts` - 2 Chromium failures
- `tests/modal-dropdown-triage.spec.ts` - 2 Chromium failures
- `tests/core/certificates.spec.ts` - 2 Chromium failures
- `tests/core/authentication.spec.ts` - 2 Chromium failures
- `tests/core/admin-onboarding.spec.ts` - 2 Chromium failures
- `tests/core/navigation.spec.ts` - 1 Chromium failure

---

## Version History

| Version | Date | Changes | Author |
|---------|------|---------|--------|
| 1.0 | 2026-02-12 | Initial plan creation | GitHub Copilot (Planning Agent) |

---

**End of Master Plan**
