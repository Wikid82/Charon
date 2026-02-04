# Phase 3 Technical Debt Issues

## Issue 1: Test Infrastructure - Resolve undici WebSocket conflicts

**Priority**: P1
**Estimate**: 8-12 hours
**Milestone**: Next Sprint

### Problem

The current test infrastructure (jsdom + undici) has a known WebSocket compatibility issue that prevents testing of components using `LiveLogViewer`:

- **Current State**: 190 pre-existing unhandled rejections in test suite
- **Blocker**: `InvalidArgumentError: websocket upgrade may only be requested on a HTTP/1.1 request`
- **Impact**: Cannot test Security.tsx, SecurityHeaders.tsx, Dashboard.tsx components (458 test cases created but unusable)
- **Coverage Impact**: Frontend stuck at 84.25%, cannot reach 85% target without infrastructure fix

### Root Cause

jsdom uses undici v5.x internally which has incomplete WebSocket support. When Mock Service Worker (MSW) v1.x intercepts fetch requests, undici's WebSocket client throws errors when attempting to upgrade connections.

**Evidence**:
```
Error: InvalidArgumentError: websocket upgrade may only be requested on a HTTP/1.1 request
    at new WebSocket (node_modules/undici/lib/web/websocket/websocket.js:95:13)
    at new WebSocketClient (frontend/src/lib/websocket-client.ts:34:5)
```

### Proposed Solutions

#### Option A: Upgrade MSW to v2.x (Recommended)
- **Effort**: 4-6 hours
- **Pros**:
  - Uses native `fetch()` API (more standards-compliant)
  - Better undici compatibility
  - Smaller migration surface (MSW API changes only)
- **Cons**:
  - Breaking changes in MSW v2.x API
  - Need to update all MSW handlers and setup files
- **Migration Guide**: https://mswjs.io/docs/migrations/1.x-to-2.x

#### Option B: Migrate to happy-dom (Alternative)
- **Effort**: 8-12 hours
- **Pros**:
  - Better WebSocket support out-of-the-box
  - Faster than jsdom for large DOM trees
  - Growing adoption in React ecosystem
- **Cons**:
  - Larger migration surface (entire test environment)
  - Potential compatibility issues with existing tests
  - Less mature than jsdom
- **Documentation**: https://github.com/capricorn86/happy-dom

#### Option C: Vitest Browser Mode (Long-term)
- **Effort**: 12-16 hours
- **Pros**:
  - Real browser environment (no DOM emulation)
  - Playwright integration (consistent with E2E tests)
  - Best WebSocket support
- **Cons**:
  - Largest migration effort
  - Requires CI infrastructure changes
  - Slower test execution
- **Documentation**: https://vitest.dev/guide/browser.html

### Recommended Approach

1. **Immediate (Sprint 1)**: Upgrade MSW to v2.x
   - Fixes WebSocket compatibility with minimal disruption
   - Validates solution with existing 458 test cases
   - Expected coverage improvement: 84.25% → 86-87%

2. **Future (Q2 2026)**: Evaluate happy-dom or Vitest browser mode
   - Re-assess after MSW v2.x validates WebSocket testing
   - Consider if additional benefits justify migration effort

### Acceptance Criteria

- [ ] 190 pre-existing unhandled rejections reduced to zero
- [ ] All test utilities using WebSocket work correctly:
  - `LiveLogViewer` component
  - `WebSocketProvider` context
  - Real-time log streaming tests
- [ ] 458 created test cases (Security.tsx, SecurityHeaders.tsx, Dashboard.tsx) execute successfully
- [ ] Frontend coverage improves from 84.25% to ≥85%
- [ ] No regression in existing 1552 passing tests
- [ ] CI pipeline remains stable (execution time <10min)

### Implementation Plan

**Phase 1: Research (Day 1)**
- [ ] Audit all MSW v1.x usages in codebase
- [ ] Review MSW v2.x migration guide
- [ ] Create detailed migration checklist
- [ ] Document breaking changes and required code updates

**Phase 2: Upgrade MSW (Days 2-3)**
- [ ] Update `package.json`: `msw@^2.0.0`
- [ ] Update MSW handlers in `frontend/src/mocks/handlers.ts`
- [ ] Update MSW setup in `frontend/src/setupTests.ts`
- [ ] Fix any breaking changes in test files
- [ ] Run frontend tests locally: `npm test`

**Phase 3: Validate WebSocket Support (Day 4)**
- [ ] Run Security.tsx test suite (200 tests)
- [ ] Run SecurityHeaders.tsx test suite (143 tests)
- [ ] Run Dashboard.tsx test suite (115 tests)
- [ ] Verify zero unhandled rejections
- [ ] Check frontend coverage: `npm run test:coverage`

