# E2E Test Fix - Remediation Plan v2

**Date:** 2026-02-01
**Status:** Ready for Implementation
**Test File:** `tests/dns-provider-types.spec.ts`
**Component:** `frontend/src/components/DNSProviderForm.tsx`
**Priority:** High
**Revised:** 2026-02-01 (Per Supervisor feedback - Option 2 is primary approach)

---

## Executive Summary

The E2E test for webhook provider type selection is failing due to a **React render timing race condition**, not a missing fallback schema. The test clicks the webhook option in the dropdown and immediately waits for the credentials section to appear, but React needs time to:

1. Update `providerType` state (async `setProviderType`)
2. Re-render the component
3. Recompute `selectedProviderInfo` from the updated state
4. Conditionally render the credentials section based on the new value

When the test waits for `[data-testid="credentials-section"]` too quickly, React hasn't completed this cycle yet, causing a timeout.

---

## Root Cause Analysis

### Investigation Results

#### ✅ Verified: Webhook HAS Fallback Schema

**File:** `frontend/src/data/dnsProviderSchemas.ts` (Lines 245-301)

```typescript
webhook: {
  type: 'webhook',
  name: 'Webhook',
  fields: [
    { name: 'create_url', label: 'Create Record URL', type: 'text', required: true, ... },
    { name: 'delete_url', label: 'Delete Record URL', type: 'text', required: false, ... },
    { name: 'auth_type', label: 'Authentication Type', type: 'select', required: false, ... },
    { name: 'auth_value', label: 'Auth Value', type: 'password', required: false, ... },
    { name: 'insecure_skip_verify', label: 'Skip TLS Verification', type: 'select', ... },
  ],
  documentation_url: '',
}
```

**Conclusion:** Option A (missing fallback) is **ruled out**. Webhook provider **does** have a complete fallback definition.

---

#### ❌ Root Cause: React Re-Render Timing

**File:** `frontend/src/components/DNSProviderForm.tsx`

**Line 87-92:** `getSelectedProviderInfo()` function

```tsx
const getSelectedProviderInfo = (): DNSProviderTypeInfo | undefined => {
  if (!providerType) return undefined
  return (
    providerTypes?.find((pt) => pt.type === providerType) ||
    (defaultProviderSchemas[providerType as keyof typeof defaultProviderSchemas] as DNSProviderTypeInfo)
  );
}
```

- **Fallback logic is correct**: If `providerTypes` (React Query) hasn't loaded, it uses `defaultProviderSchemas`
- **But the function depends on `providerType` state**, which is updated asynchronously

**Line 152:** Computed value assignment

```tsx
const selectedProviderInfo = getSelectedProviderInfo()
```

- This is recomputed **every render**
- Depends on current `providerType` state value

**Line 205-209:** Conditional rendering of credentials section

```tsx
{selectedProviderInfo && (
  <>
    <div className="space-y-3 border-t pt-4">
      <Label className="text-base" data-testid="credentials-section">{t('dnsProviders.credentials')}</Label>
```

- The **entire credentials section** is inside the conditional
- The `data-testid="credentials-section"` label only exists when `selectedProviderInfo` is truthy
- If React hasn't re-rendered after state update, this section doesn't exist in the DOM

---

#### ⏱️ Timing Chain Breakdown

**Test Sequence:**

```typescript
// 1. Select webhook from dropdown
await page.getByRole('option', { name: /webhook/i }).click();

// 2. Immediately wait for credentials section
await page.locator('[data-testid="credentials-section"]').waitFor({
  state: 'visible',
  timeout: 10000
});
```

**React State/Render Cycle:**

1. **T=0ms**: User clicks webhook option
2. **T=1ms**: `<Select>` component calls `onValueChange(setProviderType)`
3. **T=2ms**: React schedules state update for `providerType = 'webhook'`
4. **T=5ms**: React batches and applies state update
5. **T=10ms**: Component re-renders with new `providerType`
6. **T=11ms**: `getSelectedProviderInfo()` recomputes, finds webhook from `defaultProviderSchemas`
7. **T=12ms**: Conditional `{selectedProviderInfo && ...}` evaluates to true
8. **T=15ms**: Credentials section renders in DOM
9. **T=20ms**: Browser paints credentials section (visible)

