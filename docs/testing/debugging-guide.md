# Playwright E2E Test Debugging Guide

This guide explains how to use the enhanced debugging features in the Playwright E2E test suite.

## Quick Start

### Local Testing with Debug Logging

To run tests with enhanced debug output locally:

```bash
# Test with full debug logging and colors
npm run e2e

# Or with more detailed logging
DEBUG=charon:*,charon-test:* npm run e2e
```

### VS Code Debug Tasks

Several new tasks are available in VS Code for debugging:

1. **Test: E2E Playwright (Debug Mode - Full Traces)**
   - Runs tests in debug mode with full trace capture
   - Opens Playwright Inspector for step-by-step execution
   - Command: `Debug=charon:*,charon-test:* npx playwright test --debug --trace=on`
   - **Use when**: You need to step through test execution interactively

2. **Test: E2E Playwright (Debug with Logging)**
   - Runs tests with enhanced logging output
   - Shows network activity and page state
   - Command: `DEBUG=charon:*,charon-test:* PLAYWRIGHT_DEBUG=1 npx playwright test --project=chromium`
   - **Use when**: You want to see detailed logs without interactive debugging

3. **Test: E2E Playwright (Trace Inspector)**
   - Opens the Playwright Trace Viewer
   - Inspect recorded traces with full DOM/network/console logs
   - Command: `npx playwright show-trace <trace.zip>`
   - **Use when**: You've captured traces and want to inspect them

4. **Test: E2E Playwright - View Coverage Report**
   - Opens the E2E coverage report in browser
   - Shows which code paths were exercised during tests
   - **Use when**: Analyzing code coverage from E2E tests

## Understanding the Debug Logger

The debug logger provides structured logging with multiple methods:

### Logger Methods

#### `step(name: string, duration?: number)`

Logs a test step with automatic duration tracking.

```typescript
const logger = new DebugLogger('my-test');
logger.step('Navigate to home page');
logger.step('Click login button', 245); // with duration in ms
```

**Output:**
```
├─ Navigate to home page
├─ Click login button (245ms)
```

#### `network(entry: NetworkLogEntry)`

Logs HTTP requests and responses with timing and status.

```typescript
logger.network({
  method: 'POST',
  url: 'https://api.example.com/login',
  status: 200,
  elapsedMs: 342,
  responseContentType: 'application/json',
  responseBodySize: 1024
});
```

**Output:**
```
✅ POST https://api.example.com/login [200] 342ms
```

#### `locator(selector, action, found, elapsedMs)`

Logs element interactions and locator resolution.

```typescript
logger.locator('[role="button"]', 'click', true, 45);
```

**Output:**
```
✓ click "[role="button"]" 45ms
```

#### `assertion(condition, passed, actual?, expected?)`

Logs test assertions with pass/fail status.

```typescript
logger.assertion('Button is visible', true);
logger.assertion('URL is correct', false, 'http://old.com', 'http://new.com');
```

**Output:**
```
✓ Assert: Button is visible
✗ Assert: URL is correct | expected: "http://new.com", actual: "http://old.com"
```

#### `error(context, error, recoveryAttempts?)`

Logs errors with context and recovery information.

```typescript
logger.error('Network request failed', new Error('TIMEOUT'), 1);
```

**Output:**
```
❌ ERROR: Network request failed - TIMEOUT
🔄 Recovery: 1 attempts remaining
```

## Local Trace Capture

Traces capture all interactions, network activity, and DOM snapshots. They're invaluable for debugging.

### Automatic Trace Capture

Traces are automatically captured:
- On first retry of failed tests
- On failure when running locally (if configured)

### Manual Trace Capture

To capture traces for all tests locally:

```bash
npx playwright test --trace=on
```

Or in code:

```typescript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  use: {
    trace: 'on', // always capture
  },
});
```

### Viewing Traces

After tests run, view traces with:

```bash
npx playwright show-trace test-results/path/to/trace.zip
```

The Trace Viewer shows:
- **Timeline**: Chronological list of all actions
- **Network**: HTTP requests/responses with full details
- **Console**: Page JS console output
- **DOM**: DOM snapshot at each step
- **Sources**: Source code view

## CI Debugging

### Viewing CI Test Results

When tests fail in CI/CD:

1. Go to the workflow run in GitHub Actions
2. Check the **E2E Tests** job summary for per-shard status
3. Download artifacts:
   - `merged-playwright-report/` - HTML test report
   - `traces-*-shard-*/` - Trace files for failures
   - `docker-logs-shard-*/` - Application logs
   - `test-results-*-shard-*/` - Raw test data

### Interpreting CI Logs

Each shard logs its execution with timing:

