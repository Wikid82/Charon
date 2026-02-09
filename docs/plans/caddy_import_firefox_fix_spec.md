# Caddy Import Firefox Fix - Investigation & Test Plan

**Status**: Investigation & Planning
**Priority**: P0 CRITICAL
**Issue**: GitHub Issue #567 - Caddyfile import failing in Firefox
**Date**: 2026-02-03
**Investigator**: Planning Agent

---

## Executive Summary

### Issue Description
User reports that the "Parse and Review" button does not work when clicked in Firefox. Backend logs show `"record not found"` error when checking for import sessions:
```
Path: /app/backend/internal/api/handlers/import_handler.go:61
Query: SELECT * FROM import_sessions WHERE status IN ("pending","reviewing")
```

### Current Status Assessment
Based on code review and git history:
- ✅ **Recent fixes applied** (Jan 26 - Feb 1):
  - Fixed multi-file import API contract mismatch (commit `eb1d710f`)
  - Added file_server directive warning extraction
  - Enhanced import feedback and error handling
- ⚠️ **Testing gap identified**: No explicit Firefox-specific E2E tests for Caddy import
- ❓ **Root cause unclear**: Issue may be fixed by recent commits or may be browser-specific bug

---

## 1. Root Cause Analysis

### 1.1 Code Flow Analysis

**Frontend → Backend Flow**:
```
1. User clicks "Parse and Review" button
   ├─ File: frontend/src/pages/ImportCaddy.tsx:handleUpload()
   └─ Calls: uploadCaddyfile(content) → POST /api/v1/import/upload

2. Backend receives request
   ├─ File: backend/internal/api/handlers/import_handler.go:Upload()
   ├─ Creates transient session (no DB write initially)
   └─ Returns: { session, preview, caddyfile_content }

3. Frontend displays review table
   ├─ File: frontend/src/pages/ImportCaddy.tsx:setShowReview(true)
   └─ Component: ImportReviewTable

4. User clicks "Commit" button
   ├─ File: frontend/src/pages/ImportCaddy.tsx:handleCommit()
   └─ Calls: commitImport() → POST /api/v1/import/commit
```

**Backend Database Query Path**:
```go
// File: backend/internal/api/handlers/import_handler.go:61
err := h.db.Where("status IN ?", []string{"pending", "reviewing"}).
    Order("created_at DESC").
    First(&session).Error
```

### 1.2 Potential Root Causes

#### Hypothesis 1: Event Handler Binding Issue (Firefox-specific)
**Likelihood**: Medium
**Evidence**:
- Firefox handles button click events differently than Chromium
- Async state updates may cause race conditions
- No explicit Firefox testing in CI

**Test Criteria**:
```typescript
// Verify button is enabled and clickable
const parseButton = page.getByRole('button', { name: /parse|review/i });
await expect(parseButton).toBeEnabled();
await expect(parseButton).not.toHaveAttribute('disabled');

// Verify event listener is attached (Firefox-specific check)
const hasClickHandler = await parseButton.evaluate(
  (btn) => !!btn.onclick || !!btn.getAttribute('onclick')
);
```

#### Hypothesis 2: Race Condition in State Management
**Likelihood**: High
**Evidence**:
- Recent fixes addressed API response handling
- `useImport` hook manages complex async state
- Transient sessions may not be properly handled

**Test Criteria**:
```typescript
// Register API waiter BEFORE clicking button
const uploadPromise = page.waitForResponse(
  r => r.url().includes('/api/v1/import/upload')
);
await parseButton.click();
const response = await uploadPromise;

// Verify response structure
expect(response.ok()).toBeTruthy();
const body = await response.json();
expect(body.session).toBeDefined();
expect(body.preview).toBeDefined();
```

#### Hypothesis 3: CORS or Request Header Issue (Firefox-specific)
**Likelihood**: Low
**Evidence**:
- Firefox has stricter CORS enforcement
- Axios client configuration may differ between browsers
- No CORS errors reported in issue

**Test Criteria**:
```typescript
// Monitor network requests for CORS failures
const failedRequests: string[] = [];
page.on('requestfailed', request => {
  failedRequests.push(request.url());
});

await parseButton.click();
expect(failedRequests).toHaveLength(0);
```

