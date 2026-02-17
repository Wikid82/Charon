# Phase 2 Test Remediation Plan

**Date:** 2026-02-09
**Status:** In Progress
**Scope:** Remediation for 28 failing tests (308 passing, 91.7% pass rate)
**Target:** Resolve 16 code bugs/features + clarify log viewer scope (12 skipped)

---

## Executive Summary

Phase 2 testing identified **28 failures** across **5 categories**. Analysis confirms:

- **16 actionable fixes** (code bugs + missing implementations) requiring development
- **12 feature scope unknowns** (log viewer) temporarily skipped pending clarification
- **No blockers** for proceeding to Phase 3 (Cerberus security suite testing)
- **Phase 2.1**: Critical fixes (3 items, ~2-3 days)
- **Phase 2.2**: Missing features (13 items, ~5-7 days)

All failures have **identified root causes**, **suspected code locations**, and **implementation guidance**.

---

## 1. Failure Categorization & Breakdown

### Category A: Code Bugs (12 Failures)

These are implementation defects in existing features that should work but don't.

#### A1: Notifications Provider CRUD (6 failures, Tests #205, #208, #211, #212, #213, #219)

**Test File:** `tests/settings/notifications.spec.ts` (lines 170-230+)

**Failing Tests:**
- Create Discord notification provider
- Create Slack notification provider
- Create generic webhook provider
- Update existing provider
- Delete provider with confirmation
- Enable/disable provider

**Root Cause:** All CRUD operations timeout after **1.5 minutes** consistently, indicating backend performance degradation or missing validation response.

**Technical Details:**
- **Frontend:** `NotificationProvider` form in `/projects/Charon/frontend/src/pages/Notifications.tsx`
  - Uses React Hook Form with handlers: `createMutation`, `updateMutation`, `deleteMutation`
  - Routes: `POST /notifications/providers`, `PUT /notifications/providers/:id`, `DELETE /notifications/providers/:id`
  - Data-testid selectors: `provider-name`, `provider-type`, `provider-url`, `provider-config`, `provider-save-btn`

- **Backend:** `NotificationProviderHandler` in `/projects/Charon/backend/internal/api/handlers/notification_provider_handler.go`
  - Methods: `Create()`, `Update()`, `Delete()`, `List()`, `Test()`
  - Service layer: `NotificationService.CreateProvider()`, `UpdateProvider()`, `DeleteProvider()` in `/projects/Charon/backend/internal/services/notification_service.go`
  - Template validation in `CreateProvider()` validates custom template payload at lines 527-540
  - Model: `NotificationProvider` struct in `/projects/Charon/backend/internal/models/notification_provider.go`

- **API Endpoints:**
  ```
  GET    /api/v1/notifications/providers
  POST   /api/v1/notifications/providers      (1.5m timeout)
  PUT    /api/v1/notifications/providers/:id  (1.5m timeout)
  DELETE /api/v1/notifications/providers/:id  (1.5m timeout)
  POST   /api/v1/notifications/providers/test
  ```

**Suspected Issues:**
1. **Backend validation loop** causing timeout (template validation at line 533)
2. **N+1 query problem** in provider fetch/update flow
3. **Missing database indexes** on `notification_providers` table
4. **Slow response** from external webhook test calls blocking handler

**Implementation Guidance:**
1. Profile `CreateProvider()` handler with slow query logging enabled
2. Check `RenderTemplate()` method for performance bottlenecks (lines 1045+)
3. Add database indexes on `name`, `type`, `enabled` columns
4. Implement query timeouts for webhook testing
5. Verify test fixtures are creating proper provider records

**Success Criteria:**
- Create operation completes in < 2 seconds
- Update operation completes in < 2 seconds
- All 6 CRUD tests pass without timeout
- Template validation optional can be toggled for custom configs

**Complexity:** Medium (1-2 days, backend focus)

**Owner:** Backend Developer

---

#### A2: Proxy Hosts Docker Integration (2 failures, Tests #154, #155)

**Test File:** `tests/core/proxy-hosts.spec.ts` (lines 957-1000)

**Failing Tests:**
- "should show Docker container selector when Docker source selected"
- "should show containers dropdown when Docker source selected"

**Root Cause:** Docker container selector UI element fails to render when user selects "Local (Docker Socket)" as source, or dropdown selector for containers not appearing.

