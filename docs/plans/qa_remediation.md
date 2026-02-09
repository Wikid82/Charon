# QA Remediation Plan

**Date:** December 23, 2025
**Status:** 🔴 CRITICAL - Required for Beta Release
**Current State:**

- Coverage: **84.7%** (Target: ≥85%) - **0.3% gap**
- Test Failures: **7 test scenarios (21 total sub-test failures)**
- Root Cause: SSRF protection correctly blocking localhost/private IPs in tests

---

## Executive Summary

The QA audit identified two blockers preventing merge:

1. **Test Failures:** 7 tests fail because SSRF protection (working correctly) blocks localhost URLs used by `httptest` servers
2. **Coverage Gap:** 0.3% below the 85% threshold due to low coverage in `internal/utils` (51.5%)

**Estimated Total Effort:** 4-6 hours
**Approach:** Mock HTTP transport for tests + add targeted test coverage
**Risk Level:** Low (fixes do not weaken security)

---

## Priority 1: Fix Test Failures (3-4 hours)

### Overview

All 7 failing tests are caused by the new SSRF protection correctly blocking `httptest.NewServer()` and `httptest.NewTLSServer()`, which bind to `127.0.0.1`. The tests need to be refactored to use mock HTTP transports instead of real HTTP servers.

### Failing Tests Breakdown

#### 1.1 URL Connectivity Tests (6 scenarios, 18 sub-tests)

**Location:** `backend/internal/utils/url_connectivity_test.go`

**Failing Tests:**

- `TestTestURLConnectivity_Success` (line 17)
- `TestTestURLConnectivity_Redirect` (line 35)
- `TestTestURLConnectivity_TooManyRedirects` (line 56)
- `TestTestURLConnectivity_StatusCodes` (11 sub-tests, line 70)
  - 200 OK, 201 Created, 204 No Content
  - 301 Moved Permanently, 302 Found
  - 400 Bad Request, 401 Unauthorized, 403 Forbidden
  - 404 Not Found, 500 Internal Server Error, 503 Service Unavailable
- `TestTestURLConnectivity_InvalidURL` (3 of 4 sub-tests, line 114)
  - Empty URL, Invalid scheme, No scheme
- `TestTestURLConnectivity_Timeout` (line 143)

**Error Message:**

```
access to private IP addresses is blocked (resolved to 127.0.0.1)
```

**Analysis:**

- These tests use `httptest.NewServer()` which creates servers on `127.0.0.1`
- SSRF protection correctly identifies and blocks these private IPs
- Tests need to use mock transport that bypasses DNS resolution and HTTP connection

**Solution Approach:**

Create a test helper that injects a custom `http.RoundTripper` into the `TestURLConnectivity` function:

```go
// Option A: Add test-only parameter (least invasive)
func TestURLConnectivity(rawURL string, transport ...http.RoundTripper) (bool, float64, error) {
    // Use custom transport if provided, otherwise use default
}

// Option B: Refactor for dependency injection (cleaner)
type ConnectivityTester struct {
    Transport http.RoundTripper
    Timeout   time.Duration
}
```

**Specific Changes Required:**

1. **Modify `url_testing.go`:**
   - Add optional `http.RoundTripper` parameter to `TestURLConnectivity()`
   - Use provided transport if available, otherwise use default client
   - This allows tests to mock HTTP without triggering SSRF checks

2. **Update all 6 failing test functions:**
   - Replace `httptest.NewServer()` with mock `http.RoundTripper`
   - Use `httptest.NewRequest()` for building test requests
   - Mock response with custom `http.Response` objects

**Example Mock Transport Pattern:**

```go
// Test helper for mocking HTTP responses
type mockTransport struct {
    statusCode int
    headers    http.Header
    body       string
    err        error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    if m.err != nil {
        return nil, m.err
    }

    resp := &http.Response{
        StatusCode: m.statusCode,
        Header:     m.headers,
        Body:       io.NopCloser(strings.NewReader(m.body)),
        Request:    req,
    }
    return resp, nil
}
```

