# Caddy Import E2E Test Plan - Gap Coverage
# Caddy Import E2E Test Plan - Gap Coverage

**Created**: 2026-01-30
**Status**: Active
**Target File**: `tests/tasks/caddy-import-gaps.spec.ts`
**Related**: `tests/tasks/caddy-import-debug.spec.ts`, `tests/tasks/import-caddyfile.spec.ts`
**Created**: 2026-01-30
**Status**: Active
**Target File**: `tests/tasks/caddy-import-gaps.spec.ts`
**Related**: `tests/tasks/caddy-import-debug.spec.ts`, `tests/tasks/import-caddyfile.spec.ts`

---

## Overview

This plan addresses 5 identified gaps in Caddy Import E2E test coverage. Tests will follow established patterns from existing test files, using:
- Stored auth state (no `loginUser()` calls needed)
- Response waiters registered BEFORE click actions
- Real API calls (no mocking) for reliable integration testing
- **TestDataManager fixture** from `auth-fixtures.ts` for automatic resource cleanup and namespace isolation
- **Relative paths** with the `request` fixture (baseURL pre-configured)
- **Automatic namespacing** via TestDataManager to prevent parallel execution conflicts

---

## Gap 1: Success Modal Navigation

**Priority**: 🔴 CRITICAL
**Complexity**: Medium

### Test Case 1.1: Success modal appears after commit

**Title**: `should display success modal after successful import commit`

**Prerequisites**:
- Container running with healthy API
- No pending import session

**Setup (API)**:
```typescript
// TestDataManager handles cleanup automatically
// No explicit setup needed - clean state guaranteed by fixture
```

**Steps**:
1. Navigate to `/tasks/import/caddyfile`
2. Paste valid Caddyfile content:
   ```
   success-modal-test.example.com {
       reverse_proxy localhost:3000
   }
   ```
3. Register response waiter for `/api/v1/import/upload`
4. Click "Parse and Review" button
5. Wait for review table to appear
6. Register response waiter for `/api/v1/import/commit`
7. Click "Commit Import" button
8. Wait for commit response

**Assertions**:
- `[data-testid="import-success-modal"]` is visible
- Modal contains text "Import Completed"
- Modal shows "1 host created" or similar count

**Selectors**:
| Element | Selector |
|---------|----------|
| Success Modal | `[data-testid="import-success-modal"]` |
| Commit Button | `page.getByRole('button', { name: /commit/i })` |
| Modal Header | `page.getByTestId('import-success-modal').locator('h2')` |

---

### Test Case 1.2: "View Proxy Hosts" button navigation

**Title**: `should navigate to /proxy-hosts when clicking View Proxy Hosts button`

**Prerequisites**:
- Success modal visible (chain from 1.1 or re-setup)

**Setup (API)**:
```typescript
// TestDataManager provides automatic cleanup
// Use helper function to complete import flow
```

**Steps**:
1. Complete import flow (reuse helper or inline steps from 1.1)
2. Wait for success modal to appear
3. Click "View Proxy Hosts" button

**Assertions**:
- `page.url()` ends with `/proxy-hosts`
- Success modal is no longer visible

**Selectors**:
| Element | Selector |
|---------|----------|
| View Proxy Hosts Button | `button:has-text("View Proxy Hosts")` |

---

### Test Case 1.3: "Go to Dashboard" button navigation

**Title**: `should navigate to /dashboard when clicking Go to Dashboard button`

**Prerequisites**:
- Success modal visible

**Steps**:
1. Complete import flow
2. Wait for success modal to appear
3. Click "Go to Dashboard" button

**Assertions**:
- `page.url()` matches `/` or `/dashboard`
- Success modal is no longer visible

**Selectors**:
| Element | Selector |
|---------|----------|
| Dashboard Button | `button:has-text("Go to Dashboard")` |
## Overview

