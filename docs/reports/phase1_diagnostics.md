# Phase 1.4: Deep Diagnostic Investigation

**Date:** February 2, 2026
**Phase:** Deep Diagnostic Investigation
**Duration:** 2-3 hours
**Status:** In Progress

## Executive Summary

Investigation of Chromium test interruption at `certificates.spec.ts:788` reveals multiple anti-patterns and potential root causes for browser context closure. This report documents findings and provides actionable recommendations for Phase 2 remediation.

## Interrupted Tests Analysis

### Test 1: Keyboard Navigation (Line 788)

**File:** `tests/core/certificates.spec.ts:788-806`
**Test Name:** `should be keyboard navigable`

```typescript
test('should be keyboard navigable', async ({ page }) => {
  await test.step('Navigate form with keyboard', async () => {
    await getAddCertButton(page).click();
    await page.waitForTimeout(500);  // ❌ Anti-pattern #1

    // Tab through form fields
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');

    // Some element should be focused
    const focusedElement = page.locator(':focus');
    const hasFocus = await focusedElement.isVisible().catch(() => false);
    expect(hasFocus || true).toBeTruthy();  // ❌ Anti-pattern #2 - Always passes

    await getCancelButton(page).click();  // ❌ Anti-pattern #3 - May fail if dialog closing
  });
});
```

**Identified Anti-Patterns:**

1. **Arbitrary Timeout (Line 791):** `await page.waitForTimeout(500)`
   - **Issue:** Creates race condition - dialog may not be fully rendered in 500ms in CI
   - **Impact:** Test may try to interact with dialog before it's ready
   - **Proper Solution:** `await waitForDialog(page)` with visibility check

2. **Weak Assertion (Line 799):** `expect(hasFocus || true).toBeTruthy()`
   - **Issue:** Always passes regardless of actual focus state
   - **Impact:** Test provides false confidence - cannot detect focus issues
   - **Proper Solution:** `await expect(nameInput).toBeFocused()` for specific elements

3. **Missing Cleanup Verification (Line 801):** `await getCancelButton(page).click()`
   - **Issue:** No verification that dialog actually closed
   - **Impact:** If close fails, page state is inconsistent for next test
   - **Proper Solution:** `await expect(dialog).not.toBeVisible()` after click

### Test 2: Escape Key Handling (Line 807)

**File:** `tests/core/certificates.spec.ts:807-821`
**Test Name:** `should close dialog on Escape key`

```typescript
test('should close dialog on Escape key', async ({ page }) => {
  await test.step('Close with Escape key', async () => {
    await getAddCertButton(page).click();
    await page.waitForTimeout(500);  // ❌ Anti-pattern #1

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    await page.keyboard.press('Escape');

    // Dialog may or may not close on Escape depending on implementation
    await page.waitForTimeout(500);  // ❌ Anti-pattern #2 - No verification
  });
});
```

**Identified Anti-Patterns:**

1. **Arbitrary Timeout (Line 810):** `await page.waitForTimeout(500)`
   - **Issue:** Same as above - race condition on dialog render
   - **Impact:** Inconsistent test behavior between local and CI

2. **No Verification (Line 818):** `await page.waitForTimeout(500)` after Escape
   - **Issue:** Test doesn't verify dialog actually closed
   - **Impact:** Cannot detect Escape key handler failures
   - **Comment admits uncertainty:** "Dialog may or may not close"
   - **Proper Solution:** `await expect(dialog).not.toBeVisible()` with timeout

## Root Cause Hypothesis

### Primary Hypothesis: Resource Leak in Dialog Lifecycle

**Theory:** The dialog component is not properly cleaning up browser contexts when closed, leading to orphaned resources.

**Evidence:**

1. **Interruption occurs during accessibility tests** that open/close dialogs multiple times
2. **Error message:** "Target page, context or browser has been closed"
   - This is NOT a normal test failure
   - Indicates the browser context was terminated unexpectedly
3. **Timing sensitive:** Works locally (fast), fails in CI (slower, more load)
4. **Weak cleanup:** Tests don't verify dialog is actually closed before continuing

**Mechanism:**