**Phase 4: CI Validation (Day 5)**
- [ ] Push to feature branch
- [ ] Monitor CI test results
- [ ] Verify no regressions in E2E tests
- [ ] Confirm Codecov patch coverage ≥85%
- [ ] Merge if all checks pass

### References

- **Root Cause Analysis**: [docs/reports/phase3_3_findings.md](../reports/phase3_3_findings.md)
- **Coverage Gap Analysis**: [docs/reports/phase3_coverage_gap_analysis.md](../reports/phase3_coverage_gap_analysis.md)
- **Completion Report**: [docs/reports/phase3_3_completion_report.md](../reports/phase3_3_completion_report.md)
- **MSW Migration Guide**: https://mswjs.io/docs/migrations/1.x-to-2.x
- **Undici WebSocket Issue**: https://github.com/nodejs/undici/issues/1671

---

## Issue 2: Weak Assertions - Strengthen certificates.spec.ts validation

**Priority**: P2
**Estimate**: 2-3 hours
**Milestone**: Q1 2026

### Problem

Phase 2 code review identified 15+ instances of weak assertions in `tests/core/certificates.spec.ts` that verify UI interactions but not underlying data changes. Examples:

- Line 403: Verifies dialog closed but not certificate data deleted from API
- Line 551: Verifies form submitted but not certificate created in database
- Line 654: Verifies toggle clicked but not "Force SSL" flag persisted

### Impact

- Tests pass even if API operations fail silently
- False sense of security (green tests, broken features)
- Reduced confidence in regression detection

### Proposed Solution

Add data validation assertions after UI interactions:

**Pattern**:
```typescript
// ❌ Weak: Only verifies UI state
await clickButton(page, 'Delete');
await expect(dialog).not.toBeVisible();

// ✅ Strong: Verifies API state
await clickButton(page, 'Delete');
await expect(dialog).not.toBeVisible();

// Verify certificate no longer exists
const response = await page.request.get(`/api/v1/certificates/${certId}`);
expect(response.status()).toBe(404);
```

### Acceptance Criteria

- [ ] All delete operations verify HTTP 404 response
- [ ] All create operations verify HTTP 201 response with correct data
- [ ] All update operations verify HTTP 200 response with updated fields
- [ ] Toggle operations verify API state matches UI state
- [ ] No reduction in test execution speed (<10% increase acceptable)

### Reference

- **Issue Document**: [docs/issues/weak_assertions_certificates_spec.md](./weak_assertions_certificates_spec.md)
- **Code Review Notes**: Phase 2.2 Supervisor checkpoint

---

## Issue 3: Coverage Improvement - Target untouched packages

**Priority**: P2
**Estimate**: 6-8 hours
**Milestone**: Q1 2026

### Problem

Phase 3 backend coverage improvements targeted 5 packages and successfully brought them to 85%+, but overall coverage only reached 84.2% due to untouched packages:

- **services package**: 82.6% (needs +2.4% to reach 85%)
- **builtin DNS provider**: 30.4% (needs +54.6% to reach 85%)
- **Other packages**: Various levels below 85%

### Proposed Solution

**Sprint 1: Services Package** (Priority, 3-4 hours)
- Target: 82.6% → 85%
- Focus areas:
  - `internal/services/certificate_service.go` (renewal logic)
  - `internal/services/proxy_host_service.go` (validation)
  - `internal/services/dns_provider_service.go` (sync operations)

**Sprint 2: Builtin DNS Provider** (Lower priority, 3-4 hours)
- Target: 30.4% → 50% (incremental improvement)
- Focus areas:
  - `internal/dnsprovider/builtin/provider.go` (ACME integration)
  - Error handling and edge cases
  - Configuration validation

### Acceptance Criteria

- [ ] Backend coverage improves from 84.2% to ≥85%
- [ ] All new tests use table-driven test pattern
- [ ] Test execution time remains <5 seconds
- [ ] No flaky tests introduced
- [ ] Codecov patch coverage ≥85% on modified files

### Reference

- **Gap Analysis**: [docs/reports/phase3_coverage_gap_analysis.md](../reports/phase3_coverage_gap_analysis.md)
- **Phase 3.2 Results**: Backend coverage increased from 83.5% to 84.2% (+0.7%)

---

## Issue 4: Feature Flag Tests - Fix async propagation failures

**Priority**: P2
**Estimate**: 2-3 hours
**Milestone**: Q1 2026

### Problem

4 tests in `tests/settings/system-settings.spec.ts` are skipped due to async propagation issues:

```typescript
test.skip('should toggle CrowdSec console enrollment', async ({ page }) => {
  // Skipped: Async propagation to frontend not working reliably
});
```

### Root Cause

Feature flag changes propagate asynchronously from backend → Caddy → frontend. Tests toggle flag and immediately verify UI state, but frontend hasn't received update yet.

