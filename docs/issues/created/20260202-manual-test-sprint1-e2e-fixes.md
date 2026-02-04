# Manual Test Plan: Sprint 1 E2E Test Timeout Fixes

**Created**: 2026-02-02
**Status**: Open
**Priority**: P1
**Assignee**: QA Team
**Sprint**: Sprint 1 Closure / Sprint 2 Week 1

---

## Objective

Manually validate Sprint 1 E2E test timeout fixes in production-like environment to ensure no regression when deployed.

---

## Test Environment

- **Browser(s)**: Chrome 131+, Firefox 133+, Safari 18+
- **OS**: macOS, Windows, Linux
- **Network**: Normal latency (no throttling)
- **Charon Version**: Development branch (Sprint 1 complete)

---

## Test Cases

### TC1: Feature Toggle Interactions

**Objective**: Verify feature toggles work without timeouts or blocking

**Steps**:
1. Navigate to Settings → System
2. Toggle "Cerberus Security" off
3. Wait for success toast
4. Toggle "Cerberus Security" back on
5. Wait for success toast
6. Repeat for "CrowdSec Console Enrollment"
7. Repeat for "Uptime Monitoring"

**Expected**:
- ✅ Toggles respond within 2 seconds
- ✅ No overlay blocking interactions
- ✅ Success toast appears after each toggle
- ✅ Settings persist after page refresh

**Pass Criteria**: All toggles work within 5 seconds with no errors

---

### TC2: Concurrent Toggle Operations

**Objective**: Verify multiple rapid toggles don't cause race conditions

**Steps**:
1. Navigate to Settings → System
2. Quickly toggle "Cerberus Security" on → off → on
3. Verify final state matches last toggle
4. Toggle "CrowdSec Console" and "Uptime" simultaneously (within 1 second)
5. Verify both toggles complete successfully

**Expected**:
- ✅ Final toggle state is correct
- ✅ No "propagation timeout" errors
- ✅ Both concurrent toggles succeed
- ✅ UI doesn't freeze or become unresponsive

**Pass Criteria**: All operations complete within 10 seconds

---

### TC3: Config Reload During Toggle

**Objective**: Verify config reload overlay doesn't permanently block tests

**Steps**:
1. Navigate to Proxy Hosts
2. Create a new proxy host (triggers config reload)
3. While config is reloading (overlay visible), immediately navigate to Settings → System
4. Attempt to toggle "Cerberus Security"

**Expected**:
- ✅ Overlay appears during config reload
- ✅ Toggle becomes interactive after overlay disappears (within 5 seconds)
- ✅ Toggle interaction succeeds
- ✅ No "intercepts pointer events" errors in browser console

**Pass Criteria**: Toggle succeeds within 10 seconds of overlay appearing

---

### TC4: Cross-Browser Feature Flag Consistency

**Objective**: Verify feature flags work identically across browsers

**Steps**:
1. Open Charon in Chrome
2. Toggle "Cerberus Security" off
3. Open Charon in Firefox (same account)
4. Verify "Cerberus Security" shows as off
5. Toggle "Uptime Monitoring" on in Firefox
6. Refresh Chrome tab
7. Verify "Uptime Monitoring" shows as on

**Expected**:
- ✅ State syncs across browsers within 3 seconds
- ✅ No discrepancies in toggle states
- ✅ Both browsers can modify settings

**Pass Criteria**: Settings sync across browsers consistently

---

### TC5: DNS Provider Form Fields (Firefox)

**Objective**: Verify DNS provider form fields are accessible in Firefox

**Steps**:
1. Open Charon in Firefox
2. Navigate to DNS → Providers
3. Click "Add Provider"
4. Select provider type "Webhook"
5. Verify "Create URL" field appears
6. Select provider type "RFC 2136"
7. Verify "DNS Server" field appears
8. Select provider type "Script"
9. Verify "Script Path/Command" field appears

**Expected**:
- ✅ All provider-specific fields appear within 2 seconds
- ✅ Fields are properly labeled
- ✅ Fields are keyboard accessible (Tab navigation works)

**Pass Criteria**: All fields appear and are accessible in Firefox

---

## Known Issues to Watch For

1. **Advanced Scenarios**: Edge case tests for 500 errors and concurrent operations may still have minor issues - these are Sprint 2 backlog items
2. **WebKit**: Some intermittent failures on WebKit (Safari) - acceptable, documented for Sprint 2
3. **DNS Provider Labels**: Label text/ID mismatches possible - deferred to Sprint 2

---

## Success Criteria

**PASS** if:
- All TC1-TC5 test cases pass
- No Critical (P0) bugs discovered
- Performance is acceptable (interactions <5 seconds)

**FAIL** if:
- Any TC1-TC3 fails consistently (>50% failure rate)
- New Critical bugs discovered
- Timeouts or blocking issues reappear

---

## Reporting

**Format**: GitHub Issue

**Template**:
```markdown
## Manual Test Results: Sprint 1 E2E Fixes

**Tester**: [Name]
**Date**: [YYYY-MM-DD]
**Environment**: [Browser/OS]
**Build**: [Commit SHA]

### Results

- [ ] TC1: Feature Toggle Interactions - PASS/FAIL
- [ ] TC2: Concurrent Toggle Operations - PASS/FAIL
- [ ] TC3: Config Reload During Toggle - PASS/FAIL
- [ ] TC4: Cross-Browser Consistency - PASS/FAIL
- [ ] TC5: DNS Provider Forms (Firefox) - PASS/FAIL

### Issues Found

1. [Issue description]
   - Severity: P0/P1/P2/P3
   - Reproduction steps
   - Screenshots/logs

### Overall Assessment

[PASS/FAIL with justification]

### Recommendation

[GO for deployment / HOLD pending fixes]
```

---

## Next Steps

1. **Sprint 2 Week 1**: Execute manual tests
2. **If PASS**: Approve for production deployment (after Docker Image Scan)
3. **If FAIL**: Create bug tickets and assign to Sprint 2 Week 2

---

**Notes**:
- This test plan focuses on potential user-facing bugs that automated tests might miss
- Emphasizes cross-browser compatibility and real-world usage patterns
- Complements automated E2E tests, doesn't replace them
