# E2E Testing & Debugging Guide

## Quick Navigation

### Getting Started with E2E Tests
- **Running Tests**: `npm run e2e`
- **All Browsers**: `npm run e2e:all`
- **Headed Mode**: `npm run e2e:headed`

### Debugging Features

This project includes comprehensive debugging enhancements for Playwright E2E tests.

#### 📚 Documentation
- [Debugging Guide](./debugging-guide.md) - Complete guide to debugging features
- [Implementation Summary](./DEBUGGING_IMPLEMENTATION.md) - Technical implementation details

#### 🛠️ VS Code Debug Tasks

Five new debug tasks are available in VS Code:

1. **Test: E2E Playwright (Debug Mode - Full Traces)**
   - Interactive debugging with Playwright Inspector
   - Full trace capture during execution
   - Best for: Step-by-step test analysis

2. **Test: E2E Playwright (Debug with Logging)**
   - Enhanced console output with timing
   - Network activity logging
   - Best for: Understanding test flow without interactive mode

3. **Test: E2E Playwright (Trace Inspector)**
   - Opens recorded trace files in Playwright Trace Viewer
   - Best for: Analyzing traces from previous test runs

4. **Test: E2E Playwright - View Coverage Report**
   - Opens E2E code coverage in browser
   - Best for: Analyzing test coverage metrics

5. **Test: E2E Playwright - View Report** (existing)
   - Opens HTML test report
   - Best for: Quick results overview

#### 📊 Debugging Utilities Available

**Debug Logger** (`tests/utils/debug-logger.ts`)
```typescript
const logger = new DebugLogger('test-name');
logger.step('Action description');
logger.network({ method, url, status, elapsedMs });
logger.assertion('Expected behavior', passed);
logger.error('Error context', error);
```

**Network Interceptor** (`tests/fixtures/network.ts`)
```typescript
const interceptor = createNetworkInterceptor(page, logger);
// ... test runs ...
const csv = interceptor.exportCSV();
```

**Test Step Helpers** (`tests/utils/test-steps.ts`)
```typescript
await testStep('Describe action', async () => {
  // test code
}, { logger });

await testAssert('Check result', assertion, logger);
```

**Switch/Toggle Helpers** (`tests/utils/ui-helpers.ts`)
```typescript
import { clickSwitch, expectSwitchState, toggleSwitch } from './utils/ui-helpers';

// Click a switch reliably (handles hidden input pattern)
await clickSwitch(page.getByRole('switch', { name: /cerberus/i }));

// Assert switch state
await expectSwitchState(switchLocator, true); // Checked
await expectSwitchState(switchLocator, false); // Unchecked

// Toggle and get new state
const newState = await toggleSwitch(switchLocator);
```

#### � Switch/Toggle Component Testing

**Problem**: Switch components use a hidden `<input>` with a styled sibling, causing "pointer events intercepted" errors.

**Solution**: Use the switch helper functions in `tests/utils/ui-helpers.ts`:

```typescript
import { clickSwitch, expectSwitchState, toggleSwitch } from './utils/ui-helpers';

// ✅ GOOD: Use clickSwitch helper
await clickSwitch(page.getByRole('switch', { name: /enable cerberus/i }));

// ✅ GOOD: Assert state after change
await expectSwitchState(page.getByRole('switch', { name: /acl/i }), true);

// ✅ GOOD: Toggle and get new state
const isEnabled = await toggleSwitch(page.getByRole('switch', { name: /waf/i }));

// ❌ BAD: Direct click on hidden input (fails in WebKit/Firefox)
await page.getByRole('switch').click({ force: true }); // Don't use force!
```

**Key Features**:
- Automatically handles hidden input pattern
- Scrolls element into view (sticky header aware)
- Cross-browser compatible (Chromium, Firefox, WebKit)
- No `force: true` or hard-coded waits needed

**When to Use**:
- Any test that clicks Switch/Toggle components
- Settings pages with enable/disable toggles
- Security dashboard module toggles
- Access lists, WAF, rate limiting controls

**References**:
- [Implementation](../../tests/utils/ui-helpers.ts) - Full helper code
- [QA Report](../reports/qa_report.md) - Test results and validation

---

#### �🔍 Common Debugging Tasks

**See test output with colors:**
```bash
npm run e2e
```

**Run specific test with debug mode:**
```bash
npm run e2e -- --grep="test name"
```

**Run with full debug logging:**
```bash
DEBUG=charon:*,charon-test:* npm run e2e
```