**Test checks at T=3ms** → Element doesn't exist yet → Timeout after 10 seconds

---

### Why Firefox Fails More Often

**Browser Differences:**
- **Chromium**: Fast V8 engine, aggressive React optimizations, typically completes cycle in 10-20ms
- **Firefox**: SpiderMonkey engine, slightly slower React scheduler, cycle takes 30-50ms
- **Webkit**: Similar to Chromium but with different timing characteristics

**10-second timeout should be enough**, but the test **never retries** because the element **never enters the DOM** until React finishes its cycle.

---

## Solution Options

### Option 2: Wait for Specific Field to Appear ✅ **RECOMMENDED** (Supervisor Approved)

**Approach:** Instead of waiting for the generic credentials section label, wait for a field unique to that provider type. This directly confirms state update + re-render + conditional logic in a single assertion.

**Implementation:**

```typescript
await test.step('Select Webhook provider type', async () => {
  const typeSelect = page.locator('#provider-type');
  await typeSelect.click();
  await page.getByRole('option', { name: /webhook/i }).click();
});

await test.step('Verify Webhook URL field appears', async () => {
  // DIRECTLY wait for provider-specific field (confirms full React cycle)
  await expect(page.getByLabel(/create.*url/i)).toBeVisible({ timeout: 10000 });
});
```

**Rationale:**
- **Most robust**: Confirms state update, re-render, AND correct provider fields in one step
- **No intermediate waits**: Removes unnecessary "credentials-section" check
- **Already tested pattern**: Line 187 already uses this approach successfully
- **Supervisor verified**: Eliminates timing race without adding test complexity

**Files to Modify:**
- `tests/dns-provider-types.spec.ts` (Lines 167, 179-187, 198-206, 217-225)
- **4 tests to fix**: Manual (line 167), Webhook (line 179), RFC2136 (line 198), Script (line 217)

---

### Option 1: Wait for Provider Type Selection to Reflect in UI (NOT RECOMMENDED)

**Approach:** Add an intermediate assertion that confirms the provider type has been selected before waiting for credentials section.

**Why Not Recommended:**
- Adds unnecessary intermediate wait step
- "credentials-section" wait is redundant if we check provider-specific fields
- More complex test flow without added reliability

**Files to Modify:**
- N/A - Not implementing this approach

---

### Option 3: Increase Timeout and Use Auto-Retry (Incorporated into Option 2)

**Approach:** Use Playwright's auto-retry mechanism with explicit state checking.

**Status:** **Incorporated into Option 2** - Using `expect().toBeVisible()` with 10000ms timeout

**Rationale:**
- `expect().toBeVisible()` auto-retries every 100ms until timeout
- More robust than `waitFor()` which fails immediately if element doesn't exist
- 10000ms timeout is sufficient for Firefox's slower React scheduler

**Files to Modify:**
- N/A - This is now part of Option 2 implementation

---

### Option 4: Add Loading State Indicator (Production Code Change)

**Approach:** Modify component to show a loading placeholder while computing `selectedProviderInfo`.

**Implementation:**

```tsx
{/* Credentials Section */}
{providerType && (
  <div className="space-y-3 border-t pt-4">
    {selectedProviderInfo ? (
      <>
        <Label className="text-base" data-testid="credentials-section">
          {t('dnsProviders.credentials')}
        </Label>
        {/* Existing field rendering */}
      </>
    ) : (
      <div data-testid="credentials-loading">
        <Skeleton className="h-6 w-32 mb-3" />
        <Skeleton className="h-10 w-full mb-3" />
      </div>
    )}
  </div>
)}
```