**Files to Modify:**

- `backend/internal/utils/url_testing.go` (add transport parameter)
- `backend/internal/utils/url_connectivity_test.go` (update all 6 failing tests)

**Estimated Effort:** 2-3 hours

---

#### 1.2 Settings Handler Test (1 scenario)

**Location:** `backend/internal/api/handlers/settings_handler_test.go`

**Failing Test:**

- `TestSettingsHandler_TestPublicURL_Success` (line 662)

**Error Details:**

```
Line 673: Expected: true, Actual: false (reachable)
Line 674: Expected not nil (latency)
Line 675: Expected not nil (message)
```

**Root Cause:**

- Test calls `/settings/test-url` endpoint which internally calls `TestURLConnectivity()`
- The test is trying to validate a localhost URL, which is blocked by SSRF protection
- Unlike the utils tests, this is testing the full HTTP handler flow

**Solution Approach:**

Use test doubles (mock/spy) for the `TestURLConnectivity` function within the handler test:

```go
// Option A: Extract URL validator as interface for mocking
type URLValidator interface {
    TestURLConnectivity(url string) (bool, float64, error)
}

// Option B: Use test server with overridden connectivity function
// (requires refactoring handler to accept injected validator)
```

**Specific Changes Required:**

1. **Refactor `settings_handler.go`:**
   - Extract URL validation logic into a testable interface
   - Allow dependency injection of validator for testing

2. **Update `settings_handler_test.go`:**
   - Create mock validator that returns success for test URLs
   - Inject mock into handler during test setup
   - Alternatively: use a real public URL (e.g., `https://httpbin.org/status/200`)

**Alternative Quick Fix (if refactoring is too invasive):**

- Change test to use a real public URL instead of localhost
- Add comment explaining why we use external URL for this specific test
- Pros: No code changes to production code
- Cons: Test depends on external service availability

**Recommended Approach:**
Use the quick fix for immediate unblocking, plan interface refactoring for post-release cleanup.

**Files to Modify:**

- `backend/internal/api/handlers/settings_handler_test.go` (line 662-676)
- Optionally: `backend/internal/api/handlers/settings_handler.go` (if using DI approach)

**Estimated Effort:** 1 hour (quick fix) or 2 hours (full refactoring)

---

## Priority 2: Increase Test Coverage to ≥85% (2 hours)

### Current Coverage by Package

**Packages Below Target:**

- `internal/utils`: **51.5%** (biggest gap)
- `internal/services`: 83.5% (close but below)
- `cmd/seed`: 62.5%

**Total Coverage:** 84.7% (need +0.3% minimum, target +1% for safety margin)

### Coverage Strategy

Focus on `internal/utils` package as it has the largest gap and fewest lines of code to cover.

#### 2.1 Missing Coverage in `internal/utils`

**Files in package:**

- `url.go` (likely low coverage)
- `url_testing.go` (covered by existing tests)

**Action Items:**

1. **Audit `url.go` for uncovered functions:**

   ```bash
   cd backend && go test -coverprofile=coverage.out ./internal/utils
   go tool cover -html=coverage.out -o utils_coverage.html
   # Open HTML to identify uncovered lines
   ```

2. **Add tests for uncovered functions in `url.go`:**
   - Look for utility functions related to URL parsing, validation, or manipulation
   - Write targeted unit tests for each uncovered function
   - Aim for 80-90% coverage of `url.go` to bring package average above 70%

**Expected Impact:**

- Adding 5-10 tests for `url.go` functions should increase package coverage from 51.5% to ~75%
- This alone should push total coverage from 84.7% to **~85.5%** (exceeding target)

**Files to Modify:**

- Add new test file: `backend/internal/utils/url_test.go`
- Or expand: `backend/internal/utils/url_connectivity_test.go`

**Estimated Effort:** 1.5 hours

---

#### 2.2 Additional Coverage (if needed)

If `url.go` tests don't push coverage high enough, target these next:

**Option A: `internal/services` (83.5% → 86%)**

