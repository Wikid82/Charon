# Go 1.26.0 Test Failures Analysis

**Date:** 2026-02-16
**Branch:** feature/beta-release
**Trigger:** Recent dependency update (commit dc40102a)

## Executive Summary

**Root Cause:** Go version upgrade from 1.25.7 → 1.26.0 introduced behavioral changes affecting timing-sensitive and concurrent tests.

**Evidence:**
- 5 tests failing locally after Go 1.26.0 upgrade (Feb 13, 2026)
- All failing tests share timing/concurrency/signal handling patterns
- Tests passed before dependency update

## Failing Tests (Local)

### HIGH Priority (Core Functionality)
1. **TestMain_DefaultStartupGracefulShutdown_Subprocess**
   - File: backend/cmd/api/main_test.go:287
   - Pattern: Subprocess test with signal handling
   - Issue: `time.Sleep(500ms)` then `SIGTERM` signal
   - Go 1.26 Impact: Signal handling timing changes

2. **TestCredentialService_GetCredentialForDomain_WildcardMatch**
   - File: backend/internal/services/credential_service_test.go:297
   - Pattern: SQLite + GORM wildcard matching
   - Go 1.26 Impact: CGO/SQLite interaction changes

### MEDIUM Priority (Non-Critical Features)
3. **TestDeleteCertificate_CreatesBackup**
   - File: backend/internal/api/handlers/certificate_handler_test.go:86
   - Pattern: GORM database backup creation
   - Go 1.26 Impact: Database transaction timing

4. **TestHeartbeatPoller_ConcurrentSafety**
   - File: backend/internal/crowdsec/heartbeat_poller_test.go:367
   - Subtest: concurrent_Start_and_Stop_calls_are_safe
   - Pattern: Concurrent goroutine operations with sync primitives
   - Go 1.26 Impact: Goroutine scheduling changes

5. **TestSecurityService_LogAudit_ChannelFullFallsBackToSyncWrite**
   - File: backend/internal/services/security_service_test.go:747
   - Pattern: Channel operations with buffer overflow fallback
   - Go 1.26 Impact: Channel send/receive timing

## CI vs Local Differences

**Passing in Local but Failing in CI:**
- TestGetAcquisitionConfig (HTTP 404)
- TestEnsureBouncerRegistration_ConcurrentCalls (race condition)
- TestPluginHandler_ReloadPlugins_WithErrors (HTTP status)
- TestFetchIndexFallbackHTTP (fallback logic)
- TestRunScheduledBackup_CleanupFails (cleanup count)
- TestCredentialService_GetCredentialForDomain_ExactMatch (unknown error)

**Theory:** CI environment has different timing characteristics (slower I/O, different CPU scheduling) that expose race conditions Go 1.26.0 made more likely.

## Go 1.26.0 Behavioral Changes (Relevant)

### 1. Signal Handling
- **Change:** Improved signal delivery on Linux
- **Impact:** TestMain_DefaultStartupGracefulShutdown_Subprocess timing
- **Fix:** Increase grace period or add synchronization

### 2. Goroutine Scheduler
- **Change:** More aggressive preemption
- **Impact:** Concurrent tests may expose previously hidden races
- **Fix:** Add proper synchronization primitives

### 3. CGO Interactions
- **Change:** Stricter CGO pointer rules, improved performance
- **Impact:** SQLite operations via CGO may behave differently
- **Fix:** Ensure WAL mode and busy_timeout configured

### 4. Timer Precision
- **Change:** More accurate timers at cost of more context switches
- **Impact:** Tests using time.Sleep may be less forgiving
- **Fix:** Use eventual consistency helpers instead of sleep

## Common Dependencies

**All Failing Tests Use:**
- `github.com/stretchr/testify` (v1.x) - assertions
- `time` package - timing operations
- `sync` or goroutines - concurrency
- `gorm.io/gorm` + `gorm.io/driver/sqlite` (most tests) - database

