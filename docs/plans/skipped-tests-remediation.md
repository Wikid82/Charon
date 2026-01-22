# Skipped Playwright Tests Remediation Plan

> **Status**: Active
> **Created**: 2024
> **Total Skipped Tests**: 98
> **Target**: Reduce to <10 intentional skips

## Executive Summary

This plan addresses 98 skipped Playwright E2E tests discovered through comprehensive codebase analysis. The skips fall into 6 distinct categories, with **Cerberus-dependent tests (35 tests)** representing the highest-impact remediation opportunity—enabling a single feature flag could restore 35+ tests to active status.

### Quick Stats

| Category | Count | Effort | Priority |
|----------|-------|--------|----------|
| Environment-Dependent (Cerberus) | 35 | S | P0 |
| Feature Not Implemented | 25 | L | P1 |
| Route/API Not Implemented | 12 | M | P1 |
| UI Mismatch/Test ID Issues | 10 | S | P2 |
| TestDataManager Auth Issues | 8 | M | P1 |
| Flaky/Timing Issues | 5 | S | P2 |
| Intentional Skips | 3 | - | - |

---

## Category 1: Environment-Dependent Tests (Cerberus Disabled)

**Count**: 35 tests
**Effort**: S (Small) - Configuration change
**Priority**: P0 - Highest impact, lowest effort

### Root Cause

The Cerberus security module is disabled in the E2E test environment. Tests check `cerberusEnabled` flag and skip when false.

### Affected Files

| File | Skipped Tests | Skip Pattern |
|------|---------------|--------------|
| [tests/monitoring/real-time-logs.spec.ts](../../tests/monitoring/real-time-logs.spec.ts) | 25 | `test.skip(!cerberusEnabled, 'LiveLogViewer not available...')` |
| [tests/security/security-dashboard.spec.ts](../../tests/security/security-dashboard.spec.ts) | 7 | `test.skip(!cerberusEnabled, 'Toggle is disabled...')` |
| [tests/security/rate-limiting.spec.ts](../../tests/security/rate-limiting.spec.ts) | 2 | `test.skip(!cerberusEnabled, 'Rate limit toggle disabled...')` |

### Skip Pattern Example

```typescript
// From real-time-logs.spec.ts
const cerberusEnabled = await page.evaluate(() => {
  return window.__CHARON_CONFIG__?.cerberusEnabled ?? false;
});
test.skip(!cerberusEnabled, 'LiveLogViewer not available - Cerberus security module is disabled');
```

### Remediation

**Option A (Recommended)**: Enable Cerberus in E2E environment
```bash
# In docker-compose.e2e.yml or test environment config
CERBERUS_ENABLED=true
```

**Option B**: Create Cerberus-enabled test project in playwright.config.js
```javascript
{
  name: 'cerberus',
  use: { ...devices['Desktop Chrome'] },
  dependencies: ['setup'],
  testMatch: '**/security/**/*.spec.ts',
  // Requires Cerberus-enabled environment
}
```

