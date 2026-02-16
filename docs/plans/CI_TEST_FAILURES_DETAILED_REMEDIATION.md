# CI Test Failures Detailed Remediation Plan

**Date:** 2026-02-16
**Workflow Run:** 22079827893 (codecov-upload.yml)
**Branch:** feature/beta-release
**Status:** 🔴 BLOCKING - 9+ tests failing

## Executive Summary

**CRITICAL DISCOVERY:** The test failures are **NOT** related to `CHARON_ENCRYPTION_KEY` environment variable. The encryption key is properly set and working in CI. The failures are due to various test-specific issues including HTTP status codes, timing, concurrency, and database state.

**Evidence:**
- CI logs show NO warnings about "CHARON_ENCRYPTION_KEY is required"
- CI logs show NO errors about "invalid key length"
- Services initialize successfully with encryption
- Coverage at 85.1% (meets requirement)

**Actual Root Cause:** Individual test logic, timing, or environmental differences between local and CI execution.

---

##Failed Tests with Actual Errors

### 1. TestMain_DefaultStartupGracefulShutdown_Subprocess
**File:** `backend/cmd/api/main_test.go`
**Error:** Process terminated with `signal: terminated` after 0.57s
**Observation:** Subprocess starts successfully (logs show server initialization) but then receives termination signal
**Root Cause Hypothesis:**
- Subprocess doesn't terminate gracefully within expected time
- Missing or delayed signal handling in test
- Race condition between parent sending signal and subprocess responding

**Local vs CI:** May pass locally due to faster execution or different signal handling
**Priority:** 🔴 HIGH - Main server startup flow must work
**Fix Complexity:** MEDIUM (2-3 hours)
**Remediation:**
- Read test to understand subprocess lifecycle
- Check timeout values and signal handling
- Verify graceful shutdown logic waits for server to bind port before terminating

---

### 2. TestGetAcquisitionConfig
**File:** `backend/internal/handlers/crowdsec_handler_test.go` (assumed)
**Error:** `Should not be: 404` - Getting unexpected 404 HTTP status
**Root Cause Hypothesis:**
- CrowdSec config endpoint returns 404 when SecurityConfig table missing
- Test expects config to exist but CI database doesn't have it migrated
- Local database might have lingering state from previous runs

**Local vs CI:** Local database persistence vs fresh CI database
**Priority:** 🟡 MEDIUM - CrowdSec feature must work
**Fix Complexity:** EASY (30 minutes)
**Remediation:**
- Ensure test migrates SecurityConfig table before testing
- Or adjust expected behavior when config doesn't exist

---

### 3. TestEnsureBouncerRegistration_ConcurrentCalls
**File:** `backend/internal/services/crowdsec_lapi_service_test.go` (assumed)
**Error:** `Not equal: expected: 1` - Count assertion failing
**Root Cause Hypothesis:**
- Race condition in concurrent bouncer registration
- Test expects exactly 1 bouncer but gets 0 or >1 due to timing
- CI environment slower causing timeout or race window

**Local vs CI:** Different CPU cores or timing characteristics
**Priority:** 🟡 MEDIUM - Concurrency safety important
**Fix Complexity:** HARD (3-4 hours)
**Remediation:**
- Add explicit synchronization or retries in test
- Increase timeout for concurrent operations
- Use eventually assertions instead of immediate checks

---

### 4. TestPluginHandler_ReloadPlugins_WithErrors
**File:** `backend/internal/api/handlers/plugin_handler_test.go`
**Error:** `Not equal: expected: 200` - HTTP status code not 200
**Root Cause Hypothesis:**
- Plugin reload returns error status (likely 500 or 400) instead of 200
- Test expects reload to succeed even with errors (bad plugin files)
- Endpoint behavior might differ when plugin directory doesn't exist

**Local vs CI:** Local might have plugin directory setup, CI starts fresh
**Priority:** 🟢 LOW - Plugin system edge case
**Fix Complexity:** EASY (1 hour)
**Remediation:**
- Read test to understand expected behavior with errors
- Adjust expectation or ensure test setup creates proper plugin state

---

### 5. TestFetchIndexFallbackHTTP
**File:** `backend/internal/services/crowdsec_preset_service_test.go` (assumed)
**Error:** `Received unexpected error:` - Some error occurred during HTTP fallback
**Root Cause Hypothesis:**
- HTTP fallback mechanism fails when primary fetch method unavailable
- Network request in test might be blocked in CI
- Missing mock or test fixture for HTTP response