```
════════════════════════════════════════════════════════════
E2E Test Shard 1/4
Browser: chromium
Start Time: 2024-01-27T10:30:45Z
════════════════════════════════════════════════════════════
...
════════════════════════════════════════════════════════════
Shard 1 Complete | Duration: 125s
════════════════════════════════════════════════════════════
```

The merged report summary shows:

```
╔════════════════════════════════════════════════════════════╗
║              E2E Test Execution Summary                      ║
╠════════════════════════════════════════════════════════════╣
║ Total Tests:        150                                     ║
║ ✅ Passed:          145 (96%)                               ║
║ ❌ Failed:          5                                       ║
║ ⏭️  Skipped:         0                                       ║
╚════════════════════════════════════════════════════════════╝
```

### Failure Analysis

CI logs include failure categorization:

```
🔍 Failure Analysis by Type:
────────────────────────────────────────────────────────────
timeout      │ ████░░░░░░░░░░░░░░░░░ 2/5 (40%)
assertion    │ ██░░░░░░░░░░░░░░░░░░  2/5 (40%)
network      │ ░░░░░░░░░░░░░░░░░░░░  1/5 (20%)
```

And slowest tests:

```
⏱️  Slow Tests (>5s):
────────────────────────────────────────────────────────────
1. Long-running test name               12.43s
2. Another slow test                     8.92s
3. Network-heavy test                    6.15s
```

## Network Debugging

The network interceptor captures all HTTP traffic:

### Viewing Network Logs

Network logs appear in console output:

```
✅ GET https://api.example.com/health [200] 156ms
⚠️ POST https://api.example.com/user [429] 1234ms
❌ GET https://cdn.example.com/asset [timeout] 5000ms
```

### Exporting Network Data

To export network logs for analysis:

```typescript
import { createNetworkInterceptor } from './fixtures/network';

test('example', async ({ page }) => {
  const interceptor = createNetworkInterceptor(page, logger);

  // ... run test ...

  // Export as CSV
  const csv = interceptor.exportCSV();
  await fs.writeFile('network.csv', csv);

  // Or JSON
  const json = interceptor.exportJSON();
  await fs.writeFile('network.json', JSON.stringify(json));
});
```

### Network Metrics Available

- **Request Headers**: Sanitized (auth tokens redacted)
- **Response Headers**: Sanitized
- **Status Code**: HTTP response code
- **Duration**: Total request time
- **Request Size**: Bytes sent
- **Response Size**: Bytes received
- **Content Type**: Response MIME type
- **Redirect Chain**: Followed redirects
- **Errors**: Network error messages

## Debug Output Formats

### Local Console Output (Colors)

When running locally, output uses ANSI colors for readability:

- 🔵 Blue: Steps
- 🟢 Green: Successful assertions/locators
- 🟡 Yellow: Warnings (missing locators, slow operations)
- 🔴 Red: Errors
- 🔵 Cyan: Network activity

### CI JSON Output

In CI, the same information is formatted as JSON for parsing:

```json
{
  "type": "step",
  "message": "├─ Navigate to home page",
  "timestamp": "2024-01-27T10:30:45.123Z"
}
```

## Common Debugging Scenarios

### Test is Timing Out

1. **Check traces**: Download and inspect with `npx playwright show-trace`
2. **Check logs**: Look for "⏳" (waiting) or "⏭️" (skipped) markers
3. **Check network**: Look for slow network requests in the network CSV
4. **Increase timeout**: Run with `--timeout=60000` locally to get more data

### Test is Flaky (Sometimes Fails)

1. **Check timing**: Look for operations near the 5000ms assertion timeout
2. **Check network**: Look for variable response times
3. **Check logs**: Search for race conditions ("expected X but got Y sometimes")
4. **Re-run locally**: Use `npm run e2e -- --grep="flaky test"` multiple times

### Test Fails on CI but Passes Locally

1. **Compare environments**: Check if URLs/tokens differ (**Check $PLAYWRIGHT_BASE_URL**)
2. **Check Docker logs**: Look for backend errors in `docker-logs-*.txt`
3. **Check timing**: CI machines are often slower; increase timeouts
4. **Check parallelization**: Try running shards sequentially locally

### Network Errors in Tests

1. **Check network CSV**: Export and analyze request times
2. **Check status codes**: Look for 429 (rate limit), 503 (unavailable), etc.
3. **Check headers**: Verify auth tokens are being sent correctly (watch for `[REDACTED]`)
4. **Check logs**: Look for error messages in response bodies

## Performance Analysis

### Identifying Slow Tests

Tests slower than 5 seconds are automatically highlighted:

```bash
npm run e2e  # Shows "Slow Tests (>5s)" in summary
```

And in CI:

```
⏱️  Slow Tests (>5s):
────────────────────────────────────────────────────────────
1. test name               12.43s
```

### Analyzing Step Duration

The debug logger tracks step duration:

