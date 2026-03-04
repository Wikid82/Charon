## E2E Security Enforcement Failures Remediation Plan (2 Remaining)

**Context**
- Branch: `feature/beta-release`
- Source: [docs/reports/qa_report.md](../reports/qa_report.md)
- Failures: `/api/v1/users` setup socket hang up (Security Dashboard navigation), Emergency token baseline blocking (Test 1)

## Phase 1 – Analyze (Root Cause Mapping)

### Failure A: `/api/v1/users` setup socket hang up (Security Dashboard navigation)
**Symptoms**
- `apiRequestContext.post` socket hang up during test setup user creation in:
  - `tests/security/security-dashboard.spec.ts` (navigation suite)

**Likely Backend Cause**
- Test setup creates an admin user via `POST /api/v1/users`, which is routed through Cerberus middleware before auth.
- If ACL is enabled and the test runner IP is not in `security.admin_whitelist`, Cerberus will block all requests when no active ACLs exist.
- This block can present as a socket hang up when the proxy closes the connection before Playwright reads the response.

**Backend Evidence**
- Cerberus middleware executes on all `/api/v1/*` routes: [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go)
  - `api.Use(cerb.Middleware())` and `protected.POST("/users", userHandler.CreateUser)`
- ACL default-deny behavior and whitelist bypass: [backend/internal/cerberus/cerberus.go](../../backend/internal/cerberus/cerberus.go)
  - `Cerberus.Middleware` and `isAdminWhitelisted`
- User creation handler expects admin role after auth: [backend/internal/api/handlers/user_handler.go](../../backend/internal/api/handlers/user_handler.go)
  - `UserHandler.CreateUser`

**Fix Options (Backend)**
1. Ensure ACL cannot block authenticated admin setup calls by moving Cerberus after auth for protected routes (so role can be evaluated).
2. Add an explicit Cerberus bypass for `/api/v1/users` setup in test/dev mode when the request has a valid admin session.
3. Require at least one allow/deny list entry before enabling ACL, and return a clear 4xx error instead of terminating the connection.

### Failure B: Emergency token baseline not blocked (Test 1)
**Symptoms**
- Expected 403 from `/api/v1/security/status`, received 200 in:
  - `tests/security-enforcement/emergency-token.spec.ts` (Test 1)

**Likely Backend Cause**
- ACL is enabled via `/api/v1/settings`, but Cerberus treats the request IP as whitelisted (e.g., `127.0.0.1/32`) and skips ACL enforcement.
- The whitelist is stored in `SecurityConfig` and can persist from prior tests, causing ACL bypass for authenticated requests even without the emergency token.

**Backend Evidence**
- Admin whitelist bypass check: [backend/internal/cerberus/cerberus.go](../../backend/internal/cerberus/cerberus.go)
  - `isAdminWhitelisted`
- Security config persistence: [backend/internal/models/security_config.go](../../backend/internal/models/security_config.go)
- ACL enablement via settings: [backend/internal/api/handlers/settings_handler.go](../../backend/internal/api/handlers/settings_handler.go)
  - `SettingsHandler.UpdateSetting` auto-enables `feature.cerberus.enabled`

**Fix Options (Backend)**
1. Make ACL bypass conditional on authenticated admin context by applying Cerberus after auth on protected routes.
2. Clear or override `security.admin_whitelist` when enabling ACL in test runs where the baseline must be blocked.
3. Add a dedicated ACL enforcement endpoint or status check that is not exempted by admin whitelist.

## Phase 2 – Focused Remediation Plan (No Code Changes Yet)

### Plan A: Diagnose `/api/v1/users` socket hang up
1. Confirm ACL and admin whitelist values immediately before test setup user creation.
2. Check server logs for Cerberus ACL blocks or upstream connection resets during `POST /api/v1/users`.
3. Validate that the request is authenticated and that Cerberus is not terminating the request before auth runs.

**Acceptance Criteria**
- `POST /api/v1/users` consistently returns a 2xx or a structured 4xx, not a socket hang up.

### Plan B: Emergency token baseline enforcement
1. Verify `security.admin_whitelist` contents before Test 1; ensure the test IP is not whitelisted.
2. Confirm `security.acl.enabled` and `feature.cerberus.enabled` are both `true` after the setup PATCH.
3. Re-run the baseline `/api/v1/security/status` request and verify 403 before applying the emergency token.

**Acceptance Criteria**
- Baseline `/api/v1/security/status` returns 403 when ACL + Cerberus are enabled.
- Emergency token bypass returns 200 for the same endpoint.

## Phase 3 – Validation Plan

1. Re-run Chromium E2E suite.
2. Verify the two failing tests pass.
3. Capture updated results and include status evidence in QA report.

## Risks & Notes

- If `security.admin_whitelist` persists across suites, ACL baseline assertions will be bypassed.
- If Cerberus runs before auth, ACL cannot distinguish authenticated admin setup calls from unauthenticated setup calls.

## Next Steps

- Execute the focused remediation steps above.
- Re-run E2E tests and update [docs/reports/qa_report.md](../reports/qa_report.md).

**Status**: SUSPENDED - Supersededby critical production bug (Settings Query ID Leakage)
**Archive Date**: 2026-01-28