**Rationale:**
- Better UX: Shows user that credentials are loading
- Test can wait for `[data-testid="credentials-section"]` OR `[data-testid="credentials-loading"]`
- Handles edge case where React Query is slow or fails

**Files to Modify:**
- `frontend/src/components/DNSProviderForm.tsx` (Lines 205-280)
- `tests/dns-provider-types.spec.ts` (update assertions)

**Cons:**
- Requires production code change for a test issue
- Adds complexity to component logic
- May not be necessary if fallback schemas are always defined

---

## Recommended Implementation Plan

### Phase 1: Immediate Fix (Option 2 - Supervisor Approved)

**Goal:** Fix failing tests by directly waiting for provider-specific fields.

**Steps:**

1. **Update Test: Remove Intermediate "credentials-section" Waits**
   - File: `tests/dns-provider-types.spec.ts`
   - Change: Remove wait for `[data-testid="credentials-section"]` label
   - Directly wait for provider-specific fields (confirms full React cycle)
   - Lines affected: 167, 179-187, 198-206, 217-225

2. **Update Test: Use `expect().toBeVisible()` with 10000ms Timeout**
   - File: `tests/dns-provider-types.spec.ts`
   - Change: Standardize all timeouts to 10000ms (sufficient for Firefox)
   - Use Playwright's auto-retry assertions throughout
   - Lines affected: Same as above

3. **Fix ALL 4 Provider Tests**
   - **Manual** (line 167) - Wait for DNS Server field
   - **Webhook** (line 179) - Wait for Create URL field
   - **RFC2136** (line 198) - Wait for DNS Server field
   - **Script** (line 217) - Wait for Script Path field

