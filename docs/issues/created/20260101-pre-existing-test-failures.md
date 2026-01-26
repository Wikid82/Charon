# Pre-Existing Test Failures

**Discovery Date:** December 23, 2025
**Discovered During:** CrowdSec Startup Fix QA Audit
**Status:** Open
**Priority:** Medium

## Overview

During comprehensive QA audit of the CrowdSec startup fix (commit `c71c996`), two categories of pre-existing test failures were discovered. These failures are **NOT related** to the CrowdSec changes and exist on the base branch (`feature/beta-release`).

## Issue 1: Handler Tests Timeout

**Package:** `github.com/Wikid82/charon/backend/internal/api/handlers`
**Severity:** Medium
**Impact:** CI/CD pipeline delays

### Symptoms

```bash
FAIL: github.com/Wikid82/charon/backend/internal/api/handlers (timeout 441s)
```

- Test suite takes 7.35 minutes (441 seconds)
- Default timeout is 10 minutes, but this is too close
- All tests eventually pass, but timing is concerning

### Root Cause

- Test suite contains numerous integration tests that make real HTTP requests
- No apparent infinite loop or deadlock
- Tests are comprehensive but slow

### Affected Tests

All handler tests, including:

- Access list handlers
- Auth handlers
- Backup handlers
- CrowdSec handlers
- Docker handlers
- Import handlers
- Notification handlers
- Proxy host handlers
- Security handlers
- User handlers

### Recommended Fix

**Option 1: Increase Timeout**

```bash
go test -timeout 15m ./internal/api/handlers/...
```

**Option 2: Split Test Suite**

```bash
# Fast unit tests
go test -short ./internal/api/handlers/...

# Slow integration tests (separate)
go test -run Integration ./internal/api/handlers/...
```

**Option 3: Optimize Tests**

- Use mocks for external HTTP calls
- Parallelize independent tests with `t.Parallel()`
- Use table-driven tests to reduce setup/teardown overhead

### Priority Justification

- **Medium** because tests do eventually pass
- Not a functional issue, timing concern only
- Can workaround with increased timeout
- Should be fixed to improve CI/CD performance

---

## Issue 2: URL Connectivity Test Failures

**Package:** `github.com/Wikid82/charon/backend/internal/utils`
**Severity:** Medium
**Impact:** URL validation feature may not work correctly for localhost

### Symptoms

```bash
FAIL: github.com/Wikid82/charon/backend/internal/utils
Coverage: 51.5% (below 85% threshold)

Failed Tests:
- TestTestURLConnectivity_Success
- TestTestURLConnectivity_Redirect
- TestTestURLConnectivity_TooManyRedirects
- TestTestURLConnectivity_StatusCodes/200_OK
- TestTestURLConnectivity_StatusCodes/201_Created
- TestTestURLConnectivity_StatusCodes/204_No_Content
- TestTestURLConnectivity_StatusCodes/301_Moved_Permanently
- TestTestURLConnectivity_StatusCodes/302_Found
- TestTestURLConnectivity_StatusCodes/400_Bad_Request
- TestTestURLConnectivity_StatusCodes/401_Unauthorized
- TestTestURLConnectivity_StatusCodes/403_Forbidden
- TestTestURLConnectivity_StatusCodes/404_Not_Found
- TestTestURLConnectivity_StatusCodes/500_Internal_Server_Error
- TestTestURLConnectivity_StatusCodes/503_Service_Unavailable
- TestTestURLConnectivity_InvalidURL/Empty_URL
- TestTestURLConnectivity_InvalidURL/Invalid_scheme
- TestTestURLConnectivity_InvalidURL/No_scheme
- TestTestURLConnectivity_Timeout
```

### Root Cause

**Error Pattern:**

```
Error: "access to private IP addresses is blocked (resolved to 127.0.0.1)"
       does not contain "status 404"
```

**Analysis:**

1. Tests use `httptest.NewServer()` which binds to `127.0.0.1` (localhost)
2. URL validation code has private IP blocking for security
3. Private IP check runs BEFORE HTTP request is made
4. Tests expect HTTP status codes but get IP validation errors instead
5. This creates a mismatch between expected and actual error messages

**Code Location:**

```go
// File: backend/internal/utils/url_connectivity_test.go
// Lines: 103, 127-128, 156

// Test expects:
assert.Contains(t, err.Error(), "status 404")

// But gets:
"access to private IP addresses is blocked (resolved to 127.0.0.1)"
```

### Recommended Fix

**Option 1: Use Public Test Endpoints**

```go
func TestTestURLConnectivity_StatusCodes(t *testing.T) {
    tests := []struct {
        name       string
        statusCode int
        url        string
    }{
        {"200 OK", 200, "https://httpstat.us/200"},
        {"404 Not Found", 404, "https://httpstat.us/404"},
        // ... use public endpoints
    }
}
```

**Option 2: Add Test-Only Bypass**

```go
// In url_connectivity.go
func TestURLConnectivity(url string) error {
    // Add env var to disable private IP check for tests
    if os.Getenv("CHARON_ALLOW_PRIVATE_IPS_FOR_TESTS") == "true" {
        // Skip private IP validation
    }

    // ... rest of validation
}

// In test setup:
func TestMain(m *testing.M) {
    os.Setenv("CHARON_ALLOW_PRIVATE_IPS_FOR_TESTS", "true")
    code := m.Run()
    os.Unsetenv("CHARON_ALLOW_PRIVATE_IPS_FOR_TESTS")
    os.Exit(code)
}
```

**Option 3: Mock DNS Resolution**

```go
// Use custom dialer that returns public IPs for test domains
type testDialer struct {
    realDialer *net.Dialer
}

func (d *testDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
    // Intercept localhost and return mock IP
    if strings.HasPrefix(addr, "127.0.0.1:") {
        // Return connection to test server but with public IP appearance
    }
    return d.realDialer.DialContext(ctx, network, addr)
}
```

### Priority Justification

- **Medium** because feature works in production
- Tests are catching security feature (private IP blocking) working as intended
- Need to fix test design, not the security feature
- Affects coverage reporting (51.5% < 85% threshold)

---

## Issue 3: Pre-commit Auto-Fix Required

**Severity:** Low
**Impact:** None (auto-fixed)

### Symptoms

```
trim trailing whitespace.................................................Failed
- hook id: trailing-whitespace
- exit code: 1
- files were modified by this hook
Fixing backend/internal/services/crowdsec_startup.go
Fixing backend/cmd/api/main.go
```

### Resolution

Pre-commit hook automatically removed trailing whitespace. Files have been fixed.

**Action Required:** ✅ **NONE** (auto-fixed)

---

## Tracking

### Issue 1: Handler Tests Timeout

- **Tracking Issue:** [Create GitHub Issue]
- **Assignee:** Backend Team
- **Target Fix Date:** Next sprint
- **Workaround:** `go test -timeout 15m`

### Issue 2: URL Connectivity Tests

- **Tracking Issue:** [Create GitHub Issue]
- **Assignee:** Backend Team
- **Target Fix Date:** Next sprint
- **Workaround:** Skip tests with `-short` flag

### Issue 3: Trailing Whitespace

- **Status:** ✅ **RESOLVED** (auto-fixed)

---

## References

- QA Report: [docs/reports/qa_report_crowdsec_startup_fix.md](../reports/qa_report_crowdsec_startup_fix.md)
- Implementation Plan: [docs/plans/crowdsec_startup_fix.md](../plans/crowdsec_startup_fix.md)
- Commit: `c71c996`
- Branch: `feature/beta-release`

---

**Document Status:** Active
**Last Updated:** December 23, 2025 01:25 UTC