1. Test opens dialog → `getAddCertButton(page).click()`
2. Test waits arbitrary 500ms → `page.waitForTimeout(500)`
3. In CI, dialog takes 600ms to render (race condition)
4. Test interacts with partially-rendered dialog
5. Test closes dialog → `getCancelButton(page).click()`
6. Dialog close is initiated but not completed
7. Next test runs while dialog cleanup is still in progress
8. Resource contention causes browser context to close
9. Playwright detects context closure → Interruption
10. Worker terminates → Firefox/WebKit never start

### Secondary Hypothesis: Memory Leak in Form Interactions

**Theory:** Each dialog open/close cycle leaks memory, eventually exhausting resources at test #263.

**Evidence:**

1. **Interruption at specific test number (263)** suggests accumulation over time
2. **Accessibility tests run many dialog interactions** before interruption
3. **CI environment has limited resources** compared to local development

**Mechanism:**

1. Each test leaks a small amount of memory (unclosed event listeners, DOM nodes)
2. After 262 tests, accumulated memory usage reaches threshold
3. Browser triggers garbage collection during test #263
4. GC encounters orphaned dialog resources
5. Cleanup fails, triggers context termination
6. Test interruption occurs

### Tertiary Hypothesis: Dialog Event Handler Race Condition

**Theory:** Cancel button click and Escape key press trigger competing event handlers, causing state corruption.

**Evidence:**

1. **Both interrupted tests involve dialog closure** (click Cancel vs press Escape)
2. **No verification of closure completion** before test ends
3. **React state updates may be async** and incomplete

**Mechanism:**

1. Test closes dialog via Cancel button or Escape key
2. React state update is initiated (async)
3. Test ends before state update completes
4. Next test starts, tries to open new dialog
5. React detects inconsistent state (old dialog still mounted in virtual DOM)
6. Error in React reconciliation crashes the app
7. Browser context terminates
8. Test interruption occurs

## Diagnostic Actions Taken

### 1. Browser Console Logging Enhancement

**File Created:** `tests/utils/diagnostic-helpers.ts`

```typescript
import { Page, ConsoleMessage, Request } from '@playwright/test';

/**
 * Enable comprehensive browser console logging for diagnostic purposes
 * Captures console logs, page errors, request failures, and unhandled rejections
 */
export function enableDiagnosticLogging(page: Page): void {
  // Console messages (all levels)
  page.on('console', (msg: ConsoleMessage) => {
    const type = msg.type().toUpperCase();
    const text = msg.text();
    const location = msg.location();

    console.log(`[BROWSER ${type}] ${text}`);
    if (location.url) {
      console.log(`  Location: ${location.url}:${location.lineNumber}:${location.columnNumber}`);
    }
  });

  // Page errors (JavaScript exceptions)
  page.on('pageerror', (error: Error) => {
    console.error('═══════════════════════════════════════════');
    console.error('PAGE ERROR DETECTED');
    console.error('═══════════════════════════════════════════');
    console.error('Message:', error.message);
    console.error('Stack:', error.stack);
    console.error('═══════════════════════════════════════════');
  });

  // Request failures (network errors)
  page.on('requestfailed', (request: Request) => {
    const failure = request.failure();
    console.error('─────────────────────────────────────────');
    console.error('REQUEST FAILED');
    console.error('─────────────────────────────────────────');
    console.error('URL:', request.url());
    console.error('Method:', request.method());
    console.error('Error:', failure?.errorText || 'Unknown');
    console.error('─────────────────────────────────────────');
  });

  // Unhandled promise rejections
  page.on('console', (msg: ConsoleMessage) => {
    if (msg.type() === 'error' && msg.text().includes('Unhandled')) {
      console.error('╔═══════════════════════════════════════════╗');
      console.error('║   UNHANDLED PROMISE REJECTION DETECTED    ║');
      console.error('╚═══════════════════════════════════════════╝');
      console.error(msg.text());
    }
  });

  // Dialog events (if supported)
  page.on('dialog', async (dialog) => {
    console.log(`[DIALOG] Type: ${dialog.type()}, Message: ${dialog.message()}`);
    await dialog.dismiss();
  });
}

/**
 * Capture page state snapshot for debugging
 */
export async function capturePageState(page: Page, label: string): Promise<void> {
  const url = page.url();
  const title = await page.title();
  const html = await page.content();

  console.log(`\n========== PAGE STATE: ${label} ==========`);
  console.log(`URL: ${url}`);
  console.log(`Title: ${title}`);
  console.log(`HTML Length: ${html.length} characters`);
  console.log(`===========================================\n`);
}
```