```typescript
const logger = new DebugLogger('test-name');
logger.step('Load page', 456);
logger.step('Submit form', 234);

// Slowest operations automatically reported
logger.printSummary(); // Shows per-step breakdown
```

### Network Performance

Check average response times by endpoint:

```typescript
const interceptor = createNetworkInterceptor(page, logger);
// ... run test ...
const avgTimes = interceptor.getAverageResponseTimeByPattern();
// {
//   'https://api.example.com/login': 234,
//   'https://api.example.com/health': 45,
// }
```

## Environment Variables

### Debugging Environment Variables

These can be set to control logging:

```bash
# Enable debug namespace logging
DEBUG=charon:*,charon-test:*

# Enable Playwright debugging
PLAYWRIGHT_DEBUG=1

# Set custom base URL
PLAYWRIGHT_BASE_URL=http://localhost:8080

# Set CI log level
CI_LOG_LEVEL=verbose
```

### In GitHub Actions

Environment variables are set automatically for CI runs:

```yaml
env:
  DEBUG: 'charon:*,charon-test:*'
  PLAYWRIGHT_DEBUG: '1'
  CI_LOG_LEVEL: 'verbose'
```

## Testing Test Utilities Locally

### Test the Debug Logger

```typescript
import { DebugLogger } from '../utils/debug-logger';

const logger = new DebugLogger({
  testName: 'my-test',
  browser: 'chromium',
  file: 'test.spec.ts'
});

logger.step('Step 1', 100);
logger.network({
  method: 'GET',
  url: 'https://example.com',
  status: 200,
  elapsedMs: 156
});
logger.assertion('Check result', true);
logger.printSummary();
```

### Test the Network Interceptor

```typescript
import { createNetworkInterceptor } from '../fixtures/network';

test('network test', async ({ page }) => {
  const interceptor = createNetworkInterceptor(page);

  await page.goto('https://example.com');

  const csv = interceptor.exportCSV();
  console.log(csv);

  const slowRequests = interceptor.getSlowRequests(1000);
  console.log(`Requests >1s: ${slowRequests.length}`);
});
```

## UI Interaction Helpers

### Switch/Toggle Helpers

The `tests/utils/ui-helpers.ts` file provides helpers for reliable Switch/Toggle interactions.

**Problem**: Switch components use a hidden `<input>` with styled siblings, causing Playwright's `click()` to fail with "pointer events intercepted" errors.

**Solution**: Use the switch helper functions:

```typescript
import { clickSwitch, expectSwitchState, toggleSwitch } from '../utils/ui-helpers';

test('should toggle security features', async ({ page }) => {
  await page.goto('/settings');

  // ✅ GOOD: Click switch reliably
  const aclSwitch = page.getByRole('switch', { name: /acl/i });
  await clickSwitch(aclSwitch);

  // ✅ GOOD: Assert switch state
  await expectSwitchState(aclSwitch, true);

  // ✅ GOOD: Toggle and get new state
  const isEnabled = await toggleSwitch(aclSwitch);
  console.log(`ACL is now ${isEnabled ? 'enabled' : 'disabled'}`);

  // ❌ BAD: Direct click (fails in WebKit/Firefox)
  await aclSwitch.click({ force: true }); // Don't use force!
});
```

**Key Features**:
- Automatically finds parent `<label>` element
- Scrolls element into view (sticky header aware)
- Cross-browser compatible (Chromium, Firefox, WebKit)
- No `force: true` or hard-coded waits needed

**When to Use**:
- Any test that clicks Switch/Toggle components
- Settings pages with enable/disable toggles
- Security dashboard module toggles (CrowdSec, ACL, WAF, Rate Limiting)
- Access lists and configuration toggles

**References**:
- [Implementation](../../tests/utils/ui-helpers.ts) - Full helper code
- [QA Report](../reports/qa_report.md) - Test results and validation

## Troubleshooting Debug Features

### Traces Not Captured

- Ensure `trace: 'on-first-retry'` or `trace: 'on'` is set in config
- Check that `test-results/` directory exists and is writable
- Verify test fails (traces only captured on retry/failure by default)

### Logs Not Appearing

- Check if running in CI (JSON format instead of colored output)
- Set `DEBUG=charon:*` environment variable
- Ensure `CI` environment variable is not set for local runs

### Reporter Errors

- Verify `tests/reporters/debug-reporter.ts` exists
- Check TypeScript compilation errors: `npx tsc --noEmit`
- Run with `--reporter=list` as fallback

## Further Reading

- [Playwright Debugging Docs](https://playwright.dev/docs/debug)
- [Playwright Trace Viewer](https://playwright.dev/docs/trace-viewer)
- [Test Reporters](https://playwright.dev/docs/test-reporters)
- [Debugging in VS Code](https://playwright.dev/docs/debug#vs-code-debugger)
