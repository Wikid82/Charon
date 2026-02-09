# E2E Test Remediation Checklist

**Status**: Active
**Plan Reference**: [docs/plans/current_spec.md](docs/plans/current_spec.md)
**Last Updated**: 2026-02-09

---

## 📋 Phase 1: Foundation & Test Harness Reliability

**Objective**: Ensure the shared test harness (global setup, auth, emergency server) is stable
**Estimated Runtime**: 2-4 minutes
**Status**: ✅ PASSED

### Setup
- [x] **docker-rebuild-e2e**: `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
  - Ensures container has latest code and env vars (`CHARON_EMERGENCY_TOKEN`, encryption key)
  - **Expected**: Container healthy, port 8080 responsive, port 2020 available
  - **Status**: ✅ Container rebuilt and ready

### Execution
- [x] **Run Phase 1 tests**:
  ```bash
  cd /projects/Charon
  npx playwright test tests/global-setup.ts tests/auth.setup.ts --project=firefox
  ```
  - **Expected**: Both tests pass without re-auth flakes
  - **Result**: ✅ **PASSED** (1 test in 5.2s)
  - **Errors found**: None

### Validation
- [x] Storage state (`tests/.auth/*.json`) created successfully
  - ✅ Auth state saved to `/projects/Charon/playwright/.auth/user.json`
- [x] Emergency token validated (check logs for "Emergency token OK")
  - ✅ Token length: 64 chars (valid), format: Valid hexadecimal
- [x] Security reset executed (check logs for "Security teardown complete")
  - ✅ Emergency reset successful [22ms]
  - ✅ Security reset complete with 526ms propagation

### Blocking Issues
- [x] **None** - Phase 1 foundational tests all passing

**Issues Encountered**:
- None

### Port Connectivity Summary
- [x] Caddy admin API (port 2019): ✅ Healthy
- [x] Emergency server (port 2020): ✅ Healthy
- [x] Application UI (port 8080): ✅ Accessible

---

## 📋 Phase 2: Core UI, Settings, Tasks, Monitoring

**Objective**: Remediate highest-traffic user journeys
**Estimated Runtime**: 25-40 minutes
**Status**: ❌ FAILED

**Note:** Verified Phase 2 directories for misfiled security-dependent tests — no remaining ACL/CrowdSec/WAF tests were found in `tests/core`, `tests/settings`, `tests/tasks` or `tests/monitoring`. CrowdSec/ACL-specific tests live in the `tests/security` and `tests/security-enforcement` suites as intended. The Caddy import tests remain in Phase 2 (they do not require security to be enabled).

### Sub-Phase 2A: Core UI (Navigation, Dashboard, CRUD)
- [x] **Run tests**:
  ```bash
  npx playwright test tests/core --project=firefox
  ```
  - **Expected**: All core CRUD and navigation pass
  - **Result**: ❌ Fail (9 passed, 2 interrupted, 187 did not run; total 198; exit code 130)
  - **Comparison**: Previous 2 failed → Now 2 interrupted (187 did not run)
  - **Errors found**:
    ```
    1) [firefox] › tests/core/access-lists-crud.spec.ts:261:5 › Access Lists - CRUD Operations › Create Access List › should add client IP addresses
       Error: page.goto: Test ended.
       Call log:
         - navigating to "http://localhost:5173/access-lists", waiting until "load"

    2) [firefox] › tests/core/access-lists-crud.spec.ts:217:5 › Access Lists - CRUD Operations › Create Access List › should create ACL with name only (IP whitelist)
       Error: Test was interrupted.
    ```

**Issue Log for Phase 2A**:
1. **Issue**: Access list creation tests interrupted by unexpected page close
  **File**: [tests/core/access-lists-crud.spec.ts](tests/core/access-lists-crud.spec.ts)
  **Root Cause**: Test run interrupted during navigation (page/context ended)
  **Fix Applied**: None (per instructions)
  **Re-test Result**: ❌

---

### Sub-Phase 2B: Settings (System, Account, Notifications, Encryption, Users)
- [x] **Run tests**:
  ```bash
  npx playwright test tests/settings --project=firefox
  ```
  - **Expected**: All settings flows pass
  - **Result**: ❌ Fail (1 passed, 2 interrupted, 129 did not run; total 132; exit code 130)
  - **Comparison**: Previous 15 failed → Now 2 interrupted (129 did not run)
  - **Errors found**:
   ```
   1) [firefox] › tests/settings/account-settings.spec.ts:37:5 › Account Settings › Profile Management › should display user profile
     Error: page.goto: Test ended.
     Call log:
       - navigating to "http://localhost:5173/settings/account", waiting until "load"

   2) [firefox] › tests/settings/account-settings.spec.ts:63:5 › Account Settings › Profile Management › should update profile name
     Error: Test was interrupted.
   ```

**Issue Log for Phase 2B**:
1. **Issue**: Settings test run interrupted during account settings navigation
  **File**: [tests/settings/account-settings.spec.ts](tests/settings/account-settings.spec.ts)
  **Root Cause**: Test ended unexpectedly during `page.goto`
  **Fix Applied**: None (per instructions)
  **Re-test Result**: ❌

---

### Sub-Phase 2C: Tasks, Monitoring, Utilities
- [x] **Run tests**:
  ```bash
  npx playwright test tests/tasks --project=firefox
  npx playwright test tests/monitoring --project=firefox
  npx playwright test tests/utils/wait-helpers.spec.ts --project=firefox
  ```
  - **Expected**: All task/monitoring flows and utilities pass
  - **Result**: ❌ Fail
    - **Tasks**: 1 passed, 2 interrupted, 94 did not run; total 97; exit code 130
    - **Monitoring**: 1 passed, 2 interrupted, 44 did not run; total 47; exit code 130
    - **Wait-helpers**: 0 passed, 0 failed, 22 did not run; total 22; exit code 130
    - **Comparison**:
     - Tasks: Previous 16 failed → Now 2 interrupted (94 did not run)
     - Monitoring: Previous 20 failed → Now 2 interrupted (44 did not run)
     - Wait-helpers: Previous 1 failed → Now 0 failed (22 did not run)
  - **Errors found**:
  ```
    Tasks
    1) [firefox] › tests/tasks/backups-create.spec.ts:58:5 › Backups Page - Creation and List › Page Layout › should show Create Backup button for admin users
      Error: browserContext.close: Protocol error (Browser.removeBrowserContext)

    2) [firefox] › tests/tasks/backups-create.spec.ts:50:5 › Backups Page - Creation and List › Page Layout › should display backups page with correct heading
      Error: browserContext.newPage: Test ended.

    Monitoring
    1) [firefox] › tests/monitoring/real-time-logs.spec.ts:247:5 › Real-Time Logs Viewer › Page Layout › should display live logs viewer with correct heading
      Error: page.goto: Test ended.
      Call log:
       - navigating to "http://localhost:5173/", waiting until "load"

    2) [firefox] › tests/monitoring/real-time-logs.spec.ts:510:5 › Real-Time Logs Viewer › Filtering › should filter logs by search text
      Error: page.goto: Target page, context or browser has been closed

    Wait-helpers
    1) [firefox] › tests/utils/wait-helpers.spec.ts:284:5 › wait-helpers - Phase 2.1 Semantic Wait Functions › waitForNavigation › should wait for URL change with string match
      Error: Test run interrupted before executing tests (22 did not run).
  ```

**Issue Log for Phase 2C**:
1. **Issue**: Tasks suite interrupted due to browser context teardown error
  **File**: [tests/tasks/backups-create.spec.ts](tests/tasks/backups-create.spec.ts)
  **Root Cause**: `Browser.removeBrowserContext` protocol error during teardown
  **Fix Applied**: None (per instructions)
  **Re-test Result**: ❌
2. **Issue**: Monitoring suite interrupted by page/context closure during navigation
  **File**: [tests/monitoring/real-time-logs.spec.ts](tests/monitoring/real-time-logs.spec.ts)
  **Root Cause**: Page closed before navigation completed
  **Fix Applied**: None (per instructions)
  **Re-test Result**: ❌
3. **Issue**: Wait-helpers suite interrupted before executing tests
  **File**: [tests/utils/wait-helpers.spec.ts](tests/utils/wait-helpers.spec.ts)
  **Root Cause**: Test run interrupted before any assertions executed
  **Fix Applied**: None (per instructions)
  **Re-test Result**: ❌

---

## 📋 Phase 3: Security UI & Enforcement

**Objective**: Stabilize Cerberus UI and enforcement workflows
**Estimated Runtime**: 30-45 minutes
**Status**: ⏳ Not Started
**⚠️ CRITICAL**: Must use `--workers=1` for security-enforcement (see Phase 3B)

### Sub-Phase 3A: Security UI (Dashboard, WAF, Headers, Rate Limiting, CrowdSec, Audit Logs)
- [ ] **Run tests**:
  ```bash
  npx playwright test tests/security --project=firefox
  ```
  - **Expected**: All security UI toggles and pages load
  - **Result**: ✅ Pass / ❌ Fail
  - **Errors found** (if any):
    ```
    [Paste errors]
    ```

**Issue Log for Phase 3A**:
1. **Issue**: [Describe]
   **File**: [tests/security/...]
   **Root Cause**: [Analyze]
   **Fix Applied**: [Link]
   **Re-test Result**: ✅ / ❌

---

### Sub-Phase 3B: Security Enforcement (ACL, WAF, CrowdSec, Rate Limits, Emergency Token, Break-Glass)

⚠️ **SERIAL EXECUTION REQUIRED**: `--workers=1` (enforces zzz-prefixed ordering)

- [ ] **Run tests WITH SERIAL FLAG**:
  ```bash
  npx playwright test tests/security-enforcement --project=firefox --workers=1
  ```
  - **Expected**: All enforcement tests pass with zzz-prefixing order enforced
  - **Result**: ✅ Pass / ❌ Fail
  - **Errors found** (if any):
    ```
    [Paste errors]
    ```

**Critical Ordering Notes**:
- `zzz-admin-whitelist-blocking.spec.ts` MUST run last (before break-glass)
- `zzzz-break-glass-recovery.spec.ts` MUST finalize cleanup
- If tests fail due to ordering, verify `--workers=1` was used

**Issue Log for Phase 3B**:
1. **Issue**: [Describe]
   **File**: [tests/security-enforcement/...]
   **Root Cause**: [Analyze - including ordering if relevant]
   **Fix Applied**: [Link]
   **Re-test Result**: ✅ / ❌

---

## 📋 Phase 4: Integration, Browser-Specific, Debug (Optional)

**Objective**: Close cross-feature and browser-specific regressions
**Estimated Runtime**: 25-40 minutes
**Status**: ⏳ Not Started

### Sub-Phase 4A: Integration Workflows
- [ ] **Run tests**:
  ```bash
  npx playwright test tests/integration --project=firefox
  ```
  - **Expected**: Cross-feature workflows pass
  - **Result**: ✅ Pass / ❌ Fail
  - **Errors found** (if any):
    ```
    [Paste errors]
    ```

**Issue Log for Phase 4A**:
1. **Issue**: [Describe]
   **File**: [tests/integration/...]
   **Root Cause**: [Analyze]
   **Fix Applied**: [Link]
   **Re-test Result**: ✅ / ❌

---

### Sub-Phase 4B: Browser-Specific Regressions (Firefox & WebKit)
- [ ] **Run Firefox-specific tests**:
  ```bash
  npx playwright test tests/firefox-specific --project=firefox
  ```
  - **Expected**: Firefox import and flow regressions pass
  - **Result**: ✅ Pass / ❌ Fail
  - **Errors found** (if any):
    ```
    [Paste errors]
    ```

- [ ] **Run WebKit-specific tests**:
  ```bash
  npx playwright test tests/webkit-specific --project=webkit
  ```
  - **Expected**: WebKit import and flow regressions pass
  - **Result**: ✅ Pass / ❌ Fail
  - **Errors found** (if any):
    ```
    [Paste errors]
    ```

**Issue Log for Phase 4B**:
1. **Issue**: [Describe]
   **File**: [tests/firefox-specific/... or tests/webkit-specific/...]
   **Root Cause**: [Analyze - may be browser-specific]
   **Fix Applied**: [Link]
   **Re-test Result**: ✅ / ❌

---

### Sub-Phase 4C: Debug/POC & Gap Coverage (Optional)
- [ ] **Run debug diagnostics**:
  ```bash
  npx playwright test tests/debug --project=firefox
  npx playwright test tests/tasks/caddy-import-gaps.spec.ts --project=firefox
  npx playwright test tests/tasks/caddy-import-cross-browser.spec.ts --project=firefox
  npx playwright test tests/modal-dropdown-triage.spec.ts --project=firefox
  npx playwright test tests/proxy-host-dropdown-fix.spec.ts --project=firefox
  ```
  - **Expected**: Debug and gap-coverage tests pass (or are identified as low-priority)
  - **Result**: ✅ Pass / ❌ Fail / ⏭️ Skip (optional)
  - **Errors found** (if any):
    ```
    [Paste errors]
    ```

**Issue Log for Phase 4C**:
1. **Issue**: [Describe]
   **File**: [tests/debug/... or tests/tasks/...]
   **Root Cause**: [Analyze]
   **Fix Applied**: [Link]
   **Re-test Result**: ✅ / ❌

---

## 🎯 Summary & Sign-Off

### Overall Status
- **Phase 1**: ✅ PASSED
- **Phase 2**: ❌ FAILED
- **Phase 3**: ⏳ Not Started
- **Phase 4**: ⏳ Not Started

### Total Issues Found & Fixed
- **Phase 1**: 0 issues
- **Phase 2**: [X] issues (all fixed: ✅ / some pending: ❌)
- **Phase 3**: [X] issues (all fixed: ✅ / some pending: ❌)
- **Phase 4**: [X] issues (all fixed: ✅ / some pending: ❌)

### Root Causes Identified
1. [Issue type] - Occurred in [Phase] - Example: "Flaky WebSocket timeout in monitoring tests"
2. [Issue type] - Occurred in [Phase]
3. ...

### Fixes Applied (with Links)
1. [Fix description] - [Link to PR/commit]
2. [Fix description] - [Link to PR/commit]
3. ...

### Final Validation
- [ ] All phases complete (phases 1-3 required; phase 4 optional)
- [ ] All blocking issues resolved
- [ ] No new regressions introduced
- [ ] Ready for CI integration

---

## 🔗 References

- **Plan**: [docs/plans/current_spec.md](docs/plans/current_spec.md)
- **Quick Start**: See Quick Start section in plan
- **Emergency Server Docs**: Check tests/security-enforcement/emergency-server/
- **Port Requirements**: 8080 (UI/API), 2020 (Emergency Server), 2019 (Caddy Admin)
- **Critical Flag**: `--workers=1` for Phase 3B (security-enforcement)

---

## 📝 Notes

Use this space to document any additional context, blockers, or learnings:

```
Remaining failures (current rerun):
- Test infra interruptions: 8 interrupted tests, 476 did not run (Phase 2A/2B/2C)
- WebSocket/logs/import verification: not validated in this rerun due to early interruptions
```