This plan addresses 5 identified gaps in Caddy Import E2E test coverage. Tests will follow established patterns from existing test files, using:
- Stored auth state (no `loginUser()` calls needed)
- Response waiters registered BEFORE click actions
- Real API calls (no mocking) for reliable integration testing
- **TestDataManager fixture** from `auth-fixtures.ts` for automatic resource cleanup and namespace isolation
- **Relative paths** with the `request` fixture (baseURL pre-configured)
- **Automatic namespacing** via TestDataManager to prevent parallel execution conflicts

---

## Gap 1: Success Modal Navigation

**Priority**: 🔴 CRITICAL
**Complexity**: Medium

### Test Case 1.1: Success modal appears after commit

**Title**: `should display success modal after successful import commit`

**Prerequisites**:
- Container running with healthy API
- No pending import session

**Setup (API)**:
```typescript
// TestDataManager handles cleanup automatically
// No explicit setup needed - clean state guaranteed by fixture
```

**Steps**:
1. Navigate to `/tasks/import/caddyfile`
2. Paste valid Caddyfile content:
   ```
   success-modal-test.example.com {
       reverse_proxy localhost:3000
   }
   ```
3. Register response waiter for `/api/v1/import/upload`
4. Click "Parse and Review" button
5. Wait for review table to appear
6. Register response waiter for `/api/v1/import/commit`
7. Click "Commit Import" button
8. Wait for commit response

**Assertions**:
- `[data-testid="import-success-modal"]` is visible
- Modal contains text "Import Completed"
- Modal shows "1 host created" or similar count

**Selectors**:
| Element | Selector |
|---------|----------|
| Success Modal | `[data-testid="import-success-modal"]` |
| Commit Button | `page.getByRole('button', { name: /commit/i })` |
| Modal Header | `page.getByTestId('import-success-modal').locator('h2')` |

---

### Test Case 1.2: "View Proxy Hosts" button navigation

**Title**: `should navigate to /proxy-hosts when clicking View Proxy Hosts button`

**Prerequisites**:
- Success modal visible (chain from 1.1 or re-setup)

**Setup (API)**:
```typescript
// TestDataManager provides automatic cleanup
// Use helper function to complete import flow
```

**Steps**:
1. Complete import flow (reuse helper or inline steps from 1.1)
2. Wait for success modal to appear
3. Click "View Proxy Hosts" button

**Assertions**:
- `page.url()` ends with `/proxy-hosts`
- Success modal is no longer visible

**Selectors**:
| Element | Selector |
|---------|----------|
| View Proxy Hosts Button | `button:has-text("View Proxy Hosts")` |

---

### Test Case 1.3: "Go to Dashboard" button navigation

**Title**: `should navigate to /dashboard when clicking Go to Dashboard button`

**Prerequisites**:
- Success modal visible

**Steps**:
1. Complete import flow
2. Wait for success modal to appear
3. Click "Go to Dashboard" button

**Assertions**:
- `page.url()` matches `/` or `/dashboard`
- Success modal is no longer visible

**Selectors**:
| Element | Selector |
|---------|----------|
| Dashboard Button | `button:has-text("Go to Dashboard")` |

---

### Test Case 1.4: "Close" button behavior

**Title**: `should close modal and stay on import page when clicking Close`

**Prerequisites**:
- Success modal visible

**Steps**:
1. Complete import flow
2. Wait for success modal to appear
3. Click "Close" button

**Assertions**:
- Success modal is NOT visible
- `page.url()` contains `/tasks/import/caddyfile`
- Page content shows import form (textarea visible)

**Selectors**:
| Element | Selector |
|---------|----------|
| Close Button | `button:has-text("Close")` inside modal |

---

## Gap 2: Conflict Details Expansion

**Priority**: 🟠 HIGH
**Complexity**: Complex

### Test Case 2.1: Conflict indicator and expand button display

**Title**: `should show conflict indicator and expand button for conflicting domain`

**Prerequisites**:
- Create existing proxy host via API first