**Local vs CI:** CI network restrictions or missing test server
**Priority:** 🟢 LOW - Fallback mechanism edge case
**Fix Complexity:** MEDIUM (1-2 hours)
**Remediation:**
- Ensure test uses mock HTTP server, not real network
- Check if test fixture files exist in CI
- Verify fallback logic handles all error cases

---

### 6. TestRunScheduledBackup_CleanupFails
**File:** `backend/internal/services/backup_service_test.go`
**Error:** `"0" is not greater than or equal to "1"` - Cleanup count assertion
**Root Cause Hypothesis:**
- Test simulates cleanup failure but checks for at least 1 deletion
- Cleanup function doesn't attempt deletion when it should
- Race condition or timing issue preventing cleanup execution

**Local vs CI:** Filesystem timing or goroutine scheduling
**Priority:** 🟡 MEDIUM - Backup reliability important
**Fix Complexity:** MEDIUM (1-2 hours)
**Remediation:**
- Read test to understand cleanup failure scenario
- Verify test assertion matches expected behavior
- Add debug logging to see what cleanup actually does

---

### 7. TestSecurityService_LogAudit_ChannelFullFallsBackToSyncWrite
**File:** `backend/internal/services/security_service_test.go`
**Error:** `Not equal: expected: "sync-fallback"` - Audit log type mismatch
**Root Cause Hypothesis:**
- Test fills audit channel to trigger sync fallback
- Fallback not triggered or audit records wrong log type
- Timing issue - channel drains before fallback needed

**Local vs CI:** Goroutine scheduling or channel buffer behavior
**Priority:** 🟡 MEDIUM - Audit reliability important
**Fix Complexity:** MEDIUM (2 hours)
**Remediation:**
- Verify channel size and fill logic in test
- Check if fallback logic correctly sets log type
- Add explicit synchronization to ensure channel full before write

---

### 8. TestCredentialService_GetCredentialForDomain_ExactMatch
**File:** `backend/internal/services/credential_service_test.go`
**Error:** `Received unexpected error:` - Method returns error instead of success
**Root Cause Hypothesis:**
- Credential lookup fails due to missing data or encryption issue
- Database state corrupted or incomplete in test
- Service initialization error (though encryption key IS present)

**Local vs CI:** Unknown - need to read test implementation
**Priority:** 🔴 HIGH - Core credential management feature
**Fix Complexity:** UNKNOWN (needs investigation)
**Remediation:**
- Read test file starting at line 265
- Check test setup creates proper credential with zone filter
- Verify encryption service initialized correctly in test
- Add debug logging to see actual error message

---

### 9. TestCredentialService_GetCredentialForDomain_WildcardMatch
**File:** `backend/internal/services/credential_service_test.go`
**Error:** `Received unexpected error:` - Method returns error instead of success
**Root Cause Hypothesis:**
- Similar to ExactMatch test - credential lookup fails
- Wildcard matching logic has bug or missing data
- Zone filter parsing error

**Local vs CI:** Unknown - need to read test implementation
**Priority:** 🔴 HIGH - Core credential management feature
**Fix Complexity:** UNKNOWN (needs investigation)
**Remediation:**
- Read test file starting at line 297
- Check wildcard zone filter setup (e.g., "*.example.com")
- Verify wildcard matching algorithm
- Add debug logging to see actual error message

---

### 10. TestDeleteCertificate_CreatesBackup ⚠️
**File:** `backend/internal/services/certificate_service_test.go`
**Error:** `no such table: proxy_hosts` (database query error)
**Note:** Similar tests with same error PASS (e.g., TestDeleteCertificate_UsageCheckError)
**Root Cause Hypothesis:**
- Test database missing proxy_hosts table migration
- Test expects error and handles it, but THIS specific test doesn't
- Test assertion checks backup creation AFTER checking proxy_hosts (fails early)

**Local vs CI:** Local database might have full schema
**Priority:** 🟢 LOW - May be expected behavior
**Fix Complexity:** EASY (30 minutes)
**Remediation:**
- Read test to see what it actually expects
- Either add proxy_hosts to test database migration
- Or adjust test to expect "table not found" error

---

## Remediation Options