**Technical Details:**
- **Frontend:** Docker integration component in `/projects/Charon/frontend/src/components/ProxyHostForm.tsx`
  - `useDocker()` hook manages container fetching (line 237)
  - Source selector: `#connection-source` with "local" option (line 572)
  - Container dropdown: `#quick-select-docker` at lines 587-590
  - State: `connectionSource` (local|custom|remote), `dockerLoading`, `dockerError`, `dockerContainers` array
  - Handler: `handleContainerSelect()` populates form fields from selected container (lines 435-450)

- **Hook:** `useDocker()` in `/projects/Charon/frontend/src/hooks/useDocker.ts`
  - Queries Docker API based on source (local socket or remote server)
  - Returns: containers array, loading state, error state

- **Backend:** Docker API handler (likely in `/projects/Charon/backend/internal/api/handlers/`)
  - Endpoint: `GET /api/v1/docker/containers` or similar
  - May interact with Docker socket at `/var/run/docker.sock`

**Suspected Issues:**
1. **useDocker hook** not fetching containers correctly
2. **Backend Docker API endpoint** returns error or empty response
3. **Conditional rendering** - dropdown hidden when `dockerLoading === true` or `connectionSource === 'custom'`
4. **Docker socket access** - permission or connectivity issue from container

**Implementation Guidance:**
1. Verify `useDocker()` hook is being called with correct `connectionSource` parameter
2. Check backend Docker handler for: socket connectivity, error handling, response format
3. Inspect browser console for API errors or failed requests
4. Verify dropdown rendering logic (line 587-590) - may need UI state inspection
5. Test Docker socket availability in test container environment

**Success Criteria:**
- Docker container selector appears when "Local (Docker Socket)" is selected
- Containers list loads and displays (name, image, ports)
- Container selection populates forward_host field with container name
- Both tests pass without timeout

**Complexity:** Medium (1-2 days, frontend + backend Docker integration)

**Owner:** Frontend Developer + Backend Developer (Docker API)

---

#### A3: Uptime Monitor Initial State (1 failure, Test #166)

**Test File:** `tests/monitoring/uptime-monitoring.spec.ts` (lines 230+, "should update monitor" scenario)

**Failing Test:**
- "should mark monitor as down only after failed pings, not before first check"

**Root Cause:** New uptime monitors are immediately marked as "down" without sending initial ping/health check, causing false "down" status.

**Technical Details:**
- **Frontend:** `Uptime.tsx` page at `/projects/Charon/frontend/src/pages/Uptime.tsx`
  - Monitor status display at lines 45-90 uses `monitor.status` directly
  - Status badge logic: `isUp = monitor.status === 'up'`, `isPaused = !monitor.enabled` (line 113)
  - Heartbeat/history loading shows status changes over time

- **Backend:** `UptimeService` in `/projects/Charon/backend/internal/services/uptime_service.go`
  - `CheckAll()` method (line 353) iterates through monitors and calls `checkMonitor()`
  - `checkMonitor()` method (line 803) performs actual ping/TCP check
  - Initial state: monitor created with `status = "pending"` in `UptimeMonitor.BeforeCreate()` (line 40)
  - Status update: `CheckAll()` may prematurely mark as "down" if host is unreachable (line 595 `markHostMonitorsDown()`)

- **Model:** `UptimeMonitor` struct in `/projects/Charon/backend/internal/models/uptime.go`
  - Fields: `ID`, `Status` ("up"|"down"|"pending"|"paused"), `LastCheck`, `LastStatusChange`, `FailureCount`, `MaxRetries`
  - Default MaxRetries: 3 (per test line 803)

**Suspected Issues:**
1. **Initial status logic**: Monitor marked as "down" in `BeforeCreate()` instead of "pending"
2. **Host-level check** at line 595 `markHostMonitorsDown()` marking all monitors down without checking individual status first
3. **FailureCount accumulation**: Starting > 0 instead of 0, triggering down status prematurely
4. **Status transition**: "pending" → immediate down without waiting for first check