**Setup (API)**:
```typescript
// Create existing host to generate conflict
// TestDataManager automatically namespaces domain to prevent parallel conflicts
const domain = testData.generateDomain('conflict-test');
const hostId = await testData.createProxyHost({
  name: 'Conflict Test Host',
  domain_names: [domain],
  forward_scheme: 'http',
  forward_host: 'localhost',
  forward_port: 8080,
  enabled: false,
});
// Cleanup handled automatically by TestDataManager fixture
```

**Steps**:
1. Navigate to `/tasks/import/caddyfile`
2. Paste Caddyfile with conflicting domain (use namespaced domain):
   ```typescript
   // Use the same namespaced domain from setup
   const caddyfile = `${domain} { reverse_proxy localhost:9000 }`;
   await page.locator('textarea').fill(caddyfile);
   ```
3. Click "Parse and Review" button
4. Wait for review table to appear

**Assertions**:
- Review table shows row with the namespaced domain
- Conflict indicator visible (yellow badge with text "Conflict")
- Expand button (▶) is visible in the row

**Selectors** (row-scoped):
| Element | Selector |
|---------|----------|
| Domain Row | `page.locator('tr').filter({ hasText: domain })` |
| Conflict Badge | `domainRow.locator('.text-yellow-400', { hasText: 'Conflict' })` |
| Expand Button | `domainRow.getByRole('button', { name: /expand/i })` |

---

### Test Case 2.2: Side-by-side comparison renders on expand

**Title**: `should display side-by-side configuration comparison when expanding conflict row`

**Prerequisites**:
- Same as 2.1 (existing host created)

**Steps**:
1-4: Same as 2.1
5. Click the expand button (▶) in the conflict row

**Assertions**:
- Expanded row appears below the conflict row
- "Current Configuration" section visible
- "Imported Configuration" section visible
- Current config shows port 8080
- Imported config shows port 9000

**Selectors**:
| Element | Selector |
|---------|----------|
| Current Config Section | `h4:has-text("Current Configuration")` |
| Imported Config Section | `h4:has-text("Imported Configuration")` |
| Expanded Row | `tr.bg-gray-900\\/30` |
| Port Display | `dd.font-mono` containing port number |

---

### Test Case 2.3: Recommendation text displays

**Title**: `should show recommendation text in expanded conflict details`

**Prerequisites**:
- Same as 2.1

**Steps**:
1-5: Same as 2.2
6. Verify recommendation section

**Assertions**:
- Recommendation box visible (blue left border)
- Text contains "Recommendation:"
- Text provides actionable guidance

**Selectors**:
| Element | Selector |
|---------|----------|
| Recommendation Box | `.border-l-4.border-blue-500` or element containing `💡 Recommendation:` |

---

## Gap 3: Overwrite Resolution Flow

**Priority**: 🟠 HIGH
**Complexity**: Complex

### Test Case 3.1: Replace with Imported updates existing host

**Title**: `should update existing host when selecting Replace with Imported resolution`

**Prerequisites**:
- Create existing host via API

**Setup (API)**:
```typescript
// Create host with initial config
// TestDataManager automatically namespaces domain
const domain = testData.generateDomain('overwrite-test');
const hostId = await testData.createProxyHost({
  name: 'Overwrite Test Host',
  domain_names: [domain],
  forward_scheme: 'http',
  forward_host: 'old-server',
  forward_port: 3000,
  enabled: false,
});
// Cleanup handled automatically
```

**Steps**:
1. Navigate to `/tasks/import/caddyfile`
2. Paste Caddyfile with same domain but different config:
   ```typescript
   // Use the same namespaced domain from setup
   const caddyfile = `${domain} { reverse_proxy new-server:9000 }`;
   await page.locator('textarea').fill(caddyfile);
   ```
3. Register response waiter for upload
4. Click "Parse and Review" button
5. Wait for review table
6. Find resolution dropdown for conflicting row
7. Select "Replace with Imported" option
8. Register response waiter for commit
9. Click "Commit Import" button
10. Wait for success modal