### Proposed Solution

Use `waitForFeatureFlagPropagation()` helper after toggle operations:

```typescript
test('should toggle CrowdSec console enrollment', async ({ page }) => {
  const toggle = page.getByRole('switch', { name: /crowdsec.*enrollment/i });
  const initialState = await toggle.isChecked();

  await clickSwitchAndWaitForResponse(page, toggle, /\/feature-flags/);

  // ✅ Wait for propagation before verifying UI
  await waitForFeatureFlagPropagation(page, {
    'crowdsec.console_enrollment': !initialState,
  });

  await expect(toggle).toBeChecked({ checked: !initialState });
});
```

### Acceptance Criteria

- [ ] All 4 skipped tests enabled and passing
- [ ] Tests pass consistently across Chromium, Firefox, WebKit
- [ ] No increase in test execution time (<5% acceptable)
- [ ] No flaky test failures in CI (run 10x to verify)

### Reference

- **Skipped Tests**: Lines 234, 298, 372, 445 in `tests/settings/system-settings.spec.ts`
- **Wait Helper Docs**: [tests/utils/wait-helpers.ts](../../tests/utils/wait-helpers.ts)

---

## Issue 5: WebKit E2E Tests - Investigate execution failure

**Priority**: P3
**Estimate**: 2-3 hours
**Milestone**: Q2 2026

### Problem

During Phase 2.4 validation, WebKit tests did not execute despite being specified in the command:

```bash
npx playwright test --project=chromium --project=firefox --project=webkit
```

**Observed**:
- Chromium: 873 tests passed
- Firefox: 873 tests passed
- WebKit: 0 tests executed (no errors, just skipped)

### Possible Root Causes

1. **Configuration Issue**: WebKit project disabled in `playwright.config.js`
2. **Environment Issue**: WebKit browser not installed or missing dependencies
3. **Container Issue**: E2E Docker container missing WebKit support
4. **Silent Skip**: WebKit tests tagged with conditional skip that wasn't reported

### Investigation Steps

1. **Verify Configuration**:
   ```bash
   # Check WebKit project exists in config
   grep -A 10 "name.*webkit" playwright.config.js
   ```

2. **Verify Browser Installation**:
   ```bash
   # List installed browsers
   npx playwright install --dry-run

   # Install WebKit if missing
   npx playwright install webkit
   ```

3. **Test WebKit Directly**:
   ```bash
   # Run single test file with WebKit only
   npx playwright test tests/core/authentication.spec.ts --project=webkit --headed
   ```

4. **Check Container Logs**:
   ```bash
   # If running in Docker
   docker logs charon-e2e | grep -i webkit
   ```

### Acceptance Criteria

- [ ] Root cause documented with evidence
- [ ] WebKit tests execute successfully (873 tests expected)
- [ ] WebKit browser installed and working in both local and CI environments
- [ ] CI workflow updated if configuration changes needed
- [ ] Documentation updated with WebKit-specific requirements (if any)

### Reference

- **Phase 2.4 Validation Report**: [docs/reports/phase2_complete.md](../reports/phase2_complete.md)
- **Playwright Config**: [playwright.config.js](../../playwright.config.js)

---

## Instructions for Creating GitHub Issues

Copy each issue above into GitHub Issues UI with the following settings:

**Issue 1 (WebSocket Infrastructure)**:
- Title: `[Test Infrastructure] Resolve undici WebSocket conflicts`
- Labels: `P1`, `testing`, `infrastructure`, `technical-debt`
- Milestone: `Next Sprint`
- Assignee: TBD

**Issue 2 (Weak Assertions)**:
- Title: `[Test Quality] Strengthen certificates.spec.ts assertions`
- Labels: `P2`, `testing`, `test-quality`, `tech-debt`
- Milestone: `Q1 2026`
- Assignee: TBD

**Issue 3 (Coverage Gaps)**:
- Title: `[Coverage] Improve backend coverage for services and builtin DNS`
- Labels: `P2`, `testing`, `coverage`, `backend`
- Milestone: `Q1 2026`
- Assignee: TBD

**Issue 4 (Feature Flag Tests)**:
- Title: `[E2E] Fix skipped feature flag propagation tests`
- Labels: `P2`, `testing`, `e2e`, `bug`
- Milestone: `Q1 2026`
- Assignee: TBD

**Issue 5 (WebKit)**:
- Title: `[E2E] Investigate WebKit test execution failure`
- Labels: `P3`, `testing`, `investigation`, `webkit`
- Milestone: `Q2 2026`
- Assignee: TBD

---

**Created**: 2026-02-03
**Related PR**: #609 (E2E Test Triage and Beta Release Preparation)
**Phase**: Phase 3 Follow-up