- Review coverage HTML report for services with lowest coverage
- Add edge case tests for error handling paths
- Focus on: `access_list_service.go`, `backup_service.go`, `certificate_service.go`

**Option B: `cmd/seed` (62.5% → 75%)**

- Add tests for seeding logic and error handling
- Mock database interactions
- Test seed data validation

**Recommended:** Start with `internal/utils`, only proceed to Option A/B if necessary.

**Estimated Effort:** 0.5-1 hour (if required)

---

## Implementation Plan & Sequence

### Phase 1: Coverage First (Get to ≥85%)

**Rationale:** Easier to run tests without failures constantly appearing

**Steps:**

1. ✅ Audit `internal/utils/url.go` for uncovered functions
2. ✅ Write unit tests for all uncovered functions in `url.go`
3. ✅ Run coverage report: `go test -coverprofile=coverage.out ./...`
4. ✅ Verify coverage ≥85.5% (safety margin)

**Exit Criteria:** Total coverage ≥85%

---

### Phase 2: Fix Test Failures (Make all tests pass)

#### Step 2A: Fix Utils Tests (Priority 1.1)

1. ✅ Add `http.RoundTripper` parameter to `TestURLConnectivity()`
2. ✅ Create mock transport helper in test file
3. ✅ Update all 6 failing test functions to use mock transport
4. ✅ Run tests: `go test ./internal/utils -v`
5. ✅ Verify all tests pass

**Exit Criteria:** All utils tests pass

#### Step 2B: Fix Settings Handler Test (Priority 1.2)

1. ✅ Choose approach: Quick fix (public URL) or DI refactoring
2. ✅ Implement fix in `settings_handler_test.go`
3. ✅ Run tests: `go test ./internal/api/handlers -v -run TestSettingsHandler_TestPublicURL`
4. ✅ Verify test passes

**Exit Criteria:** Settings handler test passes

---

### Phase 3: Validation & Sign-Off

1. ✅ Run full test suite: `go test ./...`
2. ✅ Run coverage: `go test -coverprofile=coverage.out ./...`
3. ✅ Verify:
   - All tests passing (FAIL count = 0)
   - Coverage ≥85%
4. ✅ Run pre-commit hooks: `.github/skills/scripts/skill-runner.sh qa-precommit-all`
5. ✅ Generate final QA report
6. ✅ Commit changes with descriptive message

**Exit Criteria:** Clean test run, coverage ≥85%, pre-commit passes

---

## Acceptance Criteria

- [x] **All tests must pass** (0 failures)
- [x] **Coverage must be ≥85%** (preferably ≥85.5% for buffer)
- [x] **SSRF protection must remain intact** (no security regression)
- [x] **Pre-commit hooks must pass** (linting, formatting)
- [x] **No changes to production code security logic** (only test code modified)

---

## Risk Assessment

### Low Risk Items ✅

- Adding unit tests for `url.go` (no production code changes)
- Mock transport in `url_connectivity_test.go` (test-only changes)
- Quick fix for settings handler test (minimal change)

### Medium Risk Items ⚠️

- Dependency injection refactoring in settings handler (if chosen)
  - **Mitigation:** Test thoroughly, use quick fix as backup

### High Risk Items 🔴

- None identified

### Security Considerations

- **CRITICAL:** Do NOT add localhost to SSRF allowlist
- **CRITICAL:** Do NOT disable SSRF checks in production code
- **CRITICAL:** Mock transport must only be available in test builds
- All fixes target test code, not production security logic

---

## Alternative Approaches Considered

### ❌ Approach 1: Add test allowlist to SSRF protection

**Why Rejected:** Weakens security, could leak to production

### ❌ Approach 2: Use build tags to disable SSRF in tests

**Why Rejected:** Too risky, could accidentally disable in production

### ❌ Approach 3: Skip failing tests temporarily

**Why Rejected:** Violates project standards, hides real issues

### ✅ Approach 4: Mock HTTP transport (SELECTED)