**Assertions**:
- Success modal appears
- Fetch the host via API: `GET /api/v1/proxy-hosts/{hostId}`
- Verify `forward_host` is `"new-server"`
- Verify `forward_port` is `9000`
- Verify no duplicate was created (only 1 host with this domain)

**Selectors** (row-scoped):
| Element | Selector |
|---------|----------|
| Domain Row | `page.locator('tr').filter({ hasText: domain })` |
| Resolution Dropdown | `domainRow.locator('select')` |
| Overwrite Option | `dropdown.selectOption({ label: /replace.*imported/i })` |

---

## Gap 4: Session Resume via Banner

**Priority**: 🟠 HIGH
**Complexity**: Medium

### Test Case 4.1: Banner appears for pending session after navigation

**Title**: `should show pending session banner when returning to import page`

**Prerequisites**:
- No existing session

**Steps**:
1. Navigate to `/tasks/import/caddyfile`
2. Paste valid Caddyfile with namespaced domain:
   ```typescript
   const domain = testData.generateDomain('session-resume-test');
   const caddyfile = `${domain} { reverse_proxy localhost:4000 }`;
   await page.locator('textarea').fill(caddyfile);
   ```
3. Click "Parse and Review" button
4. Wait for review table to appear (session now created)
5. Navigate away: `page.goto('/proxy-hosts')`
6. Navigate back: `page.goto('/tasks/import/caddyfile')`

**Assertions**:
- `[data-testid="import-banner"]` is visible
- Banner contains text "Pending Import Session"
- "Review Changes" button is visible
- Review table is NOT visible (until clicking Review Changes)

**Selectors**:
| Element | Selector |
|---------|----------|
| Import Banner | `[data-testid="import-banner"]` |
| Banner Text | Text containing "Pending Import Session" |
| Review Changes Button | `button:has-text("Review Changes")` |

#### 8.3 Linter Configuration

**Verify gopls/staticcheck:**
- Build tags are standard Go feature
- No linter configuration changes needed
- GoReleaser will compile each platform separately

---

### Test Case 4.2: Review Changes button restores review table

**Title**: `should restore review table with previous content when clicking Review Changes`

**Prerequisites**:
- Pending session exists (from 4.1)

**Steps**:
1-6: Same as 4.1
7. Click "Review Changes" button in banner

**Assertions**:
- Review table becomes visible
- Table contains the namespaced domain from original upload
- Banner is no longer visible (or integrated into review)

**Selectors**:
| Element | Selector |
|---------|----------|
| Review Table | `page.getByTestId('import-review-table')` |
| Domain in Table | `page.getByTestId('import-review-table').getByText(domain)` |

### GoReleaser Integration

## Gap 5: Name Editing in Review

**Priority**: 🟡 MEDIUM
**Complexity**: Simple

### Test Case 5.1: Custom name is saved on commit

**Title**: `should create proxy host with custom name from review table input`

**Steps**:
1. Navigate to `/tasks/import/caddyfile`
2. Paste valid Caddyfile with namespaced domain:
   ```typescript
   const domain = testData.generateDomain('custom-name-test');
   const caddyfile = `${domain} { reverse_proxy localhost:5000 }`;
   await page.locator('textarea').fill(caddyfile);
   ```
3. Click "Parse and Review" button
4. Wait for review table
5. Find the name input field in the row
6. Clear and fill with custom name: `My Custom Proxy Name`
7. Click "Commit Import" button
8. Wait for success modal
9. Close modal

**Assertions**:
- Fetch all proxy hosts: `GET /api/v1/proxy-hosts`
- Find host with the namespaced domain
- Verify `name` field equals `"My Custom Proxy Name"`

**Selectors** (row-scoped):
| Element | Selector |
|---------|----------|
| Domain Row | `page.locator('tr').filter({ hasText: domain })` |
| Name Input | `domainRow.locator('input[type="text"]')` |
| Commit Button | `page.getByRole('button', { name: /commit/i })` |

---

## Required Data-TestId Additions