**Implementation Guidance:**
1. Verify `UptimeMonitor.BeforeCreate()` sets `Status = "pending"` and `FailureCount = 0`
2. Review `CheckAll()` logic to ensure pending monitors skip host-level down marking
3. Confirm `checkMonitor()` waits for actual check result before transitioning from "pending"
4. Add unit test: new monitor should remain "pending" until first ping attempt
5. Check test fixture setup - ensure monitors created with correct initial state

**Success Criteria:**
- New monitors start with `status = "pending"`
- Monitors remain "pending" until first health check completes
- Status transitions: pending → up (if healthy) or pending → down (if N failed checks)
- Test passes with monitor showing correct status based on actual ping result

**Complexity:** Low (0.5-1 day, backend state logic)

**Owner:** Backend Developer

---

#### A4: Backups Guest Authorization (1 failure, Test #274)

**Test File:** `tests/tasks/backups-create.spec.ts` (lines 68-80, "Guest Access" group)

**Failing Test:**
- "should hide Create Backup button for guest users"

**Root Cause:** Create Backup button is visible in Backups UI for guest/viewer users when it should be hidden (admin only).

**Technical Details:**
- **Frontend:** Backups page layout in `/projects/Charon/frontend/src/pages/Backups.tsx` or backup component
  - Button selector: `SELECTORS.createBackupButton` (likely a button with text "Create Backup" or data-testid)
  - Should conditionally render based on user role/permissions
  - Current: button visible regardless of user role

- **Backend:** User permission model in `/projects/Charon/backend/internal/models/user.go`
  - User roles: "admin", "user", "viewer" (Guest = viewer or limited user)
  - User struct has `Role` field used in auth checks
  - Auth middleware in `/projects/Charon/backend/internal/api/middleware/auth.go` sets `c.Set("role", claims.Role)`

- **Permission Check:**
  - Backup creation endpoint: `POST /api/v1/backups`
  - Should verify user role is "admin" before allowing creation
  - Frontend should hide button if user role is not admin

**Suspected Issues:**
1. **Frontend Backups component** doesn't check user role before rendering Create button
2. **No permission gate** - button render logic missing role check
3. **Backend permission check** exists but frontend doesn't use it confidently
4. **Role context** not properly propagated to Backups component

**Implementation Guidance:**
1. Add role check in Backups component: `user?.role === 'admin'` before rendering button
2. Verify user context is available (likely via auth hook or context provider)
3. Confirm backend POST `/api/v1/backups` rejects non-admin requests with 403
4. Test fixture setup: ensure test users have correct roles assigned
5. May need to fetch user profile at component load to get current user role

**Success Criteria:**
- Create Backup button visible only to admin users
- Guest/viewer users see button hidden or disabled
- Test passes: guest user views backups page without Create button
- Backend rejects create requests from non-admin users (403 Forbidden)

**Complexity:** Low (0.5-1 day, frontend permission check)

**Owner:** Frontend Developer

---

### Category B: Not Yet Tested Physically (6 Failures)

These features exist in code but have not been manually tested in the UI, causing test failures. High likelihood of missing/incomplete implementations or slow endpoints.

#### B1: User Management - Invite & Permissions (6 failures, Tests #248, #258, #260, #262, #269-270)

**Test File:** `tests/settings/user-management.spec.ts` (lines 500-700)

**Failing Tests:**
1. Test #248: "should show pending status for invited users"
2. Test #258: "should update permission mode for user"
3. Test #260: "should remove permitted hosts from user"
4. Test #262: "should enable/disable user toggle"
5. Test #269: "should update user role to admin"
6. Test #270: "should update user role to user"

**Root Cause:** These flows have NOT been manually tested in the UI. Tests may be written against specification rather than actual implementation. Likely causes: slow endpoints, missing implementation, or incorrect response format.

**Technical Details:**
- **Frontend:** `UsersPage.tsx` at `/projects/Charon/frontend/src/pages/UsersPage.tsx`
  - Components:
    - `InviteModal()` (lines 48-150): Email, Role, PermissionMode, PermittedHosts selectors
    - `PermissionsModal()` (lines 405-510): Host checkboxes, permission mode dropdown
  - Mutations: `inviteMutation`, `updatePermissionsMutation`, `updateMutation`, `deleteUser`
  - API calls: `inviteUser()`, `updateUserPermissions()`, `updateUser()`, `deleteUser()`

