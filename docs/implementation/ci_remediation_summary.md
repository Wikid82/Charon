# CI Remediation Summary

**Date**: February 5, 2026
**Task**: Stabilize E2E testing pipeline and fix workflow timeouts.

## Problem
The end-to-end (E2E) testing pipeline was experiencing significant instability, characterized by:
1.  **Workflow Timeouts**: Shard 4 was consistently timing out (>20 minutes), obstructing the CI process.
2.  **Missing Dependencies**: Security jobs for Firefox and WebKit were failing because they lacked the required Chromium dependency.
3.  **Flaky Tests**:
    -   `certificates.spec.ts` failed intermittently due to race conditions when ensuring either an empty state or a table was visible.
    -   `crowdsec-import.spec.ts` failed due to transient locks on the backend API.

## Solution

### Workflow Optimization
-   **Shard Rebalancing**: Reduced the number of shards from 4 to 3. This seemingly counter-intuitive move rebalanced the test load, preventing the specific bottlenecks that were causing Shard 4 to hang.
-   **Dependency Fix**: Explicitly added the Chromium installation step to Firefox and WebKit security jobs to ensure all shared test utilities function correctly.

### Test Logic Improvements
-   **Robust Empty State Detection**: Replaced fragile boolean checks with Playwright's `.or()` locator pattern.
    -   *Old*: `isVisible().catch()` (Bypassed auto-waits, led to race conditions)
    -   *New*: `expect(locatorA.or(locatorB)).toBeVisible()` (Leverages built-in retry logic)
-   **Resilient API Retries**: Implemented `.toPass()` for the CrowdSec import test.
    -   This allows the test to automatically retry the import request with exponential backoff if the backend is temporarily locked or busy, significantly reducing flakes.

## Results
-   **Stability**: The "Empty State OR Table" flake in certificates is resolved.
-   **Reliability**: CrowdSec import tests now handle transient backend states gracefully.
-   **Performance**: CI jobs now complete within the allocated time budget with balanced shards.