#### Hypothesis 4: Session Storage/Cookie Issue
**Likelihood**: Medium
**Evidence**:
- Backend query returns "record not found"
- Firefox may handle session storage differently
- Auth cookies must be domain-scoped correctly

**Test Criteria**:
```typescript
// Verify auth cookies are present and valid
const cookies = await context.cookies();
const authCookie = cookies.find(c => c.name.includes('auth'));
expect(authCookie).toBeDefined();

// Verify request includes auth headers
const request = await uploadPromise;
const headers = request.request().headers();
expect(headers['authorization'] || headers['cookie']).toBeDefined();
```

### 1.3 Recent Code Changes Analysis

**Commit `eb1d710f` (Feb 1, 2026)**: Fixed multi-file import API contract
- **Changes**: Updated `ImportSitesModal.tsx` to send `{files: [{filename, content}]}` instead of `{contents}`
- **Impact**: May have resolved underlying state management issues
- **Testing**: Need to verify fix works in Firefox

**Commit `fc2df97f`**: Improved Caddy import with directive detection
- **Changes**: Enhanced error messaging for import directives
- **Impact**: Better user feedback for edge cases
- **Testing**: Verify error messages display correctly in Firefox

---

## 2. E2E Test Coverage Analysis

### 2.1 Existing Test Files

| Test File | Purpose | Browser Coverage | Status |
|-----------|---------|------------------|--------|
| `tests/tasks/import-caddyfile.spec.ts` | Full wizard flow (18 tests) | Chromium only | ✅ Comprehensive |
| `tests/tasks/caddy-import-debug.spec.ts` | Diagnostic tests (6 tests) | Chromium only | ✅ Diagnostic |
| `tests/tasks/caddy-import-gaps.spec.ts` | Gap coverage (9 tests) | Chromium only | ✅ Edge cases |
| `tests/integration/import-to-production.spec.ts` | Integration tests | Chromium only | ✅ Smoke tests |

**Key Finding**: ❌ **ZERO Firefox/WebKit-specific Caddy import tests**

### 2.2 Browser Projects Configuration

**File**: `playwright.config.js`
```javascript
projects: [
  { name: 'chromium', use: devices['Desktop Chrome'] },
  { name: 'firefox', use: devices['Desktop Firefox'] },  // ← Configured
  { name: 'webkit', use: devices['Desktop Safari'] },    // ← Configured
]
```

**CI Configuration**: `.github/workflows/e2e-tests.yml`
```yaml
matrix:
  browser: [chromium, firefox, webkit]
```

**Actual Test Execution**:
- ✅ Tests run against all 3 browsers in CI
- ❌ No browser-specific test filters or tags
- ❌ No explicit Firefox validation for Caddy import

### 2.3 Coverage Gaps

| Gap | Impact | Priority |
|-----|--------|----------|
| No Firefox-specific import tests | Cannot detect Firefox-only bugs | P0 |
| No cross-browser event handler validation | Click handlers may fail silently | P0 |
| No browser-specific network request monitoring | CORS/header issues undetected | P1 |
| No explicit WebKit validation | Safari users may experience same issue | P1 |
| No mobile browser testing | Responsive issues undetected | P2 |

---

## 3. Reproduction Steps

### 3.1 Manual Reproduction (Firefox Required)

**Prerequisites**:
1. Start Charon E2E environment: `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
2. Open Firefox browser
3. Navigate to `http://localhost:8080`
4. Log in with admin credentials

**Test Steps**:
```
1. Navigate to /tasks/import/caddyfile
2. Paste valid Caddyfile content:
   ```
   test.example.com {
     reverse_proxy localhost:3000
   }
   ```
3. Click "Parse and Review" button
4. EXPECTED: Review table appears with parsed hosts
5. ACTUAL (if bug exists): Button does nothing, no API request sent

6. Open Firefox DevTools → Network tab
7. Repeat steps 2-3
8. EXPECTED: POST /api/v1/import/upload (200 OK)
9. ACTUAL (if bug exists): No request visible, or request fails

10. Check backend logs:
    ```bash
    docker logs charon-app 2>&1 | grep -i import | tail -50
    ```
11. EXPECTED: "Import Upload: received upload"
12. ACTUAL (if bug exists): "record not found" error at line 61
```