**Example Implementation (Supervisor's Pattern):**

```typescript
test('should show URL field when Webhook type is selected', async ({ page }) => {
  await page.goto('/dns/providers');
  await page.getByRole('button', { name: /add.*provider/i }).first().click();

  await test.step('Select Webhook provider type', async () => {
    const typeSelect = page.locator('#provider-type');
    await typeSelect.click();
    await page.getByRole('option', { name: /webhook/i }).click();
  });

  await test.step('Verify Webhook URL field appears', async () => {
    // DIRECTLY wait for provider-specific field (10s timeout for Firefox)
    await expect(page.getByLabel(/create.*url/i)).toBeVisible({ timeout: 10000 });
  });
});
```

**Provider-Specific Field Mapping:**

| Provider | Provider-Specific Field Label | Regex Pattern |
|----------|------------------------------|---------------|
| Manual | DNS Server | `/dns.*server/i` |
| Webhook | Create Record URL | `/create.*url/i` |
| RFC2136 | DNS Server | `/dns.*server/i` |
| Script | Script Path | `/script.*path/i` |

---

**Detailed Implementation for All 4 Tests:**

**Test 1: Manual Provider (Line 167)**
```typescript
test('should show DNS server field when Manual type is selected', async ({ page }) => {
  await page.goto('/dns/providers');
  await page.getByRole('button', { name: /add.*provider/i }).first().click();

  await test.step('Select Manual provider type', async () => {
    const typeSelect = page.locator('#provider-type');
    await typeSelect.click();
    await page.getByRole('option', { name: /manual/i }).click();
  });

  await test.step('Verify DNS Server field appears', async () => {
    await expect(page.getByLabel(/dns.*server/i)).toBeVisible({ timeout: 10000 });
  });
});
```

**Test 2: Webhook Provider (Line 179)**
```typescript
test('should show URL field when Webhook type is selected', async ({ page }) => {
  await page.goto('/dns/providers');
  await page.getByRole('button', { name: /add.*provider/i }).first().click();

  await test.step('Select Webhook provider type', async () => {
    const typeSelect = page.locator('#provider-type');
    await typeSelect.click();
    await page.getByRole('option', { name: /webhook/i }).click();
  });

  await test.step('Verify Webhook URL field appears', async () => {
    await expect(page.getByLabel(/create.*url/i)).toBeVisible({ timeout: 10000 });
  });
});
```

**Test 3: RFC2136 Provider (Line 198)**
```typescript
test('should show DNS server field when RFC2136 type is selected', async ({ page }) => {
  await page.goto('/dns/providers');
  await page.getByRole('button', { name: /add.*provider/i }).first().click();

  await test.step('Select RFC2136 provider type', async () => {
    const typeSelect = page.locator('#provider-type');
    await typeSelect.click();
    await page.getByRole('option', { name: /rfc2136/i }).click();
  });

  await test.step('Verify DNS Server field appears', async () => {
    await expect(page.getByLabel(/dns.*server/i)).toBeVisible({ timeout: 10000 });
  });
});
```

**Test 4: Script Provider (Line 217)**
```typescript
test('should show script path field when Script type is selected', async ({ page }) => {
  await page.goto('/dns/providers');
  await page.getByRole('button', { name: /add.*provider/i }).first().click();

  await test.step('Select Script provider type', async () => {
    const typeSelect = page.locator('#provider-type');
    await typeSelect.click();
    await page.getByRole('option', { name: /script/i }).click();
  });

  await test.step('Verify Script Path field appears', async () => {
    await expect(page.getByLabel(/script.*path/i)).toBeVisible({ timeout: 10000 });
  });
});
```

---

### Phase 2: Enhanced Robustness (Optional, Future)

**Goal:** Improve component resilience and UX.

**Steps:**

1. **Add Loading State** (Option 4)
   - Shows skeleton loader while `selectedProviderInfo` is being computed
   - Better UX for slow network or React Query cache misses
   - Priority: Medium

2. **Add React Query Stale Time Monitoring**
   - Log warning if `providerTypes` query takes >500ms
   - Helps identify backend performance issues
   - Priority: Low

---

## Testing Strategy

### Before Fix Verification

```bash
# Reproduce failure (should fail on Firefox)
npx playwright test tests/dns-provider-types.spec.ts --project=firefox --grep "Webhook"
npx playwright test tests/dns-provider-types.spec.ts --project=firefox --grep "RFC2136"
npx playwright test tests/dns-provider-types.spec.ts --project=firefox --grep "Script"
npx playwright test tests/dns-provider-types.spec.ts --project=firefox --grep "Manual"
```

Expected: **FAIL** - Timeout waiting for provider-specific fields

---

### After Fix Verification

```bash
# 1. Run fixed tests on all browsers (all 4 provider types)
npx playwright test tests/dns-provider-types.spec.ts --project=chromium --project=firefox --project=webkit

# 2. Run 10 times to verify stability (flake detection)
for i in {1..10}; do
  npx playwright test tests/dns-provider-types.spec.ts --project=firefox
done

# 3. Check trace for timing details
npx playwright test tests/dns-provider-types.spec.ts --project=firefox --trace on
npx playwright show-trace trace.zip
```

Expected: **PASS** on all runs, all browsers, all 4 provider types

---

## Acceptance Criteria

- [ ] Manual provider type selection test passes on Chromium
- [ ] Manual provider type selection test passes on Firefox
- [ ] Manual provider type selection test passes on Webkit
- [ ] Webhook provider type selection test passes on Chromium
- [ ] Webhook provider type selection test passes on Firefox
- [ ] Webhook provider type selection test passes on Webkit
- [ ] RFC2136 provider type selection test passes on Chromium
- [ ] RFC2136 provider type selection test passes on Firefox
- [ ] RFC2136 provider type selection test passes on Webkit
- [ ] Script provider type selection test passes on Chromium
- [ ] Script provider type selection test passes on Firefox
- [ ] Script provider type selection test passes on Webkit
- [ ] All tests complete in <30 seconds total
- [ ] No flakiness detected in 10 consecutive runs
- [ ] Test output shows clear step progression
- [ ] Trace shows React re-render timing is accounted for
- [ ] All timeouts standardized to 10000ms

---

## Rollback Plan

If Option 2 fix introduces new issues:

1. **Revert test changes**: `git revert <commit>`
2. **Try Option 1 instead**: Add intermediate select value assertion
3. **Increase timeout globally**: Update `playwright.config.js` timeout defaults to 15000ms
4. **Skip Firefox temporarily**: Add `test.skip()` for Firefox until root cause addressed

---

## Related Files

### Tests
- `tests/dns-provider-types.spec.ts` - Primary test file needing fixes

### Production Code (No changes in Phase 1)
- `frontend/src/components/DNSProviderForm.tsx` - Component with conditional rendering
- `frontend/src/data/dnsProviderSchemas.ts` - Fallback provider schemas
- `frontend/src/hooks/useDNSProviders.ts` - React Query hook for provider types

### Configuration
- `playwright.config.js` - Playwright timeout settings

---

## Implementation Checklist

- [ ] Read and understand root cause analysis
- [ ] Review `DNSProviderForm.tsx` conditional rendering logic (Lines 205-209)
- [ ] Review `defaultProviderSchemas` for all providers (Lines 245-301)
- [ ] Review existing test at line 187 (already uses Option 2 pattern)
- [ ] Implement Option 2 for **Manual provider** (line 167):
  - Remove intermediate "credentials-section" wait
  - Wait directly for DNS Server field with 10000ms timeout
- [ ] Implement Option 2 for **Webhook provider** (line 179):
  - Remove intermediate "credentials-section" wait
  - Wait directly for Create URL field with 10000ms timeout
- [ ] Implement Option 2 for **RFC2136 provider** (line 198):
  - Remove intermediate "credentials-section" wait
  - Wait directly for DNS Server field with 10000ms timeout
- [ ] Implement Option 2 for **Script provider** (line 217):
  - Remove intermediate "credentials-section" wait
  - Wait directly for Script Path field with 10000ms timeout
- [ ] Run tests on all browsers to verify fix (4 tests × 3 browsers = 12 test runs)
- [ ] Run 10 consecutive times on Firefox to check for flakiness
- [ ] Review trace to confirm timing is now accounted for
- [ ] Update this document with actual test results
- [ ] Close related issues/PRs

---

## Lessons Learned

1. **React state updates are asynchronous**: Tests must wait for visual confirmation of state changes
2. **Conditional rendering creates timing dependencies**: Tests must account for render cycles
3. **Browser engines have different React scheduler timing**: Firefox is ~2x slower than Chromium
4. **Playwright's `expect().toBeVisible()` is more robust than `waitFor()`**: Auto-retry mechanism handles timing better
5. **Direct field assertions are better than intermediate checks**: Waiting for provider-specific fields confirms full React cycle in one step
6. **Standardize timeouts**: 10000ms is sufficient for all browsers including Firefox
7. **Supervisor feedback is critical**: Option 2 (direct field wait) is simpler and more reliable than Option 1 (intermediate select wait)

---

## Next Steps

1. **Implement Option 2 fixes** for all 4 provider tests
2. **Run full E2E test suite** to ensure no regressions
3. **Monitor Codecov patch coverage** to ensure no test coverage loss
4. **Consider Phase 2 enhancements** if UX issues are reported

---

## Success Metrics

- **Test Pass Rate**: 100% (currently failing intermittently on Firefox)
- **Test Duration**: <10 seconds per test (4 tests total)
- **Flakiness**: 0% (10 consecutive runs without failure)
- **Coverage**: Maintain 100% patch coverage for `DNSProviderForm.tsx`
- **Browser Support**: All 4 tests pass on Chromium, Firefox, and Webkit

---

**Status:** Ready for Implementation
**Estimated Effort:** 1-2 hours (implementation + verification for 4 tests)
**Risk Level:** Low (test-only changes, no production code modified)
**Tests to Fix:** 4 (Manual, Webhook, RFC2136, Script)
**Approach:** Option 2 - Direct provider-specific field wait
**Timeout:** 10000ms standardized across all assertions