**View test report:**
```bash
npx playwright show-report
```

**Inspect a trace file:**
```bash
npx playwright show-trace test-results/[test-name]/trace.zip
```

#### 📋 CI Features

When tests run in CI/CD:

- **Per-shard summaries** with timing for parallel tracking
- **Failure categorization** (timeout, assertion, network)
- **Slowest tests** automatically highlighted (>5s)
- **Job summary** with links to artifacts
- **Enhanced logs** for debugging CI failures

#### 🎯 Key Features

| Feature | Purpose | File |
|---------|---------|------|
| Debug Logger | Structured logging with timing | `tests/utils/debug-logger.ts` |
| Network Interceptor | HTTP request/response capture | `tests/fixtures/network.ts` |
| Test Helpers | Step and assertion logging | `tests/utils/test-steps.ts` |
| Switch Helpers | Reliable toggle/switch interactions | `tests/utils/ui-helpers.ts` |
| Reporter | Failure analysis and statistics | `tests/reporters/debug-reporter.ts` |
| Global Setup | Enhanced initialization logging | `tests/global-setup.ts` |
| Config | Trace/video/screenshot setup | `playwright.config.js` |
| Tasks | VS Code debug commands | `.vscode/tasks.json` |
| CI Workflow | Per-shard logging and summaries | `.github/workflows/e2e-tests.yml` |

#### 📈 Output Examples

**Local Test Run:**
```
├─ Navigate to home page
├─ Click login button (234ms)
   ✅ POST https://api.example.com/login [200] 342ms
   ✓ click "[role='button']" 45ms
   ✓ Assert: Button is visible
```

**Test Summary:**
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

#### 🚀 Performance Analysis

Slow tests (>5s) are automatically reported:
```
⏱️  Slow Tests (>5s):
1. Complex test name           12.43s
2. Another slow test            8.92s
3. Network-heavy test           6.15s
```

Failures are categorized:
```
🔍 Failure Analysis by Type:
timeout      │ ████░░░░░░░░░░░░░░░░░ 2/5 (40%)
assertion    │ ██░░░░░░░░░░░░░░░░░░  2/5 (40%)
network      │ ░░░░░░░░░░░░░░░░░░░░  1/5 (20%)
```

#### 📦 What's Captured

- **Videos**: Recorded on failure (Visual debugging)
- **Traces**: Full interaction traces (Network, DOM, Console)
- **Screenshots**: On failure only
- **Network Logs**: CSV export of all HTTP traffic
- **Docker Logs**: Application logs on failure

#### 🔧 Configuration

Environment variables for debugging:
```bash
DEBUG=charon:*,charon-test:*    # Enable debug logging
PLAYWRIGHT_DEBUG=1               # Playwright debug mode
PLAYWRIGHT_BASE_URL=...          # Override application URL
CI_LOG_LEVEL=verbose             # CI log level
```

#### 📖 Additional Resources

- [Complete Debugging Guide](./debugging-guide.md) - Detailed usage for all features
- [Implementation Summary](./DEBUGGING_IMPLEMENTATION.md) - Technical details and file inventory
- [Playwright Docs](https://playwright.dev/docs/debug) - Official debugging docs

---

## File Structure

```
docs/testing/
├── README.md                           # This file
├── debugging-guide.md                  # Complete debugging guide
└── DEBUGGING_IMPLEMENTATION.md         # Implementation details

tests/
├── utils/
│   ├── debug-logger.ts                 # Core logging utility
│   └── test-steps.ts                   # Step/assertion helpers
├── fixtures/
│   └── network.ts                      # Network interceptor
└── reporters/
    └── debug-reporter.ts               # Custom Playwright reporter

.vscode/
└── tasks.json                          # Updated with 4 new debug tasks

playwright.config.js                    # Updated with trace/video config

.github/workflows/
└── e2e-tests.yml                       # Enhanced with per-shard logging
```

## Quick Links

- **Run Tests**: See [Debugging Guide - Quick Start](./debugging-guide.md#quick-start)
- **Local Debugging**: See [Debugging Guide - VS Code Tasks](./debugging-guide.md#vs-code-debug-tasks)
- **CI Debugging**: See [Debugging Guide - CI Debugging](./debugging-guide.md#ci-debugging)
- **Troubleshooting**: See [Debugging Guide - Troubleshooting](./debugging-guide.md#troubleshooting-debug-features)

---

**Total Implementation**: 2,144 lines of new code and documentation
**Status**: ✅ Complete and ready to use
**Date**: January 27, 2026