These `data-testid` attributes would improve test reliability:

| Component | Recommended TestId | Location | Notes |
|-----------|-------------------|----------|-------|
| Success Modal | `import-success-modal` | `ImportSuccessModal.tsx` | Already exists |
| View Proxy Hosts Button | `success-modal-view-hosts` | `ImportSuccessModal.tsx` line ~85 | Static testid |
| Go to Dashboard Button | `success-modal-dashboard` | `ImportSuccessModal.tsx` line ~90 | Static testid |
| Close Button | `success-modal-close` | `ImportSuccessModal.tsx` line ~80 | Static testid |
| Review Changes Button | `banner-review-changes` | `ImportBanner.tsx` line ~14 | Static testid |
| Import Banner | `import-banner` | `ImportBanner.tsx` | Static testid |
| Review Table | `import-review-table` | `ImportReviewTable.tsx` | Static testid |

**Note**: Avoid dynamic testids like `name-input-{domain}`. Instead, use structural locators that scope to rows first, then find elements within.

---

## Test File Structure

```typescript
// tests/tasks/caddy-import-gaps.spec.ts

import { test, expect } from '../fixtures/auth-fixtures';

test.describe('Caddy Import Gap Coverage @caddy-import-gaps', () => {

  test.describe('Success Modal Navigation', () => {
    test('should display success modal after successful import commit', async ({ page, testData }) => {
      // testData provides automatic cleanup and namespace isolation
      const domain = testData.generateDomain('success-modal-test');
      await completeImportFlow(page, testData, `${domain} { reverse_proxy localhost:3000 }`);

      await expect(page.getByTestId('import-success-modal')).toBeVisible();
      await expect(page.getByTestId('import-success-modal')).toContainText('Import Completed');
    });
    // 1.2, 1.3, 1.4
  });

  test.describe('Conflict Details Expansion', () => {
    // 2.1, 2.2, 2.3
  });

  test.describe('Overwrite Resolution Flow', () => {
    // 3.1
  });

  test.describe('Session Resume via Banner', () => {
    // 4.1, 4.2
  });

  test.describe('Name Editing in Review', () => {
    // 5.1
  });

});
```

---

## Complexity Summary

| Test Case | Complexity | API Setup Required | Cleanup Required |
|-----------|------------|-------------------|------------------|
| 1.1 Success modal appears | Medium | None (TestDataManager) | Automatic |
| 1.2 View Proxy Hosts nav | Medium | None (TestDataManager) | Automatic |
| 1.3 Dashboard nav | Medium | None (TestDataManager) | Automatic |
| 1.4 Close button | Medium | None (TestDataManager) | Automatic |
| 2.1 Conflict indicator | Complex | Create host via testData | Automatic |
| 2.2 Side-by-side expand | Complex | Create host via testData | Automatic |
| 2.3 Recommendation text | Complex | Create host via testData | Automatic |
| 3.1 Overwrite resolution | Complex | Create host via testData | Automatic |
| 4.1 Banner appears | Medium | None | Automatic |
| 4.2 Review Changes click | Medium | None | Automatic |
| 5.1 Custom name commit | Simple | None | Automatic |

**Total**: 11 test cases
**Estimated Implementation Time**: 6-8 hours

**Rationale for Time Increase**:
- TestDataManager integration requires understanding fixture patterns
- Row-scoped locator strategies more complex than simple testids
- Parallel execution validation with namespacing
- Additional validation for automatic cleanup

---

## Helper Functions to Create

