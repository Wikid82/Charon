# Charon E2E Test Suite

**Playwright-based end-to-end tests for the Charon management interface.**

Quick Links:
- 📖 [Complete Testing Documentation](../docs/testing/)
- 📝 [E2E Test Writing Guide](../docs/testing/e2e-test-writing-guide.md)
- 🐛 [Debugging Guide](../docs/testing/debugging-guide.md)

---

## Running Tests

```bash
# All tests (Chromium only)
npm run e2e

# All browsers (Chromium, Firefox, WebKit)
npm run e2e:all

# Headed mode (visible browser)
npm run e2e:headed

# Single test file
npx playwright test tests/settings/system-settings.spec.ts

# Specific test by name
npx playwright test --grep "Enable Cerberus"

# Debug mode with inspector
npx playwright test --debug

# Generate code (record interactions)
npx playwright codegen http://localhost:8080
```

---

## Project Structure

```
tests/
├── core/                           # Core application tests
├── dns-provider-crud.spec.ts      # DNS provider CRUD tests
├── dns-provider-types.spec.ts     # DNS provider type-specific tests
├── emergency-server/              # Emergency API tests
├── manual-dns-provider.spec.ts    # Manual DNS provider tests
├── monitoring/                     # Uptime monitoring tests
├── security/                       # Security dashboard tests
├── security-enforcement/          # ACL, WAF, Rate Limiting enforcement tests
├── settings/                       # Settings page tests
│   └── system-settings.spec.ts    # Feature flag toggle tests
├── tasks/                          # Async task tests
├── utils/                          # Test helper utilities
│   ├── debug-logger.ts            # Structured logging
│   ├── test-steps.ts              # Step and assertion helpers
│   ├── ui-helpers.ts              # UI interaction helpers (switches, toasts, forms)
│   └── wait-helpers.ts            # Wait/polling utilities (feature flags, API)
├── fixtures/                       # Shared test fixtures
├── reporters/                      # Custom Playwright reporters
├── auth.setup.ts                   # Global authentication setup
└── global-setup.ts                 # Global test initialization
```

---

## Available Helper Functions

### UI Interaction Helpers (`utils/ui-helpers.ts`)

#### Switch/Toggle Components

```typescript
import { clickSwitch, expectSwitchState, toggleSwitch } from './utils/ui-helpers';

// Click a switch reliably (handles hidden input pattern)
await clickSwitch(page.getByRole('switch', { name: /cerberus/i }));

// Assert switch state
await expectSwitchState(switchLocator, true); // Checked
await expectSwitchState(switchLocator, false); // Unchecked

// Toggle and get new state
const newState = await toggleSwitch(switchLocator);
console.log(`Switch is now ${newState ? 'enabled' : 'disabled'}`);
```

**Why**: Switch components use a hidden `<input>` with styled siblings. Direct clicks fail in WebKit/Firefox.

#### Cross-Browser Form Field Locators (Phase 2)

```typescript
import { getFormFieldByLabel } from './utils/ui-helpers';

// Basic usage
const nameInput = getFormFieldByLabel(page, /name/i);
await nameInput.fill('John Doe');

// With fallbacks for robust cross-browser support
const scriptPath = getFormFieldByLabel(
  page,
  /script.*path/i,
  {
    placeholder: /dns-challenge\.sh/i,
    fieldId: 'field-script_path'
  }
);
await scriptPath.fill('/usr/local/bin/dns-challenge.sh');
```

**Why**: Browsers handle label association differently. This helper provides 4-tier fallback:
1. `getByLabel()` — Standard label association
2. `getByPlaceholder()` — Fallback to placeholder text
3. `locator('#id')` — Fallback to direct ID
4. `getByRole()` with proximity — Fallback to role + nearby label text

**Impact**: Prevents timeout errors in Firefox/WebKit.

#### Toast Notifications

```typescript
import { waitForToast, getToastLocator } from './utils/ui-helpers';

// Wait for toast with text
await waitForToast(page, /success/i, { type: 'success', timeout: 5000 });

// Get toast locator for custom assertions
const toast = getToastLocator(page, /error/i, { type: 'error' });
await expect(toast).toBeVisible();
```

### Wait/Polling Helpers (`utils/wait-helpers.ts`)

#### Feature Flag Propagation (Phase 2 Optimized)

```typescript
import { waitForFeatureFlagPropagation } from './utils/wait-helpers';

// Wait for feature flag to propagate after toggle
await clickSwitch(cerberusToggle);
await waitForFeatureFlagPropagation(page, {
  'cerberus.enabled': true
});

// Wait for multiple flags
await waitForFeatureFlagPropagation(page, {
  'cerberus.enabled': false,
  'crowdsec.enabled': false
}, { timeout: 60000 });
```

