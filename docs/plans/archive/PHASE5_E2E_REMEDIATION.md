# Phase 5 E2E Test Remediation Plan

## Executive Summary

**Test Run Results**: 120 tests total
- ✅ **88 passed**
- ⏭️ **24 skipped** (Cerberus disabled - expected behavior)
- ❌ **8 failed** (all timeout-related)

This is **not 99 failing tests** as initially estimated. The actual failures are concentrated in a specific pattern: **API response wait timeouts**.

---

## 1. Failure Summary by Category

| Category | Count | Tests |
|----------|-------|-------|
| API Response Timeout | 8 | All failures are `waitForAPIResponse` timeouts |
| Missing Selectors | 0 | All selectors match actual DOM |
| Skipped (Cerberus disabled) | 24 | Real-time logs tests - expected behavior |

---

## 2. Root Cause Analysis

### Primary Issue: API Response Wait Pattern

All 8 failing tests share the same failure pattern:

```
Error: page.waitForResponse: Test timeout of 30000ms exceeded.
at utils/wait-helpers.ts:89
```

**Root Cause**: The tests set up API route mocks correctly, but the `waitForAPIResponse()` call happens **after** the action that triggers the API call. This creates a race condition where:

1. Test clicks button → API call fires
2. Route mock intercepts and fulfills immediately
3. `waitForAPIResponse()` starts waiting but the response already happened
4. Test times out after 30s

### Affected Tests:

| Test File | Test Name | API Endpoint |
|-----------|-----------|--------------|
| `uptime-monitoring.spec.ts:612` | should trigger manual health check | `/api/v1/uptime/monitors/1/check` |
| `backups-create.spec.ts:186` | should create a new backup successfully | `/api/v1/backups` (POST) |
| `backups-create.spec.ts:250` | should update backup list with new backup | `/api/v1/backups` (POST) |
| `backups-restore.spec.ts:157` | should restore backup successfully after confirmation | `/api/v1/backups/{filename}/restore` |
| `import-crowdsec.spec.ts:180` | should create backup before import and complete successfully | `/api/v1/crowdsec/import` |
| `import-crowdsec.spec.ts:237` | should handle import errors gracefully | `/api/v1/crowdsec/import` |
| `import-crowdsec.spec.ts:281` | should show loading state during import | `/api/v1/crowdsec/import` |
| `logs-viewing.spec.ts:418` | should paginate large log files | `/api/v1/logs/access.log` |

---

## 3. Secondary Issue: CrowdSec API Path Mismatch

The import-crowdsec tests mock the wrong API endpoint:

| Component | Actual API Path | Test Mock Path |
|-----------|-----------------|----------------|
| ImportCrowdSec.tsx | `/api/v1/admin/crowdsec/import` | `/api/v1/crowdsec/import` |

The frontend uses `client.post('/admin/crowdsec/import', ...)` which becomes `/api/v1/admin/crowdsec/import`.

---

## 4. Required Fixes

### Fix Category A: Race Condition in waitForAPIResponse (8 tests)

**Pattern to Fix**: Change from:

```typescript
// ❌ BROKEN: Race condition
await page.click(SELECTORS.createBackupButton);
await waitForAPIResponse(page, '/api/v1/backups', { status: 201 });
```

**To**:

```typescript
// ✅ FIXED: Set up listener before action
const responsePromise = page.waitForResponse(
  (response) => response.url().includes('/api/v1/backups') && response.status() === 201
);
await page.click(SELECTORS.createBackupButton);
await responsePromise;
```

**Or use Promise.all pattern**:

```typescript
// ✅ FIXED: Parallel wait and action
await Promise.all([
  page.waitForResponse(resp => resp.url().includes('/api/v1/backups') && resp.status() === 201),
  page.click(SELECTORS.createBackupButton),
]);
```

### Fix Category B: CrowdSec API Path (3 tests)

**Files**: `tests/tasks/import-crowdsec.spec.ts`

**Change**:
- `**/api/v1/crowdsec/import` → `**/api/v1/admin/crowdsec/import`
- `waitForAPIResponse(page, '/api/v1/crowdsec/import', ...)` → `waitForAPIResponse(page, '/api/v1/admin/crowdsec/import', ...)`

---

## 5. Detailed Fix List by File

### `tests/tasks/backups-create.spec.ts`

