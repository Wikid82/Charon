# Test Isolation Findings - Go 1.26.0

**Date:** 2026-02-16
**Investigation:** Test failures after Go 1.26.0 upgrade
**Status:** Partial fix committed, further investigation required

## Summary

**Root Cause Confirmed:** Go 1.26.0 upgrade (commit dc40102a) changed timing/signal handling/scheduling behavior.

**Key Finding:** All 5 failing tests **PASS individually** but **FAIL in full suite** → Test isolation issue.

## Fixes Completed

### ✅ Fix #1: TestMain_DefaultStartupGracefulShutdown_Subprocess
- **File:** backend/cmd/api/main_test.go:287
- **Change:** Increased SIGTERM timeout from 500ms → 1000ms
- **Commit:** 62740eb5
- **Status:** ✅ PASSING individually
- **Reason:** Go 1.26.0 signal delivery timing changes on Linux

## Tests Status Matrix

| Test | Individual | Full Suite | Priority | Notes |
|------|-----------|------------|----------|-------|
| TestMain_DefaultStartupGracefulShutdown_Subprocess | ✅ PASS | ❓ Unknown | HIGH | Fixed timeout |
| TestCredentialService_GetCredentialForDomain_WildcardMatch | ✅ PASS | ❌ FAIL | HIGH | No code changes needed |
| TestDeleteCertificate_CreatesBackup | ✅ PASS | ❌ FAIL | MEDIUM | No code changes needed |
| TestHeartbeatPoller_ConcurrentSafety | ✅ PASS | ❌ FAIL | MEDIUM | No code changes needed |
| TestSecurityService_LogAudit_ChannelFullFallsBackToSyncWrite | ✅ PASS | ❌ FAIL | MEDIUM | No code changes needed |

## Test Isolation Issue

**Observation:** Tests pass when run individually but fail in full suite execution.

**Likely Causes:**
1. **Global State Pollution:**
   - Tests modifying shared package-level variables
   - Singleton initialization state persisting between tests
   - Environment variables not being properly cleaned up

2. **Database Connection Leaks:**
   - SQLite in-memory databases not properly closed
   - GORM connection pool exhaustion
   - WAL mode journal files persisting

3. **Goroutine Leaks:**
   - Background goroutines from previous tests still running
   - Channels not being closed
   - Context cancellations not propagating

4. **Test Execution Order:**
   - Tests depending on specific execution order
   - Previous test failures leaving system in bad state
   - Resource cleanup in t.Cleanup() not executing due to panics

5. **Race Conditions (Go 1.26.0 Scheduler):**
   - Go 1.26.0's more aggressive preemption exposing hidden races
   - Tests making timing assumptions that no longer hold
   - Concurrent test execution causing interference

## Investigation Blockers

**Current Block:** Full test suite hangs or takes excessive time (>2 minutes).

**Symptoms:**
- `go test ./...` hangs indefinitely or terminates after 120s timeout
- Cannot get full suite results to see which tests are actually failing
- Cannot collect coverage data from full suite run

**Needed:**
- Identify which test(s) are causing the hang
- Isolate hanging test(s) and run rest of suite
- Check for infinite loops or deadlocks in test cleanup

## Next Steps

### Option A: Sequential Investigation (4-6 hours)
1. Run tests package-by-package to identify hanging package
2. Use `-timeout 30s` flag to catch hanging tests quickly
3. Add goroutine leak detection: `go test -race -p 1 ./...`
4. Use `t.Parallel()` marking to understand parallelization issues
5. Add `t.Cleanup()` verification to catch leak sources

### Option B: Quick Workaround (30 minutes)
1. Run tests with `-p 1` (no parallelism) to avoid race conditions
2. Increase timeout: `-timeout 10m`
3. Skip known flaky tests temporarily with `t.Skip("Go 1.26.0 isolation issue")`
4. Create tracking issue for proper fix

### Option C: Rollback Go Version (NOT RECOMMENDED)
- Revert to Go 1.25.7
- Loses security fixes
- Kicks can down road

## Recommendation

**Hybrid Approach:**
1. **Immediate (now):** Run tests with `-p 1 -timeout 5m` to force sequential execution
2. **Short-term (today):** Identify hanging tests and skip with tracking issue
3. **Long-term (this week):** Fix test isolation properly with cleanup audits

**Why:** Unblocks CI immediately while preserving investigation path.

## Commands for Investigation

```bash
# Run sequentially with timeout
go test -p 1 -timeout 5m ./...

# Find hanging test packages
for pkg in $(go list ./...); do
    echo "Testing $pkg..."
    timeout 30s go test -v "$pkg" || echo "FAILED or TIMEOUT: $pkg"
done

# Check for goroutine leaks
go test -race -p 1 -count=1 ./...

# Run specific packages
go test -v ./cmd/... ./internal/api/... ./internal/services/...
```

## Related Documents
- [docs/plans/GO_126_TEST_FAILURES_ANALYSIS.md](./GO_126_TEST_FAILURES_ANALYSIS.md) - Initial analysis
- [docs/plans/CI_TEST_FAILURES_DETAILED_REMEDIATION.md](./CI_TEST_FAILURES_DETAILED_REMEDIATION.md) - CI failures

## Action Items
- [ ] Run tests sequentially (`-p 1`) to check if parallelism is the issue
- [ ] Identify hanging test package
- [ ] Add timeout flags to test execution script
- [ ] Audit all tests for proper t.Cleanup() usage
- [ ] Add goroutine leak detection to CI
- [ ] Create tracking issue for test isolation fixes