**Performance**: Includes conditional skip optimization — exits immediately if flags already match.

#### API Responses

```typescript
import { clickAndWaitForResponse, waitForAPIResponse } from './utils/wait-helpers';

// Click and wait for response atomically (prevents race conditions)
const response = await clickAndWaitForResponse(
  page,
  saveButton,
  /\/api\/v1\/proxy-hosts/,
  { status: 200 }
);
expect(response.ok()).toBeTruthy();

// Wait for response without interaction
const response = await waitForAPIResponse(page, /\/api\/v1\/feature-flags/, {
  status: 200,
  timeout: 10000
});
```

#### Retry with Exponential Backoff

```typescript
import { retryAction } from './utils/wait-helpers';

// Retry action with backoff (2s, 4s, 8s)
await retryAction(async () => {
  await clickSwitch(toggle);
  await waitForFeatureFlagPropagation(page, { 'flag': true });
}, { maxAttempts: 3, baseDelay: 2000 });
```

#### Other Wait Utilities

```typescript
// Wait for loading to complete
await waitForLoadingComplete(page, { timeout: 10000 });

// Wait for modal dialog
const modal = await waitForModal(page, /edit.*host/i);
await modal.getByLabel(/domain/i).fill('example.com');

// Wait for table rows
await waitForTableLoad(page, 'table', { minRows: 5 });

// Wait for WebSocket connection
await waitForWebSocketConnection(page, /\/ws\/logs/);
```

### Debug Helpers (`utils/debug-logger.ts`)

```typescript
import { DebugLogger } from './utils/debug-logger';

const logger = new DebugLogger('test-name');

logger.step('Navigate to settings');
logger.network({ method: 'GET', url: '/api/v1/feature-flags', status: 200, elapsedMs: 123 });
logger.assertion('Cerberus toggle is visible', true);
logger.error('Failed to load settings', new Error('Network timeout'));
```

---

## Performance Best Practices (Phase 2)

### 1. Only Poll When State Changes

❌ **Before (Inefficient)**:
```typescript
test.beforeEach(async ({ page }) => {
  // Polls even if flags already correct
  await waitForFeatureFlagPropagation(page, { 'cerberus.enabled': false });
});

test('Test', async ({ page }) => {
  await clickSwitch(toggle);
  await waitForFeatureFlagPropagation(page, { 'cerberus.enabled': true });
});
```

✅ **After (Optimized)**:
```typescript
test.afterEach(async ({ request }) => {
  // Restore defaults once at end
  await request.post('/api/v1/settings/restore', {
    data: { module: 'system', defaults: true }
  });
});

test('Test', async ({ page }) => {
  // Test starts from defaults (no beforeEach poll needed)
  await clickSwitch(toggle);

  // Only poll when state changes
  await waitForFeatureFlagPropagation(page, { 'cerberus.enabled': true });
});
```

**Impact**: Removed ~90% of unnecessary API calls.

### 2. Use Conditional Skip Optimization

The helper automatically checks if flags are already in the expected state:

```typescript
// If flags match, exits immediately (no polling)
await waitForFeatureFlagPropagation(page, { 'cerberus.enabled': false });
// Console: "[POLL] Already in expected state - skipping poll"
```

**Impact**: ~50% reduction in polling iterations.

### 3. Request Coalescing for Parallel Workers

Tests running in parallel share in-flight requests:

```typescript
// Worker 0: Waits for {cerberus: false}
// Worker 1: Waits for {cerberus: false}
// Result: 1 polling loop per worker (cached promise), not 2 separate loops
```

**Cache Key Format**: `[worker_index]:[sorted_flags_json]`

---

## Test Isolation Pattern

Always clean up in `afterEach`, not `beforeEach`:

```typescript
test.describe('Feature', () => {
  test.afterEach(async ({ request }) => {
    // Restore defaults after each test
    await request.post('/api/v1/settings/restore', {
      data: { module: 'system', defaults: true }
    });
  });

  test('Test A', async ({ page }) => {
    // Starts from defaults (restored by previous test)
    // ...test logic...
    // Cleanup happens in afterEach
  });

  test('Test B', async ({ page }) => {
    // Also starts from defaults
  });
});
```

**Why**: Prevents test pollution and ensures each test starts from known state.

---

## Common Patterns

### Toggle Feature Flag

