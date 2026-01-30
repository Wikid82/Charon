# Security Test Helpers

Helper utilities for managing security module state during E2E tests.

## Overview

The security helpers module (`tests/utils/security-helpers.ts`) provides utilities for:

- Capturing and restoring security module state
- Toggling individual security modules (ACL, WAF, Rate Limiting, CrowdSec)
- Ensuring test isolation without ACL deadlock

## Problem Solved

During E2E testing, if ACL is left enabled from a previous test run (e.g., due to test failure), it creates a **deadlock**:

1. ACL blocks API requests → returns 403 Forbidden
2. Global cleanup can't run → API blocked
3. Auth setup fails → tests skip
4. Manual intervention required to reset volumes

The security helpers solve this by using Playwright's `test.afterAll()` fixture to guarantee cleanup even when tests fail.

## Usage

### Capture and Restore Pattern

```typescript
import { captureSecurityState, restoreSecurityState } from '../utils/security-helpers';
import { request } from '@playwright/test';

let originalState;

test.beforeAll(async ({ request: reqFixture }) => {
  originalState = await captureSecurityState(reqFixture);
});

test.afterAll(async () => {
  const cleanup = await request.newContext({ baseURL: '...' });
  try {
    await restoreSecurityState(cleanup, originalState);
  } finally {
    await cleanup.dispose();
  }
});
```

### Toggle Security Module

```typescript
import { setSecurityModuleEnabled } from '../utils/security-helpers';

await setSecurityModuleEnabled(request, 'acl', true);
await setSecurityModuleEnabled(request, 'waf', false);
```

### With Guaranteed Cleanup

```typescript
import { withSecurityEnabled } from '../utils/security-helpers';

test.describe('ACL Tests', () => {
  let cleanup: () => Promise<void>;

  test.beforeAll(async ({ request }) => {
    cleanup = await withSecurityEnabled(request, { acl: true, cerberus: true });
  });

  test.afterAll(async () => {
    await cleanup();
  });

  test('should enforce ACL', async ({ page }) => {
    // ACL is now enabled, test enforcement
  });
});
```

## Functions

| Function | Purpose |
|----------|---------|
| `getSecurityStatus` | Fetch current security module states |
| `setSecurityModuleEnabled` | Toggle a specific module on/off |
| `captureSecurityState` | Snapshot all module states |
| `restoreSecurityState` | Restore to captured snapshot |
| `withSecurityEnabled` | Enable modules with guaranteed cleanup |
| `disableAllSecurityModules` | Emergency reset |

## API Endpoints Used

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/security/status` | GET | Returns current state of all security modules |
| `/api/v1/settings` | POST | Toggle settings with `{ key: "...", value: "true/false" }` |

## Settings Keys

| Key | Values | Description |
|-----|--------|-------------|
| `security.acl.enabled` | `"true"` / `"false"` | Toggle ACL enforcement |
| `security.waf.enabled` | `"true"` / `"false"` | Toggle WAF enforcement |
| `security.rate_limit.enabled` | `"true"` / `"false"` | Toggle Rate Limiting |
| `security.crowdsec.enabled` | `"true"` / `"false"` | Toggle CrowdSec |
| `feature.cerberus.enabled` | `"true"` / `"false"` | Master toggle for all security |

## Best Practices

1. **Always use `test.afterAll`** for cleanup - it runs even when tests fail
2. **Capture state before modifying** - enables precise restoration
3. **Enable Cerberus first** - it's the master toggle for all security modules
4. **Don't toggle back in individual tests** - let `afterAll` handle cleanup
5. **Use `withSecurityEnabled`** for the cleanest pattern

## Troubleshooting

### ACL Deadlock Recovery

If the test suite is stuck due to ACL deadlock:

```bash
# Check current security status
curl http://localhost:8080/api/v1/security/status

# Manually disable ACL (requires auth)
curl -X POST http://localhost:8080/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"key": "security.acl.enabled", "value": "false"}'
```

### Complete Reset

Use `disableAllSecurityModules` in global setup to ensure clean slate:

```typescript
import { disableAllSecurityModules } from './utils/security-helpers';

async function globalSetup() {
  const context = await request.newContext({ baseURL: '...' });
  await disableAllSecurityModules(context);
  await context.dispose();
}
```
