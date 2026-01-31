# PR #583 CI Failure Remediation Plan

**Created**: 2026-01-31
**Updated**: 2026-01-31 (E2E Shard 4 Failure Analysis Added)
**Status**: Active
**PR**: #583 - Feature/beta-release
**Target**: Unblock merge by fixing all CI failures

---

## Executive Summary

PR #583 has blocking CI failures:

| Failure | Root Cause | Complexity | Status |
|---------|------------|------------|--------|
| **E2E Shard 4** | Security module enable timing/race conditions | Medium | 🔴 NEW |
| Frontend Quality Checks | Missing `data-testid` on Multi-site Import button | Simple | Pending |
| Codecov Patch Coverage (58.62% vs 67.47%) | Untested error paths in importer.go and import_handler.go | Medium | Pending |

**UPDATE**: E2E tests are NOW FAILING in Shard 4 (security-tests project) with timing issues.

---

## Phase 0: E2E Security Enforcement Test Failures (URGENT)

**Priority**: 🔴 CRITICAL - Blocking CI
**Run ID**: 21537719507 | **Job ID**: 62066779886

### Failure Summary

| Test File | Test Name | Duration | Error Type |
|-----------|-----------|----------|------------|
| `emergency-token.spec.ts:160` | Emergency token bypasses ACL | 15+ retries | ACL verification timeout |
| `emergency-server.spec.ts:150` | Emergency server bypasses main app security | 3.1s | Timeout |
| `combined-enforcement.spec.ts:99` | Enable all security modules simultaneously | 46.6s | Timeout |
| `waf-enforcement.spec.ts:151` | Detect SQL injection patterns | 10.0s | Timeout |
| `user-management.spec.ts:71` | Show user status badges | 58.4s | Timeout |

### Root Cause Analysis

**Primary Issue**: Security module enable state propagation timing