**Why Selected:** Industry standard, no security impact, clean separation of concerns

---

## Dependencies & Blockers

**Dependencies:**

- None (all work can proceed immediately)

**Potential Blockers:**

- If `url.go` has fewer functions than expected, may need to add tests to `services` package
  - **Mitigation:** Identified backup options in Section 2.2

---

## Testing Strategy

### Unit Tests

- ✅ All new tests must follow existing patterns in `*_test.go` files
- ✅ Use table-driven tests for multiple scenarios
- ✅ Mock external dependencies (HTTP, DNS)

### Integration Tests

- ⚠️ No integration test changes required (SSRF tests pass)
- ✅ Verify SSRF protection still blocks localhost in production

### Regression Tests

- ✅ Run full suite before and after changes
- ✅ Compare coverage reports to ensure no decrease
- ✅ Verify no new failures introduced

---

## Rollback Plan

If any issues arise during implementation:

1. **Revert to last known good state:**

   ```bash
   git checkout HEAD -- backend/internal/utils/
   git checkout HEAD -- backend/internal/api/handlers/settings_handler_test.go
   ```

2. **Re-run tests to confirm stability:**

   ```bash
   go test ./...
   ```

3. **Escalate to team lead** if issues persist

---

## Success Metrics

### Before Remediation

- Coverage: 84.7%
- Test Failures: 21
- Failing Test Scenarios: 7
- QA Status: ⚠️ CONDITIONAL PASS

### After Remediation

- Coverage: ≥85.5%
- Test Failures: 0
- Failing Test Scenarios: 0
- QA Status: ✅ PASS

---

## Deliverables

1. **Code Changes:**
   - `backend/internal/utils/url_testing.go` (add transport parameter)
   - `backend/internal/utils/url_connectivity_test.go` (mock transport)
   - `backend/internal/utils/url_test.go` (new tests for url.go)
   - `backend/internal/api/handlers/settings_handler_test.go` (fix failing test)

2. **Documentation:**
   - Updated test documentation explaining mock transport pattern
   - Code comments in test files explaining SSRF protection handling

3. **Reports:**
   - Final QA report with updated metrics
   - Coverage report showing ≥85%

---

## Post-Remediation Tasks

### Immediate (Before Merge)

- [ ] Update QA report with final metrics
- [ ] Update PR description with remediation summary
- [ ] Request final code review

### Short-Term (Post-Merge)

- [ ] Document mock transport pattern in testing guidelines
- [ ] Add linter rule to prevent `httptest.NewServer()` in SSRF-tested code paths
- [ ] Create tech debt ticket for settings handler DI refactoring (if using quick fix)

### Long-Term

- [ ] Evaluate test coverage targets for individual packages
- [ ] Consider increasing global coverage target to 90%
- [ ] Add automated coverage regression detection to CI

---

## Timeline

**Day 1 (4-6 hours):**

- Hour 1-2: Increase coverage in `internal/utils`
- Hour 3-4: Fix URL connectivity tests (mock transport)
- Hour 5: Fix settings handler test
- Hour 6: Final validation and QA report

**Estimated Completion:** Same day (December 23, 2025)

---

## Questions & Answers

**Q: Why not just disable SSRF protection for tests?**
A: Would create security vulnerability risk and violate secure coding standards.

**Q: Can we use a real localhost exception just for tests?**
A: No. Build-time exceptions can leak to production and create attack vectors.

**Q: Why mock transport instead of using a test container?**
A: Faster, no external dependencies, standard Go testing practice.

**Q: What if coverage doesn't reach 85% after utils tests?**
A: Backup plan documented in Section 2.2 (services or seed package).

---

## Approvals

**Prepared By:** GitHub Copilot (Automated QA Agent)
**Reviewed By:** [Pending]
**Approved By:** [Pending]

**Status:** 🟡 DRAFT - Awaiting Implementation

---

**Document Version:** 1.0
**Last Updated:** December 23, 2025
**Location:** `/projects/Charon/docs/plans/qa_remediation.md`