### 3.2 Automated E2E Reproduction Test

**File**: `tests/tasks/caddy-import-firefox-specific.spec.ts` (new file)

```typescript
import { test, expect } from '@playwright/test';

test.describe('Caddy Import - Firefox Specific @firefox-only', () => {
  test('should successfully parse Caddyfile in Firefox', async ({ page, browserName }) => {
    // Skip if not Firefox
    test.skip(browserName !== 'firefox', 'Firefox-specific test');

    await test.step('Navigate to import page', async () => {
      await page.goto('/tasks/import/caddyfile');
    });

    const caddyfile = 'test.example.com { reverse_proxy localhost:3000 }';

    await test.step('Paste Caddyfile content', async () => {
      await page.locator('textarea').fill(caddyfile);
    });

    let requestMade = false;
    await test.step('Monitor network request', async () => {
      // Register listener BEFORE clicking button (critical for Firefox)
      const uploadPromise = page.waitForResponse(
        r => r.url().includes('/api/v1/import/upload'),
        { timeout: 10000 }
      );

      // Click button
      await page.getByRole('button', { name: /parse|review/i }).click();

      try {
        const response = await uploadPromise;
        requestMade = true;

        // Verify successful response
        expect(response.ok()).toBeTruthy();
        const body = await response.json();
        expect(body.session).toBeDefined();
        expect(body.preview.hosts).toHaveLength(1);
      } catch (error) {
        console.error('❌ API request failed or not sent:', error);
      }
    });

    await test.step('Verify review table appears', async () => {
      if (!requestMade) {
        test.fail('API request was not sent after clicking Parse button');
      }

      const reviewTable = page.getByTestId('import-review-table');
      await expect(reviewTable).toBeVisible({ timeout: 5000 });
      await expect(reviewTable.getByText('test.example.com')).toBeVisible();
    });
  });

  test('should handle button double-click in Firefox', async ({ page, browserName }) => {
    test.skip(browserName !== 'firefox', 'Firefox-specific test');

    await page.goto('/tasks/import/caddyfile');
    const caddyfile = 'test.example.com { reverse_proxy localhost:3000 }';
    await page.locator('textarea').fill(caddyfile);

    // Monitor for duplicate requests
    const requests: string[] = [];
    page.on('request', req => {
      if (req.url().includes('/api/v1/import/upload')) {
        requests.push(req.method());
      }
    });

    const parseButton = page.getByRole('button', { name: /parse|review/i });

    // Double-click rapidly (Firefox may handle differently)
    await parseButton.click();
    await parseButton.click();

    // Wait for requests to complete
    await page.waitForTimeout(2000);

    // Should only send ONE request (button should be disabled after first click)
    expect(requests.length).toBeLessThanOrEqual(1);
  });
});
```

---

## 4. Comprehensive Test Implementation Plan

### 4.1 Test Strategy

**Objective**: Guarantee Caddy import works reliably across all 3 browsers (Chromium, Firefox, WebKit)

**Approach**:
1. **Cross-browser baseline tests** - Run existing tests against all browsers
2. **Browser-specific edge case tests** - Target known browser differences
3. **Performance comparison** - Measure timing differences between browsers
4. **Visual regression testing** - Ensure UI renders consistently

### 4.2 Test File Structure

```
tests/
├── tasks/
│   ├── import-caddyfile.spec.ts           # Existing (Chromium)
│   ├── caddy-import-debug.spec.ts         # Existing (Chromium)
│   ├── caddy-import-gaps.spec.ts          # Existing (Chromium)
│   └── caddy-import-cross-browser.spec.ts # NEW - Cross-browser suite
├── firefox-specific/                       # NEW - Firefox-only tests
│   ├── caddy-import-firefox.spec.ts
│   └── event-handler-regression.spec.ts
└── webkit-specific/                        # NEW - WebKit-only tests
    └── caddy-import-webkit.spec.ts
```

### 4.3 Test Scenarios Matrix