```typescript
import type { Page } from '@playwright/test';
import type { TestDataManager } from '../fixtures/auth-fixtures';

// Helper to complete import and return to success modal
// Uses TestDataManager for automatic cleanup
async function completeImportFlow(
  page: Page,
  testData: TestDataManager,
  caddyfile: string
): Promise<void> {
  await page.goto('/tasks/import/caddyfile');
  await page.locator('textarea').fill(caddyfile);

  const uploadPromise = page.waitForResponse(r =>
    r.url().includes('/api/v1/import/upload') && r.status() === 200
  );
  await page.getByRole('button', { name: /parse|review/i }).click();
  await uploadPromise;

  await expect(page.getByTestId('import-review-table')).toBeVisible();

  const commitPromise = page.waitForResponse(r =>
    r.url().includes('/api/v1/import/commit') && r.status() === 200
  );
  await page.getByRole('button', { name: /commit/i }).click();
  await commitPromise;

  await expect(page.getByTestId('import-success-modal')).toBeVisible();
}

// Note: TestDataManager already provides createProxyHost() method
// No need for standalone helper - use testData.createProxyHost() directly
// Example:
//   const hostId = await testData.createProxyHost({
//     name: 'Test Host',
//     domain_names: [testData.generateDomain('test')],
//     forward_scheme: 'http',
//     forward_host: 'localhost',
//     forward_port: 8080,
//     enabled: false,
//   });

// Note: TestDataManager handles cleanup automatically
// No manual cleanup helper needed
```
## Complexity Summary

| Test Case | Complexity | API Setup Required | Cleanup Required |
|-----------|------------|-------------------|------------------|
| 1.1 Success modal appears | Medium | None (TestDataManager) | Automatic |
| 1.2 View Proxy Hosts nav | Medium | None (TestDataManager) | Automatic |
| 1.3 Dashboard nav | Medium | None (TestDataManager) | Automatic |
| 1.4 Close button | Medium | None (TestDataManager) | Automatic |
| 2.1 Conflict indicator | Complex | Create host via testData | Automatic |
| 2.2 Side-by-side expand | Complex | Create host via testData | Automatic |
| 2.3 Recommendation text | Complex | Create host via testData | Automatic |
| 3.1 Overwrite resolution | Complex | Create host via testData | Automatic |
| 4.1 Banner appears | Medium | None | Automatic |
| 4.2 Review Changes click | Medium | None | Automatic |
| 5.1 Custom name commit | Simple | None | Automatic |

**Total**: 11 test cases
**Estimated Implementation Time**: 6-8 hours

**Rationale for Time Increase**:
- TestDataManager integration requires understanding fixture patterns
- Row-scoped locator strategies more complex than simple testids
- Parallel execution validation with namespacing
- Additional validation for automatic cleanup

---

## Helper Functions to Create

```typescript
import type { Page } from '@playwright/test';
import type { TestDataManager } from '../fixtures/auth-fixtures';

// Helper to complete import and return to success modal
// Uses TestDataManager for automatic cleanup
async function completeImportFlow(
  page: Page,
  testData: TestDataManager,
  caddyfile: string
): Promise<void> {
  await page.goto('/tasks/import/caddyfile');
  await page.locator('textarea').fill(caddyfile);

  const uploadPromise = page.waitForResponse(r =>
    r.url().includes('/api/v1/import/upload') && r.status() === 200
  );
  await page.getByRole('button', { name: /parse|review/i }).click();
  await uploadPromise;

  await expect(page.getByTestId('import-review-table')).toBeVisible();

  const commitPromise = page.waitForResponse(r =>
    r.url().includes('/api/v1/import/commit') && r.status() === 200
  );
  await page.getByRole('button', { name: /commit/i }).click();
  await commitPromise;

  await expect(page.getByTestId('import-success-modal')).toBeVisible();
}

// Note: TestDataManager already provides createProxyHost() method
// No need for standalone helper - use testData.createProxyHost() directly
// Example:
//   const hostId = await testData.createProxyHost({
//     name: 'Test Host',
//     domain_names: [testData.generateDomain('test')],
//     forward_scheme: 'http',
//     forward_host: 'localhost',
//     forward_port: 8080,
//     enabled: false,
//   });

// Note: TestDataManager handles cleanup automatically
// No manual cleanup helper needed
```

---

## Acceptance Criteria

