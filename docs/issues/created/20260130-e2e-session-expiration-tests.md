# [E2E] Fix Session Expiration Test Failures

## Summary

3 tests in `tests/core/authentication.spec.ts` are failing due to difficulty simulating session expiration scenarios.

## Failing Tests

1. `should clear authentication cookies on logout` (line 219)
2. `should redirect to login when session expires` (line 310)
3. `should handle 401 response gracefully` (line 335)

## Root Cause

These tests require either:

1. Backend API endpoint to invalidate sessions programmatically
2. Playwright route interception to mock 401 responses

## Proposed Solution

Add a route interception utility in `tests/utils/route-mocks.ts`:

```typescript
export async function mockAuthenticationFailure(page: Page) {
  await page.route('**/api/v1/**', route => {
    route.fulfill({ status: 401, body: JSON.stringify({ error: 'Unauthorized' }) });
  });
}
```

## Priority

Medium - Edge case handling, does not block core functionality testing

## Labels

- e2e-testing
- phase-2
- enhancement

## Phase

Phase 2 - Critical Path
