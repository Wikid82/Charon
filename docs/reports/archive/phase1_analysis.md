# Phase 1.1: Test Execution Order Analysis

**Date:** February 2, 2026
**Phase:** Analyze Test Execution Order
**Duration:** 30 minutes

## Current Configuration Analysis

### Project Dependency Chain (playwright.config.js:195-223)

```
setup (auth)
   ↓
security-tests (sequential, 1 worker, headless chromium)
   ↓
security-teardown (cleanup)
   ↓
┌──────────┬──────────┬──────────┐
│ chromium │ firefox  │ webkit   │  ← Parallel execution (no inter-dependencies)
└──────────┴──────────┴──────────┘
```

**Configuration Details:**
- **Workers (CI):** `workers: 1` (Line 116) - Forces sequential execution
- **Retries (CI):** `retries: 2` (Line 114) - Tests retry twice on failure
- **Timeout:** 90s per test (Line 108)
- **Dependencies:** Browser projects depend on `setup` and `security-tests`, NOT on each other

### Why Sequential Execution Amplifies Failure

**The Problem:**

With `workers: 1` in CI, Playwright runs ALL projects sequentially in a single worker:

```
Worker 1: [setup] → [security-tests] → [security-teardown] → [chromium] → [firefox] → [webkit]
```

**When Chromium encounters an interruption** (not a normal failure):
1. Error: `Target page, context or browser has been closed` at test #263
2. This is an **INTERRUPTION**, not a normal test failure
3. The worker encounters an unrecoverable error (browser context closed unexpectedly)
4. **Playwright terminates the worker** to prevent cascading failures
5. Since there's only 1 worker, **the entire test run terminates**
6. Firefox and WebKit never start - marked as "did not run"

**Root Cause:** The interruption is treated as a fatal worker error, not a test failure.

### Interruption vs Failure

| Type | Behavior | Impact |
|------|----------|--------|
| **Normal Failure** | Test fails assertion, runner continues | Next test runs |
| **Interruption** | Browser/context closed unexpectedly | Worker terminates |
| **Timeout** | Test exceeds 90s, marked as timeout | Next test runs |
| **Error** | Uncaught exception, test marked as error | Next test runs |

**Interruptions are non-recoverable** - they indicate the test environment is in an inconsistent state.

### Current GitHub Actions Architecture

**Current workflow uses matrix sharding:**
```yaml
strategy:
  matrix:
    shard: [1, 2, 3, 4]
    browser: [chromium, firefox, webkit]
```

This creates 12 jobs:
- chromium-shard-1, chromium-shard-2, chromium-shard-3, chromium-shard-4
- firefox-shard-1, firefox-shard-2, firefox-shard-3, firefox-shard-4
- webkit-shard-1, webkit-shard-2, webkit-shard-3, webkit-shard-4

**BUT:** All jobs run in the same `e2e-tests` job definition. If one browser has issues, it affects that browser's shards only.

**The issue:** The sharding is already browser-isolated at the GitHub Actions level. The problem is likely in **local testing** or in how the interruption is being reported.

### Analysis Conclusion

**Finding:** The GitHub Actions workflow is ALREADY browser-isolated via matrix strategy. Each browser runs in separate jobs.

**The Real Problem:**
1. The diagnostic report shows Chromium interrupted at test #263
2. Firefox and WebKit show "did not run" (0 tests executed)
3. This suggests the issue is in the **Playwright CLI command** or **local testing**, NOT GitHub Actions

**Next Steps:**
1. Verify if the issue is in local testing vs CI
2. Check if there's a project dependency issue in playwright.config.js
3. Implement Phase 1.2 hotfix to ensure complete browser isolation
4. Add diagnostic logging to capture the actual interruption error

**Recommendation:** Proceed with Phase 1.2 to add explicit browser job separation and enhanced logging.