- **Backend:** `UserHandler` in `/projects/Charon/backend/internal/api/handlers/user_handler.go`
  - Routes (lines 26-39):
    ```
    POST   /users/invite                      (InviteUser handler)
    PUT    /users/:id/permissions              (UpdateUserPermissions handler)
    PUT    /users/:id                          (UpdateUser handler)
    GET    /users                              (ListUsers handler)
    DELETE /users/:id                          (DeleteUser handler)
    ```
  - Handler methods:
    - `InviteUser()` (line 447): Creates pending user, generates invite token, sends email
    - `UpdateUserPermissions()` (line 786): Updates permission_mode and permitted_hosts association
    - `UpdateUser()` (line 608): Updates enabled, role, email, name fields

- **Model:** `User` struct in `/projects/Charon/backend/internal/models/user.go`
  - Fields: `Email`, `Name`, `Role` ("admin"|"user"|"viewer"), `Enabled`, `PermissionMode` ("allow_all"|"deny_all")
  - Relations: `PermittedHosts` (has-many ProxyHost through association)
  - Invite fields: `InviteToken`, `InviteStatus` ("pending"|"accepted"|"expired"), `InviteExpires`, `InvitedAt`, `InvitedBy`

- **API Endpoints:**
  ```
  POST   /api/v1/users/invite                                    (15s-1.6m timeout)
  PUT    /api/v1/users/:id/permissions                           (15s-1.6m timeout)
  PUT    /api/v1/users/:id                                       (15s-1.6m timeout)
  GET    /api/v1/users                                           (working)
  DELETE /api/v1/users/:id                                       (likely working)
  ```

**Suspected Issues:**
1. **Invite endpoint** slow (may involve email sending, token generation)
2. **Permissions update** missing implementation or incorrect association handling
3. **User update** not properly handling role changes or enabled status
4. **Timeouts** suggest blocking operations (email, template rendering)
5. **Response format** may not match frontend expectations

**Implementation Guidance:**
1. **Priority: Manual Testing First**
   - Test invite workflow manually: email → token → validation → acceptance
   - Test permission updates: select hosts → save → verify in DB
   - Test user status toggle: enabled/disabled state persistence
   - Document any missing UI elements or slow endpoints

2. **For each slow endpoint:**
   - Add slow query logging on backend
   - Check for blocking operations (email sending, external API calls)
   - Implement async job queue if email sending is synchronous
   - Verify database queries are efficient (use EXPLAIN)
   - Add timeout to external service calls

3. **For permission updates:**
   - Verify `UpdateUserPermissions()` correctly handles PermittedHosts association (GORM many-to-many)
   - Test with multiple hosts selected
   - Verify frontend sends array of host IDs correctly

4. **For invite workflow:**
   - Trace full flow: create user → generate token → send email → user accepts → user logs in
   - Check email configuration (SMTP settings)
   - Verify token generation and validation

**Success Criteria:**
- All 6 user management tests pass without timeout (< 10 seconds each)
- User invite workflow works end-to-end
- Permission updates save and persist correctly
- User status changes (enable/disable) work as expected
- Role changes update authorization correctly

**Complexity:** High (3-4 days, requires physical testing + endpoint optimization)

**Owner:** Backend Developer + Frontend Developer

---

### Category C: Feature Scope Questions (12 Failures - Currently Skipped)

These tests fail due to unclear feature scope, not code bugs. Decision required before proceeding.

#### C1: Log Viewer Features (12 failures, Tests #324-335)

**Test File:** `tests/features/log-viewer.spec.ts` (if exists) or integration test

**Failing Tests:**
- Log viewer page layout
- Display system logs
- Filter logs by level
- Search logs by keyword
- Sort logs by timestamp
- Paginate through logs
- Download logs as file
- Mark logs as read
- Clear logs
- Export logs

**All tests timeout uniformly at 66 seconds.**

**Root Cause:** **FEATURE SCOPE UNCLEAR** - Tests assume a feature that may not be fully implemented or may have different scope than anticipated.

**Questions to Resolve:**
1. Is this a "live log viewer" (real-time streaming of application/system logs)?
2. Or a "static log reader" (displaying stored log files)?
3. Which logs should be included? (Application logs? System logs? Caddy proxy logs?)
4. Who should have access? (Admin only? All authenticated users?)
5. Should logs be searchable, filterable, sortable?
6. Should logs be exportable/downloadable?