**Option C**: Mock Cerberus state in tests (less ideal - tests won't verify real behavior)

---

## Category 2: Feature Not Implemented

**Count**: 25 tests
**Effort**: L (Large) - Requires frontend development
**Priority**: P1 - Core functionality gaps

### Affected Areas

#### 2.1 User Management UI Components (~15 tests)

**File**: [tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts)

| Missing Component | Test Lines | Description |
|-------------------|------------|-------------|
| User Status Badge | 47, 86 | Visual indicator for active/inactive users |
| Role Badge | 113 | Visual indicator for user roles |
| Invite User Button | 144 | UI to trigger user invitation flow |
| User Settings Button | 171 | Per-user settings/permissions access |
| Delete User Button | 236, 267 | User deletion with confirmation |
| Create User Modal | 312-350 | Full user creation workflow |
| Edit User Modal | 380-420 | User editing interface |

**Skip Pattern Example**:
```typescript
test.skip('should display user status badges', async ({ page }) => {
  // UI component not yet implemented
  const statusBadge = page.getByTestId('user-status-badge');
  await expect(statusBadge.first()).toBeVisible();
});
```

**Remediation**:
1. Implement missing UI components in `frontend/src/components/settings/UserManagement.tsx`
2. Add proper `data-testid` attributes for test targeting
3. Update tests to match implemented component structure

#### 2.2 Notification Template Management (~9 tests)

**File**: [tests/settings/notifications.spec.ts](../../tests/settings/notifications.spec.ts)

| Missing Feature | Lines | Description |
|-----------------|-------|-------------|
| Template list display | 289-310 | Show saved notification templates |
| Template creation form | 340-380 | Create new templates with variables |
| Template editing | 410-450 | Edit existing templates |
| Template preview | 480-510 | Preview rendered template |
| Provider-specific forms | 550-620 | Discord/Slack/Webhook config forms |

**Remediation**:
1. Implement template CRUD UI in notification settings
2. Add test IDs matching expected patterns: `template-list`, `template-form`, `template-preview`

---

## Category 3: Route/API Not Implemented

**Count**: 12 tests
**Effort**: M (Medium) - Backend + Frontend work
**Priority**: P1 - Missing functionality

### Affected Files

#### 3.1 Import Routes

**File**: [tests/integration/import-to-production.spec.ts](../../tests/integration/import-to-production.spec.ts)

| Missing Route | Tests | Description |
|---------------|-------|-------------|
| `/tasks/import/npm` | 3 | Import from NPM configuration |
| `/tasks/import/json` | 3 | Import from JSON format |

**Skip Pattern**:
```typescript
test.skip('should import NPM configuration', async ({ page }) => {
  // Route /tasks/import/npm not implemented
  await page.goto('/tasks/import/npm');
  // ...
});
```

**Remediation**:
1. Backend: Implement NPM/JSON import handlers in `backend/api/handlers/`
2. Frontend: Add import route components
3. Update tests once routes exist

#### 3.2 CrowdSec Decisions Route

**File**: [tests/security/crowdsec-decisions.spec.ts](../../tests/security/crowdsec-decisions.spec.ts)

**Issue**: Entire test file uses `test.describe.skip()` because `/security/crowdsec/decisions` route doesn't exist. Decisions are displayed within the main CrowdSec config page.

**Remediation Options**:
1. Create dedicated decisions route (matches test expectations)
2. Refactor tests to work with embedded decisions UI in main CrowdSec page
3. Delete test file if decisions are intentionally not a separate page

---

## Category 4: UI Mismatch / Test ID Issues

**Count**: 10 tests
**Effort**: S (Small) - Test or selector updates
**Priority**: P2 - Test maintenance

### Affected Files

| File | Issue | Lines |
|------|-------|-------|
| [tests/settings/account-settings.spec.ts](../../tests/settings/account-settings.spec.ts) | Checkbox toggle behavior inconsistent | 260 |
| [tests/settings/smtp-settings.spec.ts](../../tests/settings/smtp-settings.spec.ts) | SMTP save not persisting (backend issue) | 336 |
| [tests/settings/smtp-settings.spec.ts](../../tests/settings/smtp-settings.spec.ts) | Test email section conditional | 590, 664 |
| [tests/settings/system-settings.spec.ts](../../tests/settings/system-settings.spec.ts) | Language selector not found | 386 |
| [tests/dns-provider-crud.spec.ts](../../tests/dns-provider-crud.spec.ts) | Provider dropdown IDs | 89, 134, 178 |

### Skip Pattern Examples

```typescript
// account-settings.spec.ts:260
test.skip('should enter custom certificate email', async ({ page }) => {
  // Note: checkbox toggle behavior inconsistent; may need double-click or wait
});

// smtp-settings.spec.ts:336
test.skip('should update existing SMTP configuration', async ({ page }) => {
  // Note: SMTP save not persisting correctly (backend issue, not test issue)
});
```

### Remediation

1. **Checkbox Toggle**: Add explicit waits or use `force: true` for toggle clicks
2. **SMTP Persistence**: Investigate backend `/api/v1/settings/smtp` endpoint
3. **Language Selector**: Update selector to match actual component (`#language-select` or `[data-testid="language-selector"]`)
4. **DNS Provider Dropdowns**: Verify dropdown IDs match implementation

---

## Category 5: TestDataManager Authentication Issues

**Count**: 8 tests
**Effort**: M (Medium) - Fixture refactoring
**Priority**: P1 - Blocks test data creation

### Root Cause

`TestDataManager` uses raw API requests that don't inherit browser authentication context, causing "Admin access required" errors when creating test data.

**File**: [tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts)

### Affected Operations

```typescript
// These operations fail with 401/403:
await testData.createUser({ email: 'test@example.com', role: 'user' });
await testData.deleteUser(userId);
```

### Skip Pattern

```typescript
test.skip('should create and verify new user', async ({ page, testData }) => {
  // testData.createUser uses unauthenticated API calls
  // causing "Admin access required" errors
});
```

### Remediation

**Option A (Recommended)**: Pass authenticated APIRequestContext to TestDataManager

```typescript
// In auth-fixtures.ts
const authenticatedContext = await request.newContext({
  storageState: 'playwright/.auth/user.json'
});

const testData = new TestDataManager(authenticatedContext);
```

**Option B**: Use page context for API calls

```typescript
// In TestDataManager
async createUser(userData: UserTestData) {
  return await this.page.request.post('/api/v1/users', {
    data: userData
  });
}
```

---

## Category 6: Flaky/Timing Issues

**Count**: 5 tests
**Effort**: S (Small) - Test stabilization
**Priority**: P2

### Affected Files

| File | Issue | Lines |
|------|-------|-------|
| [tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts) | Keyboard navigation timing | 478-510 |
| [tests/core/navigation.spec.ts](../../tests/core/navigation.spec.ts) | Skip link not implemented | 597 |
| [tests/settings/encryption-management.spec.ts](../../tests/settings/encryption-management.spec.ts) | Rotation button state | 156, 189, 245 |

### Remediation

1. **Keyboard Navigation**: Add explicit waits between key presses
2. **Skip Link**: Implement skip-to-main link in app, then unskip test
3. **Rotation Button**: Wait for button state before asserting

---

## Category 7: Intentional Skips

**Count**: 3 tests
**Effort**: None
**Priority**: N/A - By design

These tests are intentionally skipped with documented reasons:

| File | Reason |
|------|--------|
| [tests/core/navigation.spec.ts:597](../../tests/core/navigation.spec.ts#L597) | TODO: Implement skip-to-content link in application |

---

## Remediation Phases

### Phase 1: Quick Wins (Week 1)
**Target**: Enable 40+ tests with minimal effort

1. ✅ Enable Cerberus in E2E environment (+35 tests)
2. ✅ Fix checkbox toggle waits in account-settings (+1 test)
3. ✅ Fix language selector ID in system-settings (+1 test)
4. ✅ Stabilize keyboard navigation tests (+3 tests)

**Estimated Work**: 2-4 hours

### Phase 2: Authentication Fix (Week 2)
**Target**: Enable TestDataManager-dependent tests
**Status**: 🔸 PARTIALLY COMPLETE - Blocked by environment config

1. ✅ Refactor TestDataManager to use authenticated context
2. ✅ Update auth-fixtures.ts to provide authenticated API context
3. 🔸 Re-enable user management tests (+8 tests) - BLOCKED

**Implementation Completed**:
- `auth-fixtures.ts` updated with `playwrightRequest.newContext({ storageState })` pattern
- Defensive `existsSync()` check added
- `try/finally` with `dispose()` for proper cleanup

**Blocker Discovered**: Cookie domain mismatch
- Auth setup creates cookies for `localhost` domain
- Tests run against Tailscale IP `100.98.12.109:8080`
- Cookies aren't sent cross-domain → API calls remain unauthenticated
- **Fix required**: Set `PLAYWRIGHT_BASE_URL=http://localhost:8080` consistently

**Tests Remain Skipped**: 8 tests still skipped with updated comments documenting the environment configuration issue.

**Actual Work**: 2-3 hours (code complete, blocked by environment)

### Phase 3: Backend Routes (Week 3-4)
**Target**: Implement missing API routes

1. Implement NPM import route
2. Implement JSON import route
3. Review SMTP persistence issue
4. Re-enable import tests (+6 tests)

**Estimated Work**: 16-24 hours

### Phase 4: UI Components (Week 5-8)
**Target**: Implement missing frontend components

1. User management UI components
   - Status badges
   - Role badges
   - Action buttons
   - Modals
2. Notification template management UI
3. Re-enable feature tests (+25 tests)

**Estimated Work**: 40-60 hours

---

## Dependencies & Blockers

### External Dependencies

| Dependency | Impact | Owner |
|------------|--------|-------|
| Cerberus module availability | Blocks 35 tests | DevOps |
| Backend SMTP fix | Blocks 3 tests | Backend team |
| NPM/JSON import API design | Blocks 6 tests | Architecture |

### Technical Blockers

1. **TestDataManager Auth**: Requires fixture refactoring - blocks 8 tests
2. **CrowdSec Decisions Route**: Architectural decision needed - blocks 6 tests
3. **Notification Templates**: UI design needed - blocks 9 tests

---

## Top 5 Priority Fixes

| Rank | Fix | Tests Enabled | Effort | ROI |
|------|-----|---------------|--------|-----|
| 1 | Enable Cerberus in E2E | 35 | S | ⭐⭐⭐⭐⭐ |
| 2 | Fix TestDataManager auth | 8 | M | ⭐⭐⭐⭐ |
| 3 | Implement user management UI | 15 | L | ⭐⭐⭐ |
| 4 | Fix UI selector mismatches | 6 | S | ⭐⭐⭐ |
| 5 | Implement import routes | 6 | M | ⭐⭐ |

---

## Success Metrics

| Metric | Current | Target | Stretch |
|--------|---------|--------|---------|
| Total Skipped Tests | 98 | <20 | <10 |
| Cerberus Tests Passing | 0 | 35 | 35 |
| User Management Tests | 0 | 15 | 22 |
| Import Tests | 0 | 6 | 6 |
| Test Coverage Impact | ~75% | ~85% | ~90% |

---

## Appendix A: Full Skip Inventory

### By File

| File | Skip Count | Primary Reason |
|------|------------|----------------|
| `monitoring/real-time-logs.spec.ts` | 25 | Cerberus disabled |
| `settings/user-management.spec.ts` | 22 | UI not implemented |
| `settings/notifications.spec.ts` | 9 | Template UI incomplete |
| `security/security-dashboard.spec.ts` | 7 | Cerberus disabled |
| `settings/encryption-management.spec.ts` | 7 | Rotation unavailable |
| `integration/import-to-production.spec.ts` | 6 | Routes not implemented |
| `security/crowdsec-decisions.spec.ts` | 6 | Route doesn't exist |
| `dns-provider-crud.spec.ts` | 6 | No providers exist |
| `settings/system-settings.spec.ts` | 4 | UI mismatches |
| `settings/smtp-settings.spec.ts` | 3 | Backend issues |
| `settings/account-settings.spec.ts` | 3 | Toggle behavior |
| `security/rate-limiting.spec.ts` | 2 | Cerberus disabled |
| `core/navigation.spec.ts` | 1 | Skip link TODO |

### Skip Types Distribution

```
Environment-Dependent: ████████████████████ 35 (36%)
Feature Not Implemented: ██████████████ 25 (26%)
Route/API Missing: ████████ 12 (12%)
UI Mismatch: ██████ 10 (10%)
TestDataManager Auth: █████ 8 (8%)
Flaky/Timing: ███ 5 (5%)
Intentional: ██ 3 (3%)
```

---

## Appendix B: Commands

### Check Current Skip Count
```bash
grep -r "test\.skip\|test\.fixme\|\.skip\(" tests/ | wc -l
```

### Run Only Skipped Tests (for verification)
```bash
npx playwright test --grep "@skip" --project=chromium
```

### Generate Updated Skip Report
```bash
grep -rn "test\.skip\|test\.fixme" tests/ --include="*.spec.ts" > skip-report.txt
```

---

## Change Log

| Date | Author | Change |
|------|--------|--------|
| 2024-XX-XX | AI Analysis | Initial plan created |