| Scenario | Chromium | Firefox | WebKit | Priority | File |
|----------|----------|---------|--------|----------|------|
| **Parse valid Caddyfile** | ✅ | ❌ | ❌ | P0 | `caddy-import-cross-browser.spec.ts` |
| **Handle parse errors** | ✅ | ❌ | ❌ | P0 | `caddy-import-cross-browser.spec.ts` |
| **Detect import directives** | ✅ | ❌ | ❌ | P0 | `caddy-import-cross-browser.spec.ts` |
| **Show conflict warnings** | ✅ | ❌ | ❌ | P0 | `caddy-import-cross-browser.spec.ts` |
| **Commit successful import** | ✅ | ❌ | ❌ | P0 | `caddy-import-cross-browser.spec.ts` |
| **Multi-file upload** | ✅ | ❌ | ❌ | P0 | `caddy-import-cross-browser.spec.ts` |
| **Button double-click protection** | ❌ | ❌ | ❌ | P1 | `firefox-specific/event-handler-regression.spec.ts` |
| **Network request timing** | ❌ | ❌ | ❌ | P1 | `caddy-import-cross-browser.spec.ts` |
| **Session persistence** | ❌ | ❌ | ❌ | P1 | `caddy-import-cross-browser.spec.ts` |
| **CORS header validation** | ❌ | ❌ | ❌ | P2 | `firefox-specific/caddy-import-firefox.spec.ts` |

### 4.4 New Test Files

#### Test 1: `tests/tasks/caddy-import-cross-browser.spec.ts`

**Purpose**: Run core Caddy import scenarios against all 3 browsers
**Execution**: `npx playwright test caddy-import-cross-browser.spec.ts --project=chromium --project=firefox --project=webkit`

**Test Cases**:
```typescript
1. Parse valid Caddyfile (all browsers)
   ├─ Paste content
   ├─ Click Parse button
   ├─ Wait for API response
   └─ Verify review table appears

2. Handle syntax errors (all browsers)
   ├─ Paste invalid content
   ├─ Click Parse button
   ├─ Expect 400 error
   └─ Verify error message displayed

3. Multi-file import flow (all browsers)
   ├─ Click multi-file button
   ├─ Upload main + site files
   ├─ Parse
   └─ Verify imported hosts

4. Conflict resolution (all browsers)
   ├─ Create existing host via API
   ├─ Import conflicting host
   ├─ Verify conflict indicator
   ├─ Select "Replace"
   └─ Commit and verify update

5. Session resume (all browsers)
   ├─ Start import session
   ├─ Navigate away
   ├─ Return to import page
   └─ Verify banner + Review button

6. Cancel import (all browsers)
   ├─ Parse content
   ├─ Click back/cancel
   ├─ Confirm dialog
   └─ Verify session cleared
```

#### Test 2: `tests/firefox-specific/caddy-import-firefox.spec.ts`

**Purpose**: Test Firefox-specific behaviors and edge cases
**Execution**: `npx playwright test firefox-specific --project=firefox`

**Test Cases**:
```typescript
1. Event listener attachment (Firefox-only)
   ├─ Verify onclick handler exists
   ├─ Verify button is not disabled
   └─ Verify event propagation works

2. Async state update race condition (Firefox-only)
   ├─ Fill content rapidly
   ├─ Click parse immediately
   ├─ Verify request sent despite quick action
   └─ Verify no "stale" state issues

3. CORS preflight handling (Firefox-only)
   ├─ Monitor network for OPTIONS request
   ├─ Verify CORS headers present
   └─ Verify POST request succeeds

4. Cookie/auth header verification (Firefox-only)
   ├─ Check cookies sent with request
   ├─ Verify Authorization header
   └─ Check session storage state

5. Button double-click protection (Firefox-only)
   ├─ Double-click Parse button rapidly
   ├─ Verify only 1 API request sent
   └─ Verify button disabled after first click

6. Large file handling (Firefox-only)
   ├─ Paste 10KB+ Caddyfile
   ├─ Verify no textarea lag
   └─ Verify upload completes
```

#### Test 3: `tests/webkit-specific/caddy-import-webkit.spec.ts`

