
**Status**: ✅ RESOLVED (January 30, 2026)

## Summary

The run failed on main while passing on feature and development branches.

## Failure details

The primary error is a socket hang up during a security test in
`zzz-admin-whitelist-blocking.spec.ts`:

```text
Error: apiRequestContext.post: socket hang up at
tests/security-enforcement/zzz-admin-whitelist-blocking.spec.ts:126:21
```

The test POSTs to [the admin reset endpoint][admin-reset], but the test
container cannot reach the admin API endpoint. This blocks the emergency
reset and fails the test.

## Likely cause

The admin backend at [http://localhost:2020][admin-base] is not running or
not reachable from the test runner container.

## Recommended fixes

- Ensure the admin backend is running and accessible from the test runner.
- Confirm the workflow starts the required service and listens on port 2020.
- If using Docker Compose, ensure the test container can reach the admin API
  container (use `depends_on` and compatible networking).
- If the endpoint should be served by the app under test, verify environment
  variables and config expose the admin API on the correct port.

## Optional code adjustment

If Playwright must target a non-default admin endpoint, read it from an
environment variable such as `CHARON_ADMIN_API_URL`.

## Resolution

Fixed by ensuring proper Docker Compose networking configuration and verifying admin backend service availability before test execution. Tests now properly wait for service readiness.

[admin-reset]: http://localhost:2020/emergency/security-reset
[admin-base]: http://localhost:2020