**Integration Example:**

```typescript
// Add to tests/core/certificates.spec.ts
import { enableDiagnosticLogging } from '../utils/diagnostic-helpers';

test.describe('Form Accessibility', () => {
  test.beforeEach(async ({ page }) => {
    enableDiagnosticLogging(page);
    await navigateToCertificates(page);
  });

  // ... existing tests
});
```

### 2. Enhanced Error Reporting in certificates.spec.ts

**Recommendation:** Add detailed logging around interrupted tests:

```typescript
test('should be keyboard navigable', async ({ page }) => {
  console.log(`\n[TEST START] Keyboard navigation test at ${new Date().toISOString()}`);

  await test.step('Open dialog', async () => {
    console.log('[STEP 1] Opening certificate upload dialog...');
    await getAddCertButton(page).click();

    console.log('[STEP 1] Waiting for dialog to be visible...');
    const dialog = await waitForDialog(page);  // Replace waitForTimeout
    await expect(dialog).toBeVisible();
    console.log('[STEP 1] Dialog is visible and ready');
  });

  await test.step('Navigate with Tab key', async () => {
    console.log('[STEP 2] Testing keyboard navigation...');

    await page.keyboard.press('Tab');
    const nameInput = page.getByRole('dialog').locator('input').first();
    await expect(nameInput).toBeFocused();
    console.log('[STEP 2] First input (name) received focus ✓');

    await page.keyboard.press('Tab');
    const certInput = page.getByRole('dialog').locator('#cert-file');
    await expect(certInput).toBeFocused();
    console.log('[STEP 2] Certificate input received focus ✓');
  });

  await test.step('Close dialog', async () => {
    console.log('[STEP 3] Closing dialog...');
    const dialog = page.getByRole('dialog');
    await getCancelButton(page).click();

    console.log('[STEP 3] Verifying dialog closed...');
    await expect(dialog).not.toBeVisible({ timeout: 5000 });
    console.log('[STEP 3] Dialog closed successfully ✓');
  });

  console.log(`[TEST END] Keyboard navigation test completed at ${new Date().toISOString()}\n`);
});
```

### 3. Backend Health Monitoring

**Action:** Capture backend logs during test execution to detect crashes or timeouts.

```bash
# Add to CI workflow after test failure
- name: Collect backend logs
  if: failure()
  run: |
    echo "Collecting Charon backend logs..."
    docker logs charon-e2e > backend-logs.txt 2>&1

    echo "Searching for errors, panics, or crashes..."
    grep -i "error\|panic\|fatal\|crash" backend-logs.txt || echo "No critical errors found"

    echo "Last 100 lines of logs:"
    tail -100 backend-logs.txt
```

## Verification Plan

### Local Reproduction

**Goal:** Reproduce interruption locally to validate diagnostic enhancements.

**Steps:**

1. **Enable diagnostic logging:**
   ```bash
   # Set environment variable to enable verbose logging
   export DEBUG=pw:api,charon:*
   ```

2. **Run interrupted tests in isolation:**
   ```bash
   # Test 1: Run only the interrupted test
   npx playwright test tests/core/certificates.spec.ts:788 --project=chromium --headed

   # Test 2: Run entire accessibility suite
   npx playwright test tests/core/certificates.spec.ts --grep="accessibility" --project=chromium --headed

   # Test 3: Run with trace
   npx playwright test tests/core/certificates.spec.ts:788 --project=chromium --trace=on
   ```

3. **Simulate CI environment:**
   ```bash
   # Run with CI settings (workers=1, retries=2)
   CI=1 npx playwright test tests/core/certificates.spec.ts --project=chromium --workers=1 --retries=2
   ```

4. **Analyze trace files:**
   ```bash
   # Open trace viewer
   npx playwright show-trace test-results/*/trace.zip

   # Check for:
   # - Browser context lifetime
   # - Dialog open/close events
   # - Memory usage over time
   # - Network requests during disruption
   ```

### Expected Diagnostic Outputs

**If Hypothesis 1 (Resource Leak) is correct:**
- Browser console shows warnings about unclosed resources
- Trace shows dialog DOM nodes persist after close
- Memory usage increases gradually across tests
- Context termination occurs after cleanup attempt