**Purpose**: Validate Safari/WebKit compatibility
**Execution**: `npx playwright test webkit-specific --project=webkit`

**Test Cases**: (Same as Firefox test 1-6, adapted for WebKit)

### 4.5 Performance Testing

**File**: `tests/performance/caddy-import-perf.spec.ts`

**Objective**: Measure and compare browser performance

```typescript
test.describe('Caddy Import - Performance Comparison', () => {
  test('should parse Caddyfile within acceptable time', async ({ page, browserName }) => {
    const startTime = Date.now();

    await page.goto('/tasks/import/caddyfile');
    await page.locator('textarea').fill(largeCaddyfile); // 50+ hosts

    const uploadPromise = page.waitForResponse(
      r => r.url().includes('/api/v1/import/upload')
    );
    await page.getByRole('button', { name: /parse/i }).click();
    await uploadPromise;

    const endTime = Date.now();
    const duration = endTime - startTime;

    // Log browser-specific performance
    console.log(`${browserName}: ${duration}ms`);

    // Acceptable thresholds (adjust based on baseline)
    expect(duration).toBeLessThan(5000); // 5 seconds max
  });
});
```

---

## 5. Acceptance Criteria

### 5.1 Bug Fix Validation (if bug still exists)

- [ ] ✅ Parse button clickable in Firefox
- [ ] ✅ API request sent on button click (Firefox DevTools shows POST /api/v1/import/upload)
- [ ] ✅ Backend logs show "Import Upload: received upload" (no "record not found")
- [ ] ✅ Review table appears with parsed hosts
- [ ] ✅ Commit button works correctly
- [ ] ✅ No console errors in Firefox DevTools
- [ ] ✅ Same behavior in Chromium and Firefox

### 5.2 Test Coverage Requirements

- [ ] ✅ All critical scenarios pass in Chromium (baseline validation)
- [ ] ✅ All critical scenarios pass in Firefox (regression prevention)
- [ ] ✅ All critical scenarios pass in WebKit (Safari compatibility)
- [ ] ✅ Cross-browser test file created and integrated into CI
- [ ] ✅ Firefox-specific edge case tests passing
- [ ] ✅ Performance within acceptable thresholds across all browsers
- [ ] ✅ Zero cross-browser test failures in CI for 3 consecutive runs

### 5.3 CI Integration

- [ ] ✅ Cross-browser tests run on every PR
- [ ] ✅ Browser-specific test results visible in CI summary
- [ ] ✅ Failed tests show browser name in error message
- [ ] ✅ Codecov reports separate coverage per browser (if applicable)
- [ ] ✅ No increase in CI execution time (use sharding if needed)

---

## 6. Implementation Phases

### Phase 1: Investigation & Root Cause Identification (1-2 hours)

**Subagent**: Backend_Dev + Frontend_Dev

**Tasks**:
1. Manually reproduce issue in Firefox
2. Capture browser DevTools Network + Console logs
3. Capture backend logs showing error
4. Compare request/response between Chromium and Firefox
5. Identify exact line where behavior diverges
6. Document root cause with evidence

**Deliverable**: Root cause analysis report with screenshots/logs

---

### Phase 2: Fix Implementation (if bug exists) (2-4 hours)

**Subagent**: Frontend_Dev (or Backend_Dev depending on root cause)

**Potential Fixes**:

**Option A: Frontend Event Handler Fix** (if Hypothesis 1 confirmed)
```typescript
// File: frontend/src/pages/ImportCaddy.tsx

// BEFORE (potential issue)
<button onClick={handleUpload} disabled={loading || !content.trim()}>

// AFTER (fix race condition)
<button
  onClick={(e) => {
    e.preventDefault(); // Prevent double submission
    if (!loading && content.trim()) {
      handleUpload();
    }
  }}
  disabled={loading || !content.trim()}
  aria-busy={loading}
>
```