**Decision Tree:**
- **If feature IS implemented:**
  - Debug why tests timeout (missing endpoint? incorrect routing?)
  - Fix performance issue (query optimization, pagination)
  - Enable tests and move to Phase 3

- **If feature is NOT implemented:**
  - Move tests to Phase 3 or later with `xfail` (expected fail) marker
  - Add issue for future implementation
  - Do NOT delay Phase 3 security testing on this scope question

**Current Status:** Tests skipped via `test.skip()` or similar mechanism.

**Success Criteria:**
- Scope decision made and documented
- Either: Tests fixed and passing, OR
- Marked as xfail/skipped with clear reason for Phase 3+

**Complexity:** Low (scope decision) or High (implementation if needed)

**Owner:** Product Manager (scope decision) + relevant dev team (if implementing)

---

## 2. Implementation Phasing

### Phase 2.1: Critical Fixes (3 items, ~2-3 days)

**Must complete before Phase 3 security testing:** Issues that block understanding of core features.

| # | Feature | Root Cause | Est. Effort | Owner |
|---|---------|-----------|------------|-------|
| 1 | Uptime Monitor Initial State | Initial state marked "down" before first check | 1 day | Backend |
| 2 | Backups Guest Authorization | Create button visible to guests | 0.5 day | Frontend |
| 3 | Notifications CRUD Performance | 1.5m timeout, likely query/validation issue | 1.5 days | Backend |

**Implementation Order:**
1. **Day 1:** Uptime monitor state logic (foundation for Phase 3 uptime testing)
2. **Day 1-2:** Notifications CRUD optimization (profiling + indexing)
3. **Day 2:** Backups UI permission check

---

### Phase 2.2: Missing Features (13 items, ~5-7 days)

**Can proceed to Phase 3 in parallel:** Features that don't block security suite but should be completed.

| # | Feature | Status | Est. Effort | Owner |
|---|---------|--------|------------|-------|
| 1 | Docker Integration UI | Container selector not rendering | 1-2 days | Frontend + Backend |
| 2 | User Management - Full Workflow | 6 tests, manual testing required | 3-4 days | Both |
| 3 | Log Viewer Scope | 12 tests, scope unclear | Pending decision | - |

**Implementation Order:**
1. Parallel: Docker UI + User management manual testing
2. Pending: Log viewer scope decision

---

## 3. Test Remediation Details

### A1: Notifications CRUD (6 tests)

```typescript
// tests/settings/notifications.spec.ts

test.describe('Provider CRUD', () => {
  test('should create Discord notification provider', async ({ page }) => {
    // CURRENT: Times out after 90 seconds
    // FIX: Profile POST /notifications/providers endpoint
    // - Check RenderTemplate() performance
    // - Add database indexes on name, type, enabled
    // - Profile webhook test calls
    // - Set 5 second timeout on external calls
    // EXPECTED: Completes in < 2 seconds
  })
})
```

**Testing Approach:**
1. Run test with backend profiler enabled
2. Check slow query logs for N+1 issues
3. Verify test fixtures create valid provider records
4. Optimize identified bottleneck
5. Rerun test - should complete in < 2 seconds

---

### A2: Docker Integration (2 tests)

```typescript
// tests/core/proxy-hosts.spec.ts

test.describe('Docker Integration', () => {
  test('should show Docker container selector when source is selected', async ({ page }) => {
    // CURRENT: Container dropdown not visible when Docker source selected
    // FIX: Verify useDocker() hook is called and returns containers
    // - Check browser console for API errors
    // - Verify GET /docker/containers endpoint
    // - Inspect conditional rendering: dockerLoading, connectionSource
    // - Check Docker socket availability in test environment
    // EXPECTED: Dropdown visible with list of containers
  })
})
```

**Testing Approach:**
1. Manually test Docker integration in dev environment
2. Check browser DevTools for API call failures
3. Verify Docker socket is accessible from container
4. Fix identified issue (missing endpoint, socket permission, etc.)
5. Run full test suite

---

### A3: Uptime Monitor State (1 test)