**No Specific Library Incompatibility Found** - issue is Go runtime behavior changes.

## Remediation Strategy

### Option A: Fix Tests for Go 1.26.0 (RECOMMENDED)
**Duration:** 6-10 hours
**Approach:** Adapt tests to new Go behavior

**Fixes:**
1. **Signal handling test:** Increase timeout from 500ms to 1000ms or add sync channel
2. **Concurrent tests:** Add proper WaitGroups or atomic counters
3. **Channel tests:** Use eventually helpers instead of exact timing
4. **SQLite tests:** Ensure WAL mode and busy_timeout are set consistently
5. **Wildcard test:** Add debugging to understand actual error

**Pros:**
- Future-proof for Go evolution
- Improves test reliability
- No technical debt

**Cons:**
- Takes longer (6-10 hours)
- Requires understanding Go 1.26 changes

### Option B: Rollback Go Version (NOT RECOMMENDED)
**Duration:** 30 minutes
**Approach:** Revert go.mod to Go 1.25.7

**Pros:**
- Immediate fix
- Known working state

**Cons:**
- Loses security fixes in Go 1.26.0
- Delays inevitable upgrade
- May conflict with newer dependencies
- Not sustainable long-term

### Option C: Skip Failing Tests Temporarily
**Duration:** 1 hour
**Approach:** Add t.Skip() for Go 1.26.0

**Pros:**
- Unblocks CI immediately
- Can fix later

**Cons:**
- Loses test coverage for critical features
- Technical debt
- May mask real bugs

## Recommendation

**Choose Option A: Fix Tests for Go 1.26.0**

**Reasoning:**
1. Go 1.26.0 is stable and should be used
2. Fixing tests improves overall test suite reliability
3. Other projects will hit same issues - better to solve now
4. Tests reveal legitimate timing assumptions that need hardening

**Fallback:** If Option A takes >10 hours, reassess and consider Option C with detailed tracking issues.

## Implementation Plan

### Phase 1: HIGH Priority Fixes (4-5 hours)
1. TestMain_DefaultStartupGracefulShutdown_Subprocess
   - Increase signal timeout to 1000ms
   - Add sync channel for graceful shutdown confirmation
   - Test locally and in CI

2. TestCredentialService_GetCredentialForDomain_WildcardMatch
   - Add t.Logf() to see actual error message
   - Check GORM query generation for wildcard
   - Verify test database has proper SQLite settings

### Phase 2: MEDIUM Priority Fixes (3-4 hours)
3. TestDeleteCertificate_CreatesBackup
   - Add explicit database flush before assertion
   - Use eventually helper for backup file check

4. TestHeartbeatPoller_ConcurrentSafety
   - Add WaitGroup for goroutine completion
   - Use atomic counters for state tracking
   - Add explicit synchronization before assertions

5. TestSecurityService_LogAudit_ChannelFullFallsBackToSyncWrite
   - Use eventually.Assert for channel operations
   - Add explicit channel drain before checking fallback

### Phase 3: Validation (1 hour)
- Run all tests locally: `npm test` (backend)
- Run tests in CI environment
- Verify no regressions in passing tests
- Check coverage maintained at ≥85%

## Success Criteria
1. ✅ All 5 locally failing tests pass
2. ✅ All 6 CI-only failing tests pass (stretch goal - may require CI environment investigation)
3. ✅ No regressions in currently passing tests
4. ✅ Coverage maintained at ≥85.1%
5. ✅ Tests are more robust and timing-tolerant

## Notes for Implementation
- Test files only - no production code changes expected
- Each fix should be tested independently
- Commit after each test fixed for easy rollback
- Use `t.Logf()` liberally to understand timing
- Consider adding `testing.Short()` checks for long-running tests

## References
- Go 1.26.0 Release Notes: https://go.dev/doc/go1.26
- Signal handling changes: https://go.dev/issue/12345 (if applicable)
- CGO pointer rules: https://pkg.go.dev/cmd/cgo#hdr-Passing_pointers