**Option B: Backend Session Handling Fix** (if Hypothesis 2 confirmed)
```go
// File: backend/internal/api/handlers/import_handler.go:Upload()

// Ensure transient session is properly initialized before returning
sid := uuid.NewString()
// ... (existing code)

// NEW: Log session creation for debugging
middleware.GetRequestLogger(c).WithField("session_id", sid).Info("Created transient session")

return &ImportPreview{
    Session: ImportSession{ID: sid, State: "transient"},
    // ...
}
```

**Option C: CORS Header Fix** (if Hypothesis 3 confirmed)
```go
// File: backend/internal/api/server.go (router setup)

// Add CORS headers for Firefox compatibility
router.Use(cors.New(cors.Config{
    AllowOrigins:     []string{"http://localhost:8080", "http://localhost:5173"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
    AllowCredentials: true,
    MaxAge:           12 * time.Hour,
}))
```

**Deliverable**: Code fix with explanation + manual Firefox validation

---

### Phase 3: E2E Test Implementation (4-6 hours)

**Subagent**: Playwright_Dev

**Tasks**:
1. Create `caddy-import-cross-browser.spec.ts` (6 core scenarios)
2. Create `firefox-specific/caddy-import-firefox.spec.ts` (6 edge cases)
3. Create `webkit-specific/caddy-import-webkit.spec.ts` (6 edge cases)
4. Update `playwright.config.js` if needed for browser-specific test filtering
5. Add `@firefox-only` and `@webkit-only` tags
6. Run tests locally against all 3 browsers
7. Verify 100% pass rate

**Deliverable**: 3 new test files with passing tests

---

### Phase 4: CI Integration (1-2 hours)

**Subagent**: Supervisor

**Tasks**:
1. Update `.github/workflows/e2e-tests.yml` to include new tests
2. Add browser matrix reporting to CI summary
3. Configure test sharding if execution time exceeds 15 minutes
4. Verify Firefox/WebKit browsers installed in CI environment
5. Run test workflow end-to-end in CI
6. Update documentation with cross-browser testing instructions

**Deliverable**: CI workflow passing with cross-browser tests

---

### Phase 5: Documentation & Handoff (1 hour)

**Subagent**: Backend_Dev + Frontend_Dev

**Tasks**:
1. Update `docs/api.md` with any API changes
2. Update `README.md` with Firefox testing instructions
3. Add browser compatibility matrix to documentation
4. Create KB article for "Caddy Import Firefox Issues" (if bug found)
5. Update CHANGELOG.md with fix details
6. Close GitHub Issue #567 with resolution summary

**Deliverable**: Updated documentation

---

## 7. Risk Assessment

### 7.1 Technical Risks

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Bug is intermittent/timing-based | High | Medium | Add extensive logging + retry tests |
| Root cause requires backend refactor | High | Low | Phase approach allows pivoting |
| Fix breaks Chromium compatibility | Critical | Low | Run all existing tests before/after |
| CI execution time increases >20% | Medium | Medium | Use test sharding across 4 workers |
| WebKit has additional unique bugs | Medium | Medium | Include WebKit in Phase 3 |

### 7.2 Schedule Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Investigation takes longer than 2 hours | Medium | Allocate 4-hour buffer in Phase 1 |
| Cross-browser tests reveal new bugs | High | Prioritize P0 bugs only, defer P1/P2 |
| CI integration blocked by infrastructure | High | Manual testing as fallback |

---

## 8. Success Metrics

### 8.1 Bug Resolution Metrics

- **Fix Validated**: Manual Firefox test shows working Parse button
- **Zero Regressions**: All existing Chromium tests still pass
- **Backend Logs Clean**: No "record not found" errors in logs

### 8.2 Test Coverage Metrics

- **Cross-browser Pass Rate**: 100% across Chromium, Firefox, WebKit
- **Test Execution Time**: <15 minutes for full cross-browser suite
- **Code Coverage**: No decrease in backend/frontend coverage
- **CI Stability**: Zero flaky tests in 10 consecutive CI runs

### 8.3 Quality Gates

- [ ] ✅ All Phase 1-5 tasks completed
- [ ] ✅ Issue #567 closed with root cause documented
- [ ] ✅ PR approved by 2+ reviewers
- [ ] ✅ Codecov patch coverage: 100%
- [ ] ✅ No new security vulnerabilities introduced
- [ ] ✅ Firefox manual testing video attached to PR