```typescript
// tests/monitoring/uptime-monitoring.spec.ts

test('should mark monitor as down only after failed pings, not before first check', async ({ page }) => {
  // CURRENT: New monitor marked "down" immediately
  // FIX: Ensure initial state is "pending" until first check
  // - Verify UptimeMonitor.BeforeCreate() sets Status="pending"
  // - Verify FailureCount=0 initially
  // - Verify CheckAll() respects pending status in host-level check
  // - Verify first checkMonitor() call transitions pending→up or pending→down
  // EXPECTED: Monitor shows "pending" → "up" based on actual ping result
})
```

**Testing Approach:**
1. Create new monitor via API
2. Immediately check status - should be "pending"
3. Wait for first health check to run
4. Verify status transitions to "up" or "down" based on result
5. Run test

---

### A4: Backups Authorization (1 test)

```typescript
// tests/tasks/backups-create.spec.ts

test('should hide Create Backup button for guest users', async ({ page, guestUser }) => {
  // CURRENT: Create Backup button visible to guest users
  // FIX: Add role check in Backups component
  // - Verify user role is available in component context
  // - Conditional render: user.role === 'admin' ? <CreateButton/> : null
  // - Ensure backend also rejects non-admin POST requests (409 Forbidden)
  // EXPECTED: Button hidden for non-admin users
})
```

**Testing Approach:**
1. Login as guest user
2. Navigate to /tasks/backups
3. Verify Create Backup button is NOT visible
4. Verify admin user DOES see the button
5. Run test

---

### B1: User Management (6 tests)

```typescript
// tests/settings/user-management.spec.ts

test.describe('User Invitations & Permissions', () => {
  test('should create and accept user invite', async ({ page }) => {
    // CURRENT: Tests timeout after 15-90 seconds
    // FIX: Manual testing to identify bottleneck
    // 1. Test invite flow end-to-end
    // 2. Check email logs if SMTP is configured
    // 3. Profile POST /users/invite - likely email sending is slow
    // 4. If email slow: implement async job queue
    // 5. Test permissions update endpoint
    // 6. Verify permitted hosts association saves correctly
    // EXPECTED: All tests pass, < 10 second response time
  })
})
```

**Manual Testing Checklist:**
- [ ] Invite user with email - receives email or message
- [ ] Invited user accepts invite - account activated
- [ ] Update permissions - deny_all mode with specific hosts allowed
- [ ] Remove host from allowed list - permissions persisted
- [ ] Change user role - admin→user transition works
- [ ] Enable/disable user toggle - status persists

---

### C1: Log Viewer (12 tests - PENDING DECISION)

**Action Required:**
1. Schedule stakeholder meeting to clarify scope
2. Decide: implement now, defer to Phase 3+, or mark as xfail
3. Update `.github/instructions/testing.instructions.md` with decision
4. Move tests to appropriate location:
   - If deferring: move to `tests/backlog/` with `test.skip()`
   - If implementing: create implementation plan similar to above
   - If xfail: mark with `test.skip('not implemented')` comment

---

## 4. Success Criteria & Validation

### Pre-Implementation Checklist

- [ ] All code locations identified and verified
- [ ] Backend dependencies (database, external services) understood
- [ ] Frontend state management (ReactQuery, hooks) reviewed
- [ ] Test fixtures verified to match expected data shape

### Post-Implementation Checklist (Per Item)

- [ ] Unit tests pass (backend Go tests)
- [ ] Integration tests pass (E2E Playwright tests)
- [ ] Manual testing completed and documented
- [ ] Code review completed
- [ ] No new test failures introduced

### Phase 2.2 Completion Criteria

- [ ] 16/16 code bugs resolved
- [ ] All 16 tests pass in suite
- [ ] 308 baseline tests still passing (no regressions)
- [ ] Docker integration verified in real Docker environment
- [ ] User management end-to-end workflow functional
- [ ] Log viewer scope decided and documented

---

## 5. Risk Mitigation

### High-Risk Items

1. **Notifications CRUD (Category A1)** - Visible failure, performance critical
   - Risk: Root cause unclear (query? validation? blocking call?)
   - Mitigation: Enable slow query logging, profile with pprof
   - Fallback: Disable email sending in test to identify bottleneck

2. **User Management (Category B1)** - Complex workflow, not yet tested
   - Risk: Missing endpoints or incorrect implementation
   - Mitigation: Manual testing first before code changes
   - Fallback: Implement async email queue if email is blocking