- [ ] All 11 test cases pass consistently (no flakiness)
- [ ] Tests use stored auth state (no login calls)
- [ ] Response waiters registered before click actions
- [ ] **All resources cleaned up automatically via TestDataManager fixtures**
- [ ] **Tests use `testData` fixture from `auth-fixtures.ts`**
- [ ] **No hardcoded domains (use TestDataManager's namespacing)**
- [ ] **Selectors use row-scoped patterns (filter by domain, then find within row)**
- [ ] **Relative paths in API calls (no `${baseURL}` interpolation)**
- [ ] Tests can run in parallel within their describe blocks
- [ ] Total test runtime < 60 seconds
- [ ] All 11 test cases pass consistently (no flakiness)
- [ ] Tests use stored auth state (no login calls)
- [ ] Response waiters registered before click actions
- [ ] **All resources cleaned up automatically via TestDataManager fixtures**
- [ ] **Tests use `testData` fixture from `auth-fixtures.ts`**
- [ ] **No hardcoded domains (use TestDataManager's namespacing)**
- [ ] **Selectors use row-scoped patterns (filter by domain, then find within row)**
- [ ] **Relative paths in API calls (no `${baseURL}` interpolation)**
- [ ] Tests can run in parallel within their describe blocks
- [ ] Total test runtime < 60 seconds

---

## Requirements (EARS Notation)

1. WHEN the import commit succeeds, THE SYSTEM SHALL display the success modal with created/updated/skipped counts.
2. WHEN clicking "View Proxy Hosts" in the success modal, THE SYSTEM SHALL navigate to `/proxy-hosts`.
3. WHEN clicking "Go to Dashboard" in the success modal, THE SYSTEM SHALL navigate to the dashboard (`/`).
4. WHEN clicking "Close" in the success modal, THE SYSTEM SHALL close the modal and remain on the import page.
5. WHEN importing a Caddyfile with a domain that already exists, THE SYSTEM SHALL display a conflict indicator.
6. WHEN expanding a conflict row, THE SYSTEM SHALL show side-by-side comparison of current vs imported configuration.
7. WHEN selecting "Replace with Imported" resolution and committing, THE SYSTEM SHALL update the existing host.
8. WHEN a pending import session exists, THE SYSTEM SHALL display a yellow banner with "Review Changes" button.
9. WHEN clicking "Review Changes" on the session banner, THE SYSTEM SHALL restore the review table with previous content.
10. WHEN editing the name field in the review table, THE SYSTEM SHALL use that custom name when creating the proxy host.

---

## ARCHIVED: Previous Spec

The GoReleaser v2 Migration spec previously in this file has been archived to `docs/plans/archived/goreleaser_v2_migration.md`.
## Requirements (EARS Notation)

1. WHEN the import commit succeeds, THE SYSTEM SHALL display the success modal with created/updated/skipped counts.
2. WHEN clicking "View Proxy Hosts" in the success modal, THE SYSTEM SHALL navigate to `/proxy-hosts`.
3. WHEN clicking "Go to Dashboard" in the success modal, THE SYSTEM SHALL navigate to the dashboard (`/`).
4. WHEN clicking "Close" in the success modal, THE SYSTEM SHALL close the modal and remain on the import page.
5. WHEN importing a Caddyfile with a domain that already exists, THE SYSTEM SHALL display a conflict indicator.
6. WHEN expanding a conflict row, THE SYSTEM SHALL show side-by-side comparison of current vs imported configuration.
7. WHEN selecting "Replace with Imported" resolution and committing, THE SYSTEM SHALL update the existing host.
8. WHEN a pending import session exists, THE SYSTEM SHALL display a yellow banner with "Review Changes" button.
9. WHEN clicking "Review Changes" on the session banner, THE SYSTEM SHALL restore the review table with previous content.
10. WHEN editing the name field in the review table, THE SYSTEM SHALL use that custom name when creating the proxy host.

---

## ARCHIVED: Previous Spec

The GoReleaser v2 Migration spec previously in this file has been archived to `docs/plans/archived/goreleaser_v2_migration.md`.