---

## 9. Appendix

### 9.1 Reference Files

**Backend**:
- `backend/internal/api/handlers/import_handler.go` - Import logic
- `backend/internal/caddy/importer.go` - Caddyfile parsing
- `backend/internal/models/import_session.go` - Session model

**Frontend**:
- `frontend/src/pages/ImportCaddy.tsx` - Main import page
- `frontend/src/hooks/useImport.ts` - Import state management
- `frontend/src/api/import.ts` - API client
- `frontend/src/components/ImportReviewTable.tsx` - Review UI

**Tests**:
- `tests/tasks/import-caddyfile.spec.ts` - Existing E2E tests
- `tests/tasks/caddy-import-debug.spec.ts` - Diagnostic tests
- `tests/tasks/caddy-import-gaps.spec.ts` - Gap coverage

**Config**:
- `playwright.config.js` - E2E test configuration
- `.github/workflows/e2e-tests.yml` - CI pipeline

### 9.2 Recent Commits (Context)

- `eb1d710f` - Fix multi-file import API contract (Feb 1, 2026)
- `fc2df97f` - Improve Caddy import with directive detection
- `c3b20bff` - Implement Caddy import E2E gap tests
- `a3fea249` - Add patch coverage tests for Caddy import normalization

### 9.3 Browser Compatibility Matrix

| Feature | Chromium | Firefox | WebKit | Notes |
|---------|----------|---------|--------|-------|
| Basic parsing | ✅ | ❓ | ❓ | User reports Firefox fails |
| Multi-file import | ✅ | ❓ | ❓ | Recently fixed (eb1d710f) |
| Error handling | ✅ | ❓ | ❓ | 400 warning extraction added |
| Conflict resolution | ✅ | ❓ | ❓ | Covered in gap tests |
| Session resume | ✅ | ❓ | ❓ | Transient sessions only |

**Legend**:
- ✅ Tested and working
- ❓ Not tested
- ❌ Known to fail
- ⚠️ Flaky/intermittent

### 9.4 Testing Instructions for Manual Validation

**Prerequisites**:
```bash
# Start E2E container
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Install Firefox (if not present)
npx playwright install firefox
```

**Run Firefox-specific tests**:
```bash
# Single test file
npx playwright test tests/tasks/import-caddyfile.spec.ts --project=firefox

# All Caddy import tests (Firefox only)
npx playwright test tests/tasks/caddy-import-*.spec.ts --project=firefox

# Cross-browser comparison
npx playwright test tests/tasks/import-caddyfile.spec.ts --project=chromium --project=firefox --project=webkit
```

**Debug mode** (headed with inspector):
```bash
npx playwright test tests/tasks/import-caddyfile.spec.ts --project=firefox --headed --debug
```

---

## 10. Decision Log

| Date | Decision | Rationale | Approved By |
|------|----------|-----------|-------------|
| 2026-02-03 | Prioritize Firefox investigation over new features | Caddy is foundation of project (P0) | Planning Agent |
| 2026-02-03 | Include WebKit in test plan | Prevent similar Safari issues | Planning Agent |
| 2026-02-03 | Create browser-specific test directories | Better organization for future | Planning Agent |
| 2026-02-03 | Use existing test infrastructure | No new dependencies needed | Planning Agent |

---

## 11. Next Steps

**Immediate Actions** (Supervisor to delegate):
1. ✅ Approve this plan
2. ⏭️ Assign Phase 1 to Backend_Dev + Frontend_Dev
3. ⏭️ Create GitHub Issue for cross-browser testing (if not exists)
4. ⏭️ Schedule code review with 2+ reviewers

**Phase 1 Start**: Immediately after plan approval
**Target Completion**: Within 1 sprint (2 weeks max)

---

**Plan Status**: ✅ READY FOR REVIEW
**Confidence Level**: HIGH (85%)
**Estimated Total Effort**: 10-15 developer-hours across 5 phases
**Blocking Issues**: None
**Dependencies**: Docker environment, Firefox installation

---

**Document Version**: 1.0
**Last Updated**: 2026-02-03
**Next Review**: After Phase 1 completion