| Line | Current | Fix |
|------|---------|-----|
| 212-215 | `await page.click(...); await waitForAPIResponse(...)` | Use Promise.all or responsePromise pattern |
| 281-284 | Same pattern | Same fix |

### `tests/tasks/backups-restore.spec.ts`

| Line | Current | Fix |
|------|---------|-----|
| 196-199 | `await confirmButton.click(); await waitForAPIResponse(...)` | Use Promise.all or responsePromise pattern |

### `tests/tasks/import-crowdsec.spec.ts`

| Line | Current | Fix |
|------|---------|-----|
| 108 | `**/api/v1/crowdsec/import` | `**/api/v1/admin/crowdsec/import` |
| 144 | Same | Same |
| 202 | Same | Same |
| 226 | `'/api/v1/crowdsec/import'` | `'/api/v1/admin/crowdsec/import'` |
| 253 | Same route pattern | Same fix |
| 275 | Same waitForAPIResponse | Same fix |
| 298 | Same route pattern | Same fix |
| 325 | Same waitForAPIResponse | Same fix |
| 223 | Click + waitForAPIResponse race | Use Promise.all pattern |
| 272 | Same | Same |
| 318 | Same | Same |

### `tests/tasks/logs-viewing.spec.ts`

| Line | Current | Fix |
|------|---------|-----|
| 449-458 | `nextButton.click(); await waitForAPIResponse(...)` | Use Promise.all pattern |

### `tests/monitoring/uptime-monitoring.spec.ts`

| Line | Current | Fix |
|------|---------|-----|
| 630-634 | `refreshButton.click(); await waitForAPIResponse(...)` | Use Promise.all pattern |

---

## 6. Fix Priority Order

1. **HIGH - Helper Pattern Fix** (impacts all tests):
   - Update `tests/utils/wait-helpers.ts` to document the race condition issue
   - Consider adding a new helper `clickAndWaitForResponse()` that handles this correctly

2. **HIGH - API Path Fix** (3 tests):
   - Fix CrowdSec import endpoint path in `import-crowdsec.spec.ts`

3. **MEDIUM - Individual Test Fixes** (8 tests):
   - Apply Promise.all pattern to each affected test

---

## 7. No Changes Needed

### Frontend Components (All selectors match)

The following components have all required `data-testid` attributes:

| Component | Verified Attributes |
|-----------|---------------------|
| `Backups.tsx` | `loading-skeleton`, `empty-state`, `backup-table`, `backup-row`, `backup-download-btn`, `backup-restore-btn`, `backup-delete-btn` |
| `ImportCrowdSec.tsx` | `crowdsec-import-file`, `import-progress` |
| `Uptime.tsx` | `monitor-card`, `status-badge`, `last-check`, `heartbeat-bar`, `uptime-summary`, `sync-button`, `add-monitor-button` |

### Skipped Tests (24 tests - Expected)

The real-time logs tests are correctly skipped when Cerberus is disabled:
```typescript
test.skip(!cerberusEnabled, 'LiveLogViewer not available - Cerberus security module is disabled');
```

This is correct behavior - these tests should only run when the Cerberus security module is enabled.

---

## 8. Estimated Effort

| Task | Effort | Tests Fixed |
|------|--------|-------------|
| Update helper documentation | 15 min | 0 (prevention) |
| Create `clickAndWaitForResponse` helper | 30 min | 0 (infrastructure) |
| Fix CrowdSec API paths | 10 min | 3 |
| Apply Promise.all to backups tests | 15 min | 3 |
| Apply Promise.all to logs test | 5 min | 1 |
| Apply Promise.all to uptime test | 5 min | 1 |
| **Total** | **~1.5 hours** | **8 tests** |

---

## 9. Verification Steps

After fixes are applied:

```bash
# Run specific failing tests
npx playwright test tests/tasks/backups-create.spec.ts tests/tasks/backups-restore.spec.ts tests/tasks/import-crowdsec.spec.ts tests/tasks/logs-viewing.spec.ts tests/monitoring/uptime-monitoring.spec.ts --project=chromium

# Expected result: All 96 non-skipped tests should pass
```

---

## 10. Cleanup Errors (Non-blocking)

One test showed a cleanup warning:
```
Failed to cleanup user:5269: Error: Failed to delete user: {"error":"Admin access required"}
```

This is a **fixture cleanup issue**, not a test failure. The test itself passed. This should be addressed by ensuring the test cleanup runs with admin privileges, but it's not blocking.