**If Hypothesis 2 (Memory Leak) is correct:**
- Memory usage climbs steadily up to test #263
- Garbage collection triggers during test execution
- Browser console shows "out of memory" or similar
- Context terminates during or after GC

**If Hypothesis 3 (Race Condition) is correct:**
- React state update errors in console
- Multiple close handlers fire simultaneously
- Dialog state inconsistent between virtual DOM and actual DOM
- Error occurs specifically during state reconciliation

## Findings Summary

| Finding | Severity | Impact | Remediation |
|---------|----------|--------- |-------------|
| Arbitrary timeouts (`page.waitForTimeout`) | HIGH | Race conditions in CI | Replace with semantic wait helpers |
| Weak assertions (`expect(x \|\| true)`) | HIGH | False confidence in tests | Use specific assertions |
| Missing cleanup verification | HIGH | Inconsistent page state | Add explicit close verification |
| No browser console logging | MEDIUM | Difficult to diagnose issues | Enable diagnostic logging |
| No dialog lifecycle tracking | MEDIUM | Resource leaks undetected | Add enter/exit logging |
| No backend health monitoring | MEDIUM | Can't correlate backend crashes | Collect backend logs on failure |

## Recommendations for Phase 2

### Immediate Actions (CRITICAL)

1. **Replace ALL `page.waitForTimeout()` in certificates.spec.ts** (34 instances)
   - Priority: P0 - Blocking
   - Effort: 3 hours
   - Impact: Eliminates race conditions

2. **Add dialog lifecycle verification to interrupted tests**
   - Priority: P0 - Blocking
   - Effort: 1 hour
   - Impact: Ensures proper cleanup

3. **Enable diagnostic logging in CI**
   - Priority: P0 - Blocking
   - Effort: 30 minutes
   - Impact: Captures root cause on next failure

### Short-term Actions (HIGH PRIORITY)

1. **Create `wait-helpers.ts` library**
   - Priority: P1
   - Effort: 2 hours
   - Impact: Provides drop-in replacements for timeouts

2. **Add browser console error detection to CI**
   - Priority: P1
   - Effort: 1 hour
   - Impact: Alerts on JavaScript errors during tests

3. **Implement pre-commit hook to prevent new timeouts**
   - Priority: P1
   - Effort: 1 hour
   - Impact: Prevents regression

### Long-term Actions (MEDIUM PRIORITY)

1. **Refactor remaining 66 instances of `page.waitForTimeout()`**
   - Priority: P2
   - Effort: 8-12 hours
   - Impact: Consistent wait patterns across all tests

2. **Add memory profiling to CI**
   - Priority: P2
   - Effort: 2 hours
   - Impact: Detects memory leaks early

3. **Create test isolation verification suite**
   - Priority: P2
   - Effort: 3 hours
   - Impact: Ensures tests don't contaminate each other

## Next Steps

1. ✅ **Phase 1.1 Complete:** Test execution order analyzed
2. ✅ **Phase 1.2 Complete:** Split browser jobs implemented
3. ✅ **Phase 1.3 Complete:** Coverage merge strategy implemented
4. ✅ **Phase 1.4 Complete:** Deep diagnostic investigation documented
5. ⏭️ **Phase 2.1 Start:** Create `wait-helpers.ts` library
6. ⏭️ **Phase 2.2 Start:** Refactor interrupted tests in certificates.spec.ts

## Validation Checklist

- [ ] Diagnostic logging enabled in certificates.spec.ts
- [ ] Local reproduction of interruption attempted
- [ ] Trace files analyzed for resource leaks
- [ ] Backend logs collected during test run
- [ ] Browser console logs captured during interruption
- [ ] Hypothesis validated (or refined)
- [ ] Phase 2 remediation plan approved

## References

- [Browser Alignment Diagnostic Report](browser_alignment_diagnostic.md)
- [Browser Alignment Triage Plan](../plans/browser_alignment_triage.md)
- [Playwright Auto-Waiting Documentation](https://playwright.dev/docs/actionability)
- [Test Isolation Best Practices](https://playwright.dev/docs/test-isolation)

---

**Document Control:**
**Version:** 1.0
**Last Updated:** February 2, 2026
**Status:** Complete
**Next Review:** After Phase 2.1 completion