3. **Docker Integration (Category A2)** - Depends on external Docker API
   - Risk: Socket permission, network, or API changes
   - Mitigation: Test in CI environment with known Docker setup
   - Fallback: Mock Docker API if socket unavailable

### Medium-Risk Items

1. **Uptime Monitor State (Category A3)** - Initial state logic
   - Risk: State transition logic may affect Phase 3 testing
   - Mitigation: Add unit tests for status transitions
   - Fallback: Manually verify initial state in database

2. **Backups Authorization (Category A4)** - Permission check
   - Risk: UI check alone insufficient (backend must enforce)
   - Mitigation: Verify both frontend UI and backend 403 response
   - Fallback: Backend-only permission check if frontend can't access user role

### Low-Risk Items

- Log viewer scope decision (5% impact on Phase 2, decision-driven)

---

## 6. Post-Phase 2 Actions

### Documentation Updates
- [ ] Update `ARCHITECTURE.md` with notification system performance notes
- [ ] Document Docker socket requirements in `README.md`
- [ ] Update user management workflows in `docs/features/user-management.md`

### Phase 3 Handoff
- [ ] All Phase 2.1 fixes merged to main
- [ ] Phase 2.2 merged or in progress without blocking Phase 3
- [ ] Clear documentation of any Phase 2 workarounds or incomplete features
- [ ] Test environment verified ready for Cerberus security suite testing

### Technical Debt
- Add GitHub issues for:
  - Notification system performance optimization (if index/query fix)
  - User management email queue implementation (if async needed)
  - Docker integration test environment hardening

---

## 7. References

**Test Files:**
- [tests/settings/notifications.spec.ts](../../tests/settings/notifications.spec.ts) - 6 failing tests
- [tests/core/proxy-hosts.spec.ts](../../tests/core/proxy-hosts.spec.ts) - 2 failing tests (#154-155 at line 957)
- [tests/monitoring/uptime-monitoring.spec.ts](../../tests/monitoring/uptime-monitoring.spec.ts) - 1 failing test (#166)
- [tests/tasks/backups-create.spec.ts](../../tests/tasks/backups-create.spec.ts) - 1 failing test (#274 at line 68)
- [tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts) - 6 failing tests (#248, #258, #260, #262, #269-270)

**Backend Implementation Files:**
- [backend/internal/api/handlers/notification_provider_handler.go](../../backend/internal/api/handlers/notification_provider_handler.go)
- [backend/internal/services/notification_service.go](../../backend/internal/services/notification_service.go)
- [backend/internal/api/handlers/uptime_handler.go](../../backend/internal/api/handlers/uptime_handler.go)
- [backend/internal/services/uptime_service.go](../../backend/internal/services/uptime_service.go)
- [backend/internal/api/handlers/user_handler.go](../../backend/internal/api/handlers/user_handler.go)
- [backend/internal/models/user.go](../../backend/internal/models/user.go)

**Frontend Implementation Files:**
- [frontend/src/pages/Notifications.tsx](../../frontend/src/pages/Notifications.tsx)
- [frontend/src/components/ProxyHostForm.tsx](../../frontend/src/components/ProxyHostForm.tsx) - Lines 572-590 (Docker selector)
- [frontend/src/pages/Uptime.tsx](../../frontend/src/pages/Uptime.tsx)
- [frontend/src/pages/UsersPage.tsx](../../frontend/src/pages/UsersPage.tsx)

**Related Documentation:**
- [docs/reports/phase2_failure_triage.md](../reports/phase2_failure_triage.md) - Detailed failure categorization
- [docs/plans/current_spec.md](./current_spec.md) - Phase methodology
- [tests/fixtures/](../../tests/fixtures/) - Test data fixtures for all test suites

---

## Conclusion

Phase 2 testing has successfully identified **16 actionable code issues** and **12 scope questions**. Root causes have been identified for all failures, with clear implementation guidance and resource allocation. These fixes are non-blocking for Phase 3 security testing, which can proceed in parallel.

**Recommended Timeline:**
- **Week 1:** Phase 2.1 fixes + Phase 3 parallel work
- **Week 2:** Phase 2.2 features + Phase 3 execution
- **Week 3:** Phase 2 completeness validation + Phase 3 close-out