```typescript
test('Enable feature', async ({ page }) => {
  const toggle = page.getByRole('switch', { name: /feature/i });

  await test.step('Toggle on', async () => {
    await clickSwitch(toggle);
    await waitForFeatureFlagPropagation(page, { 'feature.enabled': true });
  });

  await test.step('Verify UI', async () => {
    await expectSwitchState(toggle, true);
  });
});
```

### Create Resource via API, Verify in UI

```typescript
test('Create proxy host', async ({ page, testData }) => {
  await test.step('Create via API', async () => {
    const host = await testData.createProxyHost({
      domain: 'example.com',
      forward_host: '192.168.1.100'
    });
  });

  await test.step('Verify in UI', async () => {
    await page.goto('/proxy-hosts');
    await waitForResourceInUI(page, 'example.com');
  });
});
```

### Wait for Async Task

```typescript
test('Start long task', async ({ page }) => {
  await page.getByRole('button', { name: /start/i }).click();

  // Wait for progress bar
  await waitForProgressComplete(page, { timeout: 30000 });

  // Verify completion
  await expect(page.getByText(/complete/i)).toBeVisible();
});
```

---

## Cross-Browser Compatibility

| Strategy | Purpose | Supported Browsers |
|----------|---------|-------------------|
| `getFormFieldByLabel()` | Form field location | ✅ Chromium ✅ Firefox ✅ WebKit |
| `clickSwitch()` | Switch interaction | ✅ Chromium ✅ Firefox ✅ WebKit |
| `getByRole()` | Semantic locators | ✅ Chromium ✅ Firefox ✅ WebKit |

**Avoid**:
- CSS selectors (brittle, browser-specific)
- `{ force: true }` clicks (bypasses real user behavior)
- `waitForTimeout()` (non-deterministic)

---

## Troubleshooting

### Test Fails in Firefox/WebKit Only

**Symptom**: `TimeoutError: locator.fill: Timeout exceeded`

**Cause**: Label matching differs between browsers.

**Fix**: Use `getFormFieldByLabel()` with fallbacks:
```typescript
const field = getFormFieldByLabel(page, /field name/i, {
  placeholder: /enter value/i
});
```

### Feature Flag Polling Times Out

**Symptom**: `Feature flag propagation timeout after 120 attempts`

**Causes**:
1. Config reload overlay stuck visible
2. Backend not updating flags
3. Database transaction not committed

**Fix**:
1. Check backend logs for PUT `/api/v1/feature-flags` errors
2. Check if overlay is stuck: `page.locator('[data-testid="config-reload-overlay"]').isVisible()`
3. Add retry wrapper:
```typescript
await retryAction(async () => {
  await clickSwitch(toggle);
  await waitForFeatureFlagPropagation(page, { 'flag': true });
});
```

### Switch Click Intercepted

**Symptom**: `click intercepted by overlay`

**Cause**: Config reload overlay or sticky header blocking interaction.

**Fix**: Use `clickSwitch()` (handles overlay automatically):
```typescript
await clickSwitch(page.getByRole('switch', { name: /feature/i }));
```

---

## Test Execution Metrics (Phase 2)

| Metric | Before Phase 2 | After Phase 2 | Improvement |
|--------|----------------|---------------|-------------|
| System Settings Tests | 23 minutes | 16 minutes | 31% faster |
| Feature Flag API Calls | ~300 calls | ~30 calls | 90% reduction |
| Polling Iterations (avg) | 60 per test | 30 per test | 50% reduction |
| Cross-Browser Pass Rate | 96% (Firefox flaky) | 100% (all browsers) | +4% |

---

## Documentation

- **[Testing README](../docs/testing/README.md)** — Quick reference, debugging, VS Code tasks
- **[E2E Test Writing Guide](../docs/testing/e2e-test-writing-guide.md)** — Comprehensive best practices
- **[Debugging Guide](../docs/testing/debugging-guide.md)** — Troubleshooting guide
- **[Security Helpers](../docs/testing/security-helpers.md)** — ACL/WAF/CrowdSec test utilities

---

## CI/CD Integration

Tests run on every PR and push:

- **Browsers**: Chromium, Firefox, WebKit
- **Sharding**: 4 parallel workers per browser
- **Artifacts**: Videos (on failure), traces, screenshots, logs
- **Reports**: HTML report, GitHub Job Summary

See [`.github/workflows/playwright.yml`](../.github/workflows/playwright.yml) for full CI configuration.

---

**Questions?** See [docs/testing/](../docs/testing/) or open an issue.