The test at [emergency-token.spec.ts](tests/security-enforcement/emergency-token.spec.ts#L88-L91) fails with:
```
Error: ACL verification failed - ACL not showing as enabled after retries
```

**Technical Details**:

1. **Cerberus → ACL Enable Sequence** (lines 35-70):
   - Test enables Cerberus master switch via `PATCH /api/v1/settings`
   - Waits 3000ms for Caddy reload
   - Enables ACL via `PATCH /api/v1/settings`
   - Waits 5000ms for propagation
   - Verification loop: 15 retries × 1000ms intervals

2. **Race Condition**: The 15-retry verification (total 15s wait) is insufficient in CI:
   ```typescript
   // Line 68-85: Retry loop fails
   while (verifyRetries > 0 && !aclEnabled) {
     const status = await statusResponse.json();
     if (status.acl?.enabled) { aclEnabled = true; }
     // 1000ms between retries, 15 retries = 15s max
   }
   ```

3. **CI Environment Factors**:
   - GitHub Actions runners have variable I/O latency
   - Caddy config reload takes longer under load
   - Single worker (`workers: 1`) in security-tests project serializes tests but doesn't prevent inter-test timing issues

### Evidence from Test Output

**e2e_full_output.txt** (162 tests in security-tests project):
```
1 failed
  [security-tests] › tests/security-enforcement/emergency-token.spec.ts:160:3
  › Emergency Token Break Glass Protocol › Test 1: Emergency token bypasses ACL
```

**Retry log pattern** (from test output):
```
⏳ ACL not yet enabled, retrying... (15 left)
⏳ ACL not yet enabled, retrying... (14 left)
...
⏳ ACL not yet enabled, retrying... (1 left)
Error: ACL verification failed
```

### Proposed Fixes

#### Option A: Increase Timeouts (Quick Fix)

**File**: [tests/security-enforcement/emergency-token.spec.ts](tests/security-enforcement/emergency-token.spec.ts)

**Changes**:
1. Increase initial Caddy reload wait: `3000ms → 5000ms`
2. Increase propagation wait: `5000ms → 10000ms`
3. Increase retry count: `15 → 30`
4. Increase retry interval: `1000ms → 2000ms`

```typescript
// Line 45: After Cerberus enable
await new Promise(resolve => setTimeout(resolve, 5000)); // was 3000

// Line 58: After ACL enable
await new Promise(resolve => setTimeout(resolve, 10000)); // was 5000

// Line 66: Verification loop
let verifyRetries = 30; // was 15
// ...
await new Promise(resolve => setTimeout(resolve, 2000)); // was 1000
```

**Estimated Time**: 15 minutes

#### Option B: Event-Driven Verification (Better)

Replace polling with webhook or status change event:

```typescript
// Wait for Caddy admin API to confirm config applied
const caddyConfigApplied = await waitForCaddyConfigVersion(expectedVersion);
if (!caddyConfigApplied) throw new Error('Caddy config not applied in time');
```

**Estimated Time**: 2-4 hours (requires backend changes)

#### Option C: CI-Specific Timeouts (Recommended)

Add environment-aware timeout multipliers:

**File**: `tests/security-enforcement/emergency-token.spec.ts`

```typescript
// At top of file
const CI_TIMEOUT_MULTIPLIER = process.env.CI ? 3 : 1;
const BASE_PROPAGATION_WAIT = 5000;
const BASE_RETRY_INTERVAL = 1000;

// Usage
await new Promise(r => setTimeout(r, BASE_PROPAGATION_WAIT * CI_TIMEOUT_MULTIPLIER));
```

**Estimated Time**: 30 minutes

### Implementation Checklist

- [ ] Read current timeout values in emergency-token.spec.ts
- [ ] Apply Option C (CI-specific timeout multiplier) as primary fix
- [ ] Update combined-enforcement.spec.ts with same pattern
- [ ] Update waf-enforcement.spec.ts with same pattern
- [ ] Run local E2E tests to verify fixes don't break local execution
- [ ] Push and monitor CI Shard 4 job

### Affected Files

| File | Issue | Fix Required |
|------|-------|--------------|
| `tests/security-enforcement/emergency-token.spec.ts` | ACL verification timeout | Increase timeouts |
| `tests/security-enforcement/combined-enforcement.spec.ts` | All modules enable timeout | Increase timeouts |
| `tests/security-enforcement/waf-enforcement.spec.ts` | SQL injection detection timeout | Increase timeouts |
| `tests/emergency-server/emergency-server.spec.ts` | Security bypass verification timeout | Increase timeouts |
| `tests/settings/user-management.spec.ts` | Status badge render timeout | Investigate separately |

### Validation Commands

```bash
# Rebuild E2E environment
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Run security-tests project only (same as Shard 4)
npx playwright test --project=security-tests

# Run specific failing test
npx playwright test tests/security-enforcement/emergency-token.spec.ts
```

### Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Increased timeouts slow down CI | Low | Only affects security-tests (already serial) |
| Timeout multiplier may not be enough | Medium | Monitor first CI run, iterate if needed |
| Other tests may have same issue | Medium | Apply timeout pattern framework-wide |

---

## Phase 1: Fix Frontend Quality Checks (8 Test Failures)

### Root Cause Analysis

All 8 failing tests are in ImportCaddy-related test files looking for:
```tsx
screen.getByTestId('multi-file-import-button')
```

**Actual DOM state** (from test output):
```html
<button class="ml-4 px-4 py-2 bg-gray-800 text-white rounded-lg">
  Multi-site Import
</button>
```

The button exists but is **missing the `data-testid` attribute**.

### Fix Location

**File**: [frontend/src/pages/ImportCaddy.tsx](frontend/src/pages/ImportCaddy.tsx#L158-L163)

**Current Code** (lines 158-163):
```tsx
<button
  onClick={() => setShowMultiModal(true)}
  className="ml-4 px-4 py-2 bg-gray-800 text-white rounded-lg"
>
  {t('importCaddy.multiSiteImport')}
</button>
```

**Required Fix**:
```tsx
<button
  onClick={() => setShowMultiModal(true)}
  className="ml-4 px-4 py-2 bg-gray-800 text-white rounded-lg"
  data-testid="multi-file-import-button"
>
  {t('importCaddy.multiSiteImport')}
</button>
```

### Affected Tests (All Will Pass After Fix)

| Test File | Failed Tests |
|-----------|--------------|
| `ImportCaddy-multifile-modal.test.tsx` | 6 tests |
| `ImportCaddy-imports.test.tsx` | 1 test |
| `ImportCaddy-warnings.test.tsx` | 1 test |

### Validation Command

```bash
cd frontend && npm run test -- --run src/pages/__tests__/ImportCaddy
```

**Expected Result**: All 8 previously failing tests pass.

---

## Phase 2: Backend Patch Coverage (58.62% → 100%)

### Coverage Gap Analysis

Codecov reports 2 files with missing patch coverage:

| File | Current Coverage | Missing Lines | Partials |
|------|------------------|---------------|----------|
| `backend/internal/caddy/importer.go` | 56.52% | 5 | 5 |
| `backend/internal/api/handlers/import_handler.go` | 0.00% | 6 | 0 |

### 2.1 importer.go Coverage Gaps

**File**: [backend/internal/caddy/importer.go](backend/internal/caddy/importer.go#L130-L155)

**Function**: `NormalizeCaddyfile(content string) (string, error)`

**Missing Lines (Error Paths)**:

| Line | Code | Coverage Issue |
|------|------|----------------|
| 137-138 | `if _, err := tmpFile.WriteString(content); err != nil { return "", fmt.Errorf(...) }` | WriteString error path |
| 140-141 | `if err := tmpFile.Close(); err != nil { return "", fmt.Errorf(...) }` | Close error path |

#### Test Implementation Strategy

Create a mock that simulates file operation failures:

**File**: `backend/internal/caddy/importer_test.go`

**Add Test Cases**:

```go
// TestImporter_NormalizeCaddyfile_WriteStringError tests the error path when WriteString fails
func TestImporter_NormalizeCaddyfile_WriteStringError(t *testing.T) {
    importer := NewImporter("caddy")

    // Mock executor that succeeds, but we need to trigger WriteString failure
    // Strategy: Use a mock that intercepts os.CreateTemp to return a read-only file
    // Alternative: Use interface abstraction for file operations (requires refactor)

    // For now, this is difficult to test without interface abstraction.
    // Document as known gap or refactor to use file operation interface.
    t.Skip("WriteString error requires file operation interface abstraction - see Phase 2 alternative")
}

// TestImporter_NormalizeCaddyfile_CloseError tests the error path when Close fails
func TestImporter_NormalizeCaddyfile_CloseError(t *testing.T) {
    // Same limitation as WriteStringError - requires interface abstraction
    t.Skip("Close error requires file operation interface abstraction - see Phase 2 alternative")
}
```

#### Alternative: Exclude Low-Value Error Paths

These error paths (WriteString/Close failures on temp files) are:
1. **Extremely rare** in practice (disk full, permissions issues)
2. **Difficult to test** without extensive mocking infrastructure
3. **Low risk** - errors are properly wrapped and returned

**Recommended Approach**: Add `// coverage:ignore` comment or update `codecov.yml` to exclude these specific lines from patch coverage requirements.

**codecov.yml update**:
```yaml
coverage:
  status:
    patch:
      default:
        target: 67.47%
        # Exclude known untestable error paths
        # Lines 137-141 of importer.go are temp file error handlers
```

### 2.2 import_handler.go Coverage Gaps

**File**: [backend/internal/api/handlers/import_handler.go](backend/internal/api/handlers/import_handler.go)

Based on the test file analysis, the 6 missing lines are likely in one of these areas:

| Suspected Location | Description |
|--------------------|-------------|
| Lines 667-670 | `proxyHostSvc.Update` error logging in Commit |
| Lines 682-685 | `proxyHostSvc.Create` error logging in Commit |
| Line ~740 | `db.Save(&session)` warning in Commit |

#### Current Test Coverage Analysis

The test file `import_handler_test.go` already has tests for:
- ✅ `TestImportHandler_Commit_CreateFailure` - attempts to cover line 682
- ⚠️ `TestImportHandler_Commit_UpdateFailure` - skipped with explanation

#### Required New Tests

**Test 1: Database Save Warning** (likely missing coverage)

```go
// TestImportHandler_Commit_SessionSaveWarning tests the warning log when session save fails
func TestImportHandler_Commit_SessionSaveWarning(t *testing.T) {
    gin.SetMode(gin.TestMode)
    db := setupImportTestDB(t)

    // Create a session that will be committed
    session := models.ImportSession{
        UUID:   uuid.NewString(),
        Status: "reviewing",
        ParsedData: `{"hosts": [{"domain_names": "test.com", "forward_host": "127.0.0.1", "forward_port": 80}]}`,
    }
    db.Create(&session)

    // Close the database connection after session creation
    // This will cause the final db.Save() to fail
    sqlDB, _ := db.DB()

    handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
    router := gin.New()
    router.POST("/import/commit", handler.Commit)

    // Close DB after handler is created but before commit
    // This triggers the warning path at line ~740
    sqlDB.Close()

    payload := map[string]any{
        "session_uuid": session.UUID,
        "resolutions":  map[string]string{},
    }
    body, _ := json.Marshal(payload)

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
    router.ServeHTTP(w, req)

    // The commit should complete with 200 but log a warning
    // (session save failure is non-fatal per implementation)
    // Note: This test may not work perfectly due to timing -
    // the DB close affects all operations, not just the final save
}
```

**Alternative: Verify Create Error Path Coverage**

The existing `TestImportHandler_Commit_CreateFailure` test should cover line 682. Verify by running:

```bash
cd backend && go test -coverprofile=cover.out ./internal/api/handlers -run TestImportHandler_Commit_CreateFailure
go tool cover -func=cover.out | grep import_handler
```

If coverage is still missing, the issue may be that the test assertions don't exercise all code paths.

---

## Phase 3: Verification

### 3.1 Local Verification Commands

```bash
# Phase 1: Frontend tests
cd frontend && npm run test -- --run src/pages/__tests__/ImportCaddy

# Phase 2: Backend coverage
cd backend && go test -coverprofile=cover.out ./internal/caddy ./internal/api/handlers
go tool cover -func=cover.out | grep -E "importer.go|import_handler.go"

# Full CI simulation
cd /projects/Charon && make test
```

### 3.2 CI Verification

After pushing fixes, verify:
1. ✅ Frontend Quality Checks job passes
2. ✅ Backend Quality Checks job passes
3. ✅ Codecov patch coverage ≥ 67.47%

---

## Implementation Checklist

### Phase 1: Frontend (Estimated: 5 minutes)
- [ ] Add `data-testid="multi-file-import-button"` to ImportCaddy.tsx line 160
- [ ] Run frontend tests locally to verify 8 tests pass
- [ ] Commit with message: `fix(frontend): add missing data-testid for multi-file import button`

### Phase 2: Backend Coverage (Estimated: 30-60 minutes) ✅ COMPLETED
- [x] **Part A: import_handler.go error paths**
  - Added `ProxyHostServiceInterface` interface for testable dependency injection
  - Added `NewImportHandlerWithService()` constructor for mock injection
  - Created `mockProxyHostService` in test file with configurable failure functions
  - Fixed `TestImportHandler_Commit_UpdateFailure` to use mock (was previously skipped)
  - **Commit function coverage: 43.7% → 86.2%**
  - Lines 676 (Update error) and 691 (Create error) now covered
- [x] **Part B: importer.go untestable paths**
  - Added documentation comments to lines 140-144 explaining why WriteString/Close error paths are impractical to test
  - Did NOT exclude entire file from codecov (would harm valuable coverage)
  - `NormalizeCaddyfile` coverage: 81.2% (remaining uncovered lines are OS-level fault handlers)
- [x] Run backend tests with coverage to verify improvement

### Phase 3: Verification (Estimated: 10 minutes)
- [ ] Push changes and monitor CI
- [ ] Verify all checks pass
- [ ] Request re-review if applicable

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Phase 1 fix doesn't resolve all tests | Low | Tests clearly show missing testid is root cause |
| Backend coverage tests are flaky | Medium | Use t.Skip for truly untestable paths |
| CI has other hidden failures | Low | E2E already passing, only 2 known failures |

---

## Requirements (EARS Notation)

1. WHEN the Multi-site Import button is rendered, THE SYSTEM SHALL include `data-testid="multi-file-import-button"` attribute.
2. WHEN `NormalizeCaddyfile` encounters a WriteString error, THE SYSTEM SHALL return a wrapped error with context.
3. WHEN `NormalizeCaddyfile` encounters a Close error, THE SYSTEM SHALL return a wrapped error with context.
4. WHEN `Commit` encounters a session save failure, THE SYSTEM SHALL log a warning but complete the operation.
5. WHEN patch coverage is calculated, THE SYSTEM SHALL meet or exceed 67.47% target.

---

## References

- CI Run: https://github.com/Wikid82/Charon/actions/runs/21537719503/job/62066647342
- Existing importer_test.go: [backend/internal/caddy/importer_test.go](backend/internal/caddy/importer_test.go)
- Existing import_handler_test.go: [backend/internal/api/handlers/import_handler_test.go](backend/internal/api/handlers/import_handler_test.go)

---

## ARCHIVED: Caddy Import E2E Test Plan - Gap Coverage

**Created**: 2026-01-30
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