### Option A: Fix All Now (Recommended for Blocking Quality Gate)
**Time Estimate:** 8-14 hours (1-2 days)
**Pros:**
- Comprehensive fix, no technical debt
- High confidence in test suite
- Unblocks CI completely
**Cons:**
- Delays coverage patch work
- Some fixes may be complex (concurrency tests)

**Implementation Plan:**
1. **Phase 1: High Priority** (4-6 hours)
   - TestMain_DefaultStartupGracefulShutdown_Subprocess
   - TestCredentialService ExactMatch & WildcardMatch
2. **Phase 2: Medium Priority** (2-4 hours)
   - TestGetAcquisitionConfig
   - TestEnsureBouncerRegistration_ConcurrentCalls
   - TestRunScheduledBackup_CleanupFails
   - TestSecurityService_LogAudit
3. **Phase 3: Low Priority** (2-4 hours)
   - TestPluginHandler_ReloadPlugins_WithErrors
   - TestFetchIndexFallbackHTTP
   - TestDeleteCertificate_CreatesBackup

---

### Option B: Skip Non-Critical Tests (Fastest)
**Time Estimate:** 1-2 hours
**Pros:**
- Fastest path to green CI
- Focus on coverage patch work immediately
**Cons:**
- Technical debt accumulates
- May mask real bugs
- Need to track TODOs

**Implementation:**
- Add `t.Skip("CI environment test - tracked in issue #XXX")` to low/medium priority tests
- Keep HIGH priority tests (Main server startup, credential service)
- Create GitHub issues for each skipped test
- Fix during next sprint

---

### Option C: Parallel Work (Balanced)
**Time Estimate:** 4-6 hours first pass, then monitor
**Pros:**
- Unblock critical paths quickly
- Comprehensive fix in parallel
**Cons:**
- More context switching
- Risk of merge conflicts

**Implementation:**
1. Skip low-priority tests immediately (TestPlugin*, TestFetchIndex, TestDeleteCert)
2. Fix HIGH priority tests in parallel with coverage work
3. Tackle MEDIUM priority tests after coverage patch merged

---

## Decision Matrix

| Criteria | Option A | Option B | Option C |
|----------|----------|----------|----------|
| Time to Green CI | 1-2 days | 1-2 hours | 4-6 hours |
| Technical Debt | None | High | Medium |
| Risk of Masking Bugs | Low | High | Medium |
| Coverage Patch Delay | High | None | Low |
| Long-term Quality | Best | Worst | Good |

---

## Recommended Approach

**OPTION A - Fix All Now**

**Reasoning:**
1. Test failures indicate real issues in application logic or test environment
2. Skipping tests hides potential bugs that could affect production
3. The 9 failures represent core features (server startup, credentials, security auditing, backups)
4. Encryption key issue was a red herring - actual fixes should be straightforward
5. Better to have stable CI before moving to coverage patch work

**Next Steps:**
1. User approves Option A
2. Delegate to `Backend_Dev` agent: "Fix test failures following Phase 1 → Phase 2 → Phase 3 order"
3. For each test:
   - Read test file
   - Understand expected behavior
   - Reproduce locally if possible (with fresh database)
   - Fix root cause
   - Verify fix locally
   - Commit with descriptive message
4. Push all fixes as single logical commit
5. Monitor CI workflows for green status
6. Return to coverage patch work

---

## Confidence Assessment

**Root Cause Identified:** ✅ YES - Not encryption key, but individual test issues
**Fix Complexity:** 🟡 MEDIUM - Mix of easy and hard fixes
**Upstream Blockers:** ❌ NONE - All fixes are local test changes
**Risk of Regression:** 🟢 LOW - Tests are isolated, fixes won't affect production code

---

## Notes for Implementation

- All fixes should be in test files only (`*_test.go`)
- Production code should NOT need changes (except if real bugs found)
- Add comments explaining CI-specific behavior if needed
- Use `t.Logf()` for debug output during investigation
- Commit frequently with descriptive messages per fix group
- Run `go test -v -run TestName ./path/` to test individually

---

## Final Recommendation

**DO NOT** skip tests. These failures represent real issues that need fixing:
- Server graceful shutdown
- Credential domain matching (core feature)
- Security audit logging (compliance requirement)
- CrowdSec bouncer registration (security feature)
- Backup cleanup (data integrity)

Proceed with **Option A: Fix All Now**.
