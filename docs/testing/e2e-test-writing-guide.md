# E2E Test Writing Guide

**Last Updated**: February 2, 2026

This guide provides best practices for writing maintainable, performant, and cross-browser compatible Playwright E2E tests for Charon.

---

## Table of Contents

- [Cross-Browser Compatibility](#cross-browser-compatibility)
- [Performance Best Practices](#performance-best-practices)
- [Feature Flag Testing](#feature-flag-testing)
- [Test Isolation](#test-isolation)
- [Common Patterns](#common-patterns)
- [Troubleshooting](#troubleshooting)

---

## Cross-Browser Compatibility

### Why It Matters

Charon E2E tests run across **Chromium**, **Firefox**, and **WebKit** (Safari engine). Browser differences in how they handle label association, form controls, and DOM queries can cause tests to pass in one browser but fail in others.

**Phase 2 Fix**: The `getFormFieldByLabel()` helper was added to address cross-browser label matching inconsistencies.

### Problem: Browser-Specific Label Handling

Different browsers handle `getByLabel()` differently:

- **Chromium**: Lenient label matching, searches visible text aggressively
- **Firefox**: Stricter matching, requires explicit `for` attribute or nesting
- **WebKit**: Strictest, often fails on complex label structures

**Example Failure**:

```typescript
// ❌ FRAGILE: Fails in Firefox/WebKit when label structure is complex
const scriptPath = page.getByLabel(/script.*path/i);
await scriptPath.fill('/path/to/script.sh');
```

**Error (Firefox/WebKit)**:

```
TimeoutError: locator.fill: Timeout 5000ms exceeded.
=========================== logs ===========================
waiting for getByLabel(/script.*path/i)
============================================================
```

### Solution: Multi-Tier Fallback Strategy

Use the `getFormFieldByLabel()` helper for robust cross-browser field location:

```typescript
import { getFormFieldByLabel } from '../utils/ui-helpers';

// ✅ ROBUST: 4-tier fallback strategy
const scriptPath = getFormFieldByLabel(
  page,
  /script.*path/i,
  {
    placeholder: /dns-challenge\.sh/i,
    fieldId: 'field-script_path'
  }
);
await scriptPath.fill('/path/to/script.sh');
```

**Fallback Chain**:

1. **Primary**: `getByLabel(labelPattern)` — Standard label association
2. **Fallback 1**: `getByPlaceholder(options.placeholder)` — Placeholder text match
3. **Fallback 2**: `locator('#' + options.fieldId)` — Direct ID selector
4. **Fallback 3**: Role-based with label proximity — `getByRole('textbox')` near label text

### When to Use `getFormFieldByLabel()`

✅ **Use when**:

- Form fields have complex label structures (nested elements, icons, tooltips)
- Tests fail in Firefox/WebKit but pass in Chromium
- Label text is dynamic or internationalized
- Multiple fields have similar labels

❌ **Don't use when**:

- Standard `getByLabel()` works reliably across all browsers
- Field has a unique `data-testid` or `name` attribute
- Field is the only one of its type on the page

---

## Performance Best Practices

### Avoid Unnecessary API Polling

**Problem**: Excessive API polling adds latency and increases flakiness.

**Before Phase 2 (❌ Inefficient)**:

```typescript
test.beforeEach(async ({ page }) => {
  await page.goto('/settings/system');

  // ❌ BAD: Polls API even when flags are already correct
  await waitForFeatureFlagPropagation(page, {
    'cerberus.enabled': false,
    'crowdsec.enabled': false
  });
});

test('Enable Cerberus', async ({ page }) => {
  const toggle = page.getByRole('switch', { name: /cerberus/i });
  await clickSwitch(toggle);

  // ❌ BAD: Another full polling cycle
  await waitForFeatureFlagPropagation(page, {
    'cerberus.enabled': true
  });
});
```

**After Phase 2 (✅ Optimized)**:

```typescript
test.afterEach(async ({ page, request }) => {
  // ✅ GOOD: Cleanup once at the end
  await request.post('/api/v1/settings/restore', {
    data: { module: 'system', defaults: true }
  });
});

test('Enable Cerberus', async ({ page }) => {
  const toggle = page.getByRole('switch', { name: /cerberus/i });

  await test.step('Toggle Cerberus on', async () => {
    await clickSwitch(toggle);

    // ✅ GOOD: Only poll when state changes
    await waitForFeatureFlagPropagation(page, {
      'cerberus.enabled': true
    });
  });

  await test.step('Verify toggle reflects new state', async () => {
    await expectSwitchState(toggle, true);
  });
});
```

### How Conditional Polling Works

The `waitForFeatureFlagPropagation()` helper includes an **early-exit optimization** (Phase 2 Fix 2.3):

```typescript
// Before polling, check if flags are already in expected state
const currentState = await page.evaluate(async () => {
  const res = await fetch('/api/v1/feature-flags');
  return res.json();
});

if (alreadyMatches(currentState, expectedFlags)) {
  console.log('[POLL] Already in expected state - skipping poll');
  return currentState; // Exit immediately
}

// Otherwise, start polling...
```

**Performance Impact**: ~50% reduction in polling iterations for tests that restore defaults in `afterEach`.

### Request Coalescing (Worker Isolation)

**Problem**: Parallel Playwright workers polling the same flag state cause redundant API calls.

**Solution**: The helper caches in-flight requests per worker:

```typescript
// Worker 1: Waits for {cerberus: false, crowdsec: false}
// Worker 2: Waits for {cerberus: false, crowdsec: false}

// Without coalescing: 2 separate polling loops (30+ API calls)
// With coalescing: 1 shared promise (15 API calls, cached per worker)
```

**Cache Key Format**:

```
[worker_index]:[sorted_flags_json]
```

**Example**:

```
Worker 0: "0:{\"feature.cerberus.enabled\":false,\"feature.crowdsec.enabled\":false}"
Worker 1: "1:{\"feature.cerberus.enabled\":false,\"feature.crowdsec.enabled\":false}"
```

---

## Feature Flag Testing

### When to Use `waitForFeatureFlagPropagation()`

✅ **Use when**:

- A test **toggles** a feature flag via the UI
- Backend state changes and you need to verify propagation
- Test depends on a specific flag state being active

❌ **Don't use when**:

- Setting up initial state in `beforeEach` (use API directly instead)
- Flags haven't changed since last verification
- Test doesn't modify flags

### Pattern: Cleanup in `afterEach`

**Best Practice**: Restore defaults at the end, not the beginning.

```typescript
test.describe('System Settings', () => {
  test.afterEach(async ({ request }) => {
    // Restore all defaults once
    await request.post('/api/v1/settings/restore', {
      data: { module: 'system', defaults: true }
    });
  });

  test('Enable and disable Cerberus', async ({ page }) => {
    await page.goto('/settings/system');

    const toggle = page.getByRole('switch', { name: /cerberus/i });

    // Test starts from whatever state exists (defaults expected)
    await clickSwitch(toggle);
    await waitForFeatureFlagPropagation(page, { 'cerberus.enabled': true });

    await clickSwitch(toggle);
    await waitForFeatureFlagPropagation(page, { 'cerberus.enabled': false });
  });
});
```

**Why This Works**:

- Each test starts from known defaults (restored by previous test's `afterEach`)
- No unnecessary polling in `beforeEach`
- Cleanup happens once, not N times per describe block

### Handling Config Reload Overlay

When toggling security features (Cerberus, ACL, WAF), Caddy reloads its configuration. A blocking overlay prevents interactions during this reload.

**Helper Handles This Automatically**:

```typescript
export async function waitForFeatureFlagPropagation(...) {
  // ✅ Wait for overlay to disappear before polling
  const overlay = page.locator('[data-testid="config-reload-overlay"]');
  await overlay.waitFor({ state: 'hidden', timeout: 10000 })
    .catch(() => {});

  // Now safe to poll API...
}
```

**You don't need to manually wait for the overlay** — it's handled by:

- `clickSwitch()`
- `clickAndWaitForResponse()`
- `waitForFeatureFlagPropagation()`

---

## Test Isolation

### Why Isolation Matters

Tests running in parallel can interfere with each other if they:

- Share mutable state (database, config files, feature flags)
- Don't clean up resources
- Rely on global defaults

**Phase 2 Fix**: Added explicit `afterEach` cleanup to restore defaults.

### Pattern: Isolated Flag Toggles

**Before (❌ Not Isolated)**:

```typescript
test('Test A', async ({ page }) => {
  // Enable Cerberus
  // ...
  // ❌ Leaves flag enabled for next test
});

test('Test B', async ({ page }) => {
  // Assumes Cerberus is disabled
  // ❌ May fail if Test A ran first
});
```

**After (✅ Isolated)**:

```typescript
test.afterEach(async ({ request }) => {
  await request.post('/api/v1/settings/restore', {
    data: { module: 'system', defaults: true }
  });
});

test('Test A', async ({ page }) => {
  // Enable Cerberus
  // ...
  // ✅ Cleanup restores defaults after test
});

test('Test B', async ({ page }) => {
  // ✅ Starts from known defaults
});
```

### Cleanup Order of Operations

```
1. Test A runs → modifies state
2. Test A finishes → afterEach runs → restores defaults
3. Test B runs → starts from defaults
4. Test B finishes → afterEach runs → restores defaults
```

---

## Common Patterns

### Toggle Feature Flag

```typescript
test('Enable and verify feature', async ({ page }) => {
  await page.goto('/settings/system');

  const toggle = page.getByRole('switch', { name: /feature name/i });

  await test.step('Enable feature', async () => {
    await clickSwitch(toggle);
    await waitForFeatureFlagPropagation(page, { 'feature.enabled': true });
  });

  await test.step('Verify UI reflects state', async () => {
    await expectSwitchState(toggle, true);
    await expect(page.getByText(/feature active/i)).toBeVisible();
  });
});
```

### Form Field with Cross-Browser Locator

```typescript
test('Fill DNS provider config', async ({ page }) => {
  await page.goto('/dns-providers/new');

  await test.step('Select provider type', async () => {
    await page.getByRole('combobox', { name: /type/i }).click();
    await page.getByRole('option', { name: /manual/i }).click();
  });

  await test.step('Fill script path', async () => {
    const scriptPath = getFormFieldByLabel(
      page,
      /script.*path/i,
      {
        placeholder: /dns-challenge\.sh/i,
        fieldId: 'field-script_path'
      }
    );
    await scriptPath.fill('/usr/local/bin/dns-challenge.sh');
  });
});
```

### Wait for API Response After Action

```typescript
test('Create resource and verify', async ({ page }) => {
  await page.goto('/resources');

  const createBtn = page.getByRole('button', { name: /create/i });

  const response = await clickAndWaitForResponse(
    page,
    createBtn,
    /\/api\/v1\/resources/,
    { status: 201 }
  );

  expect(response.ok()).toBeTruthy();

  const json = await response.json();
  await expect(page.getByText(json.name)).toBeVisible();
});
```

---

## Troubleshooting

### Test Fails in Firefox/WebKit, Passes in Chromium

**Symptom**: `TimeoutError: locator.fill: Timeout 5000ms exceeded`

**Cause**: Label matching strategy differs between browsers.

**Fix**: Use `getFormFieldByLabel()` with fallbacks:

```typescript
// ❌ BEFORE
await page.getByLabel(/field name/i).fill('value');

// ✅ AFTER
const field = getFormFieldByLabel(page, /field name/i, {
  placeholder: /enter value/i
});
await field.fill('value');
```

### Feature Flag Polling Times Out

**Symptom**: `Feature flag propagation timeout after 120 attempts (60000ms)`

**Causes**:

1. Backend not updating flags
2. Config reload overlay blocking UI
3. Database transaction not committed

**Fix Steps**:

1. Check backend logs: Does PUT `/api/v1/feature-flags` succeed?
2. Check overlay state: Is `[data-testid="config-reload-overlay"]` stuck visible?
3. Increase timeout temporarily: `waitForFeatureFlagPropagation(page, flags, { timeout: 120000 })`
4. Add retry wrapper: Use `retryAction()` for transient failures

```typescript
await retryAction(async () => {
  await clickSwitch(toggle);
  await waitForFeatureFlagPropagation(page, { 'flag': true });
}, { maxAttempts: 3, baseDelay: 2000 });
```

### Switch Click Intercepted

**Symptom**: `Error: Element is not visible` or `click intercepted by overlay`

**Cause**: Config reload overlay or sticky header blocking interaction.

**Fix**: Use `clickSwitch()` helper (handles overlay automatically):

```typescript
// ❌ BEFORE
await page.getByRole('switch').click({ force: true }); // Bad!

// ✅ AFTER
await clickSwitch(page.getByRole('switch', { name: /feature/i }));
```

### Test Pollution (Fails When Run in Suite, Passes Alone)

**Symptom**: Test passes when run solo (`--grep`), fails in full suite.

**Cause**: Previous test left state modified (flags enabled, resources created).

**Fix**: Add cleanup in `afterEach`:

```typescript
test.afterEach(async ({ request }) => {
  // Restore defaults
  await request.post('/api/v1/settings/restore', {
    data: { module: 'system', defaults: true }
  });
});
```

---

## Reference

### Helper Functions

| Helper | Purpose | File |
|--------|---------|------|
| `getFormFieldByLabel()` | Cross-browser form field locator | `tests/utils/ui-helpers.ts` |
| `clickSwitch()` | Reliable switch/toggle interaction | `tests/utils/ui-helpers.ts` |
| `expectSwitchState()` | Assert switch checked state | `tests/utils/ui-helpers.ts` |
| `waitForFeatureFlagPropagation()` | Poll for flag state | `tests/utils/wait-helpers.ts` |
| `clickAndWaitForResponse()` | Atomic click + wait | `tests/utils/wait-helpers.ts` |
| `retryAction()` | Retry with exponential backoff | `tests/utils/wait-helpers.ts` |

### Best Practices Summary

1. ✅ **Cross-Browser**: Use `getFormFieldByLabel()` for complex label structures
2. ✅ **Performance**: Only poll when flags change, not in `beforeEach`
3. ✅ **Isolation**: Restore defaults in `afterEach`, not `beforeEach`
4. ✅ **Reliability**: Use semantic locators (`getByRole`, `getByLabel`) over CSS selectors
5. ✅ **Debugging**: Use `test.step()` for clear failure context

---

**See Also**:

- [Testing README](./README.md) — Quick reference and debugging guide
- [Switch Component Testing](./README.md#-switchtoggle-component-testing) — Detailed switch patterns
- [Debugging Guide](./debugging-guide.md) — Troubleshooting slow/flaky tests
