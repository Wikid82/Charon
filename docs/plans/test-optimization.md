# Test Optimization Implementation Plan

> **Created:** January 3, 2026
> **Status:** ✅ Phase 4 Complete - Ready for Production
> **Estimated Impact:** 40-60% reduction in test execution time
> **Actual Impact:** ~12% immediate reduction with `-short` mode

## Executive Summary

This plan outlines a four-phase approach to optimize the Charon backend test suite:

1. ✅ **Phase 1:** Replace `go test` with `gotestsum` for real-time progress visibility
2. ⏳ **Phase 2:** Add `t.Parallel()` to eligible test functions for concurrent execution
3. ⏳ **Phase 3:** Optimize database-heavy tests using transaction rollbacks
4. ✅ **Phase 4:** Implement `-short` mode for quick feedback loops

---

## Implementation Status

### Phase 4: `-short` Mode Support ✅ COMPLETE

**Completed:** January 3, 2026

**Results:**

- ✅ 21 tests now skip in short mode (7 integration + 14 heavy network)
- ✅ ~12% reduction in test execution time
- ✅ New VS Code task: "Test: Backend Unit (Quick)"
- ✅ Environment variable support: `CHARON_TEST_SHORT=true`
- ✅ All integration tests properly gated
- ✅ Heavy HTTP/network tests identified and skipped

**Files Modified:** 10 files

- 6 integration test files
- 2 heavy unit test files
- 1 tasks.json update
- 1 skill script update

**Documentation:** [PHASE4_SHORT_MODE_COMPLETE.md](../implementation/PHASE4_SHORT_MODE_COMPLETE.md)

---

## Analysis Summary

| Metric | Count |
|--------|-------|
| **Total test files analyzed** | 191 |
| **Backend internal test files** | 182 |
| **Integration test files** | 7 |
| **Tests already using `t.Parallel()`** | ~200+ test functions |
| **Tests needing parallelization** | ~300+ test functions |
| **Database-heavy test files** | 35+ |
| **Tests with `-short` support** | 2 (currently) |

---

## Phase 1: Infrastructure (gotestsum)

### Objective

Replace raw `go test` output with `gotestsum` for:

- Real-time test progress with pass/fail indicators
- Better failure summaries
- JUnit XML output for CI integration
- Colored output for local development

### Changes Required

#### 1.1 Install gotestsum as Development Dependency

```bash
# Add to Makefile or development setup
go install gotest.tools/gotestsum@latest
```

**File:** `Makefile`

```makefile
# Add to tools target
.PHONY: install-tools
install-tools:
 go install gotest.tools/gotestsum@latest
```

#### 1.2 Update Backend Test Skill Scripts

**File:** `.github/skills/test-backend-unit-scripts/run.sh`

Replace:

```bash
if go test "$@" ./...; then
```

With:

```bash
# Check if gotestsum is available, fallback to go test
if command -v gotestsum &> /dev/null; then
    if gotestsum --format pkgname -- "$@" ./...; then
        log_success "Backend unit tests passed"
        exit 0
    else
        exit_code=$?
        log_error "Backend unit tests failed (exit code: ${exit_code})"
        exit "${exit_code}"
    fi
else
    log_warn "gotestsum not found, falling back to go test"
    if go test "$@" ./...; then
```

**File:** `.github/skills/test-backend-coverage-scripts/run.sh`

Update the legacy script call to use gotestsum when available.

#### 1.3 Update VS Code Tasks (Optional Enhancement)

**File:** `.vscode/tasks.json`

Add new task for verbose test output:

```jsonc
{
    "label": "Test: Backend Unit (Verbose)",
    "type": "shell",
    "command": "cd backend && gotestsum --format testdox ./...",
    "group": "test",
    "problemMatcher": []
}
```

#### 1.4 Update scripts/go-test-coverage.sh

**File:** `scripts/go-test-coverage.sh` (Line 42)

Replace:

```bash
if ! go test -race -v -mod=readonly -coverprofile="$COVERAGE_FILE" ./...; then
```

With:

```bash
if command -v gotestsum &> /dev/null; then
    if ! gotestsum --format pkgname -- -race -mod=readonly -coverprofile="$COVERAGE_FILE" ./...; then
        GO_TEST_STATUS=$?
    fi
else
    if ! go test -race -v -mod=readonly -coverprofile="$COVERAGE_FILE" ./...; then
        GO_TEST_STATUS=$?
    fi
fi
```

---

## Phase 2: Parallelism (t.Parallel)

### Objective

Add `t.Parallel()` to test functions that can safely run concurrently.

### 2.1 Files Already Using t.Parallel() ✅

These files are already well-parallelized:

| File | Parallel Tests |
|------|---------------|
| `internal/services/log_watcher_test.go` | 30+ tests |
| `internal/api/handlers/auth_handler_test.go` | 35+ tests |
| `internal/api/handlers/crowdsec_handler_test.go` | 40+ tests |
| `internal/api/handlers/proxy_host_handler_test.go` | 50+ tests |
| `internal/api/handlers/proxy_host_handler_update_test.go` | 15+ tests |
| `internal/api/handlers/handlers_test.go` | 11 tests |
| `internal/api/handlers/testdb_test.go` | 2 tests |
| `internal/api/handlers/security_notifications_test.go` | 10 tests |
| `internal/api/handlers/cerberus_logs_ws_test.go` | 9 tests |
| `internal/services/backup_service_disk_test.go` | 3 tests |

### 2.2 Files Needing t.Parallel() Addition

**Priority 1: High-impact files (many tests, no shared state)**

| File | Est. Tests | Pattern |
|------|-----------|---------|
| `internal/network/safeclient_test.go` | 30+ | Add to all `func Test*` |
| `internal/network/internal_service_client_test.go` | 9 | Add to all `func Test*` |
| `internal/security/url_validator_test.go` | 25+ | Add to all `func Test*` |
| `internal/security/audit_logger_test.go` | 10+ | Add to all `func Test*` |
| `internal/metrics/security_metrics_test.go` | 5 | Add to all `func Test*` |
| `internal/metrics/metrics_test.go` | 2 | Add to all `func Test*` |
| `internal/crowdsec/hub_cache_test.go` | 18 | Add to all `func Test*` |
| `internal/crowdsec/hub_sync_test.go` | 30+ | Add to all `func Test*` |
| `internal/crowdsec/presets_test.go` | 4 | Add to all `func Test*` |

**Priority 2: Medium-impact files**

| File | Est. Tests | Notes |
|------|-----------|-------|
| `internal/cerberus/cerberus_test.go` | 10+ | Uses shared DB setup |
| `internal/cerberus/cerberus_isenabled_test.go` | 10+ | Uses shared DB setup |
| `internal/cerberus/cerberus_middleware_test.go` | 8 | Uses shared DB setup |
| `internal/config/config_test.go` | 10+ | Uses env vars - CANNOT parallelize |
| `internal/database/database_test.go` | 7 | Uses file system |
| `internal/database/errors_test.go` | 6 | Uses file system |
| `internal/util/sanitize_test.go` | 1 | Simple, can parallelize |
| `internal/util/crypto_test.go` | 2 | Simple, can parallelize |
| `internal/version/version_test.go` | ~2 | Simple, can parallelize |

**Priority 3: Handler tests (many already parallelized)**

| File | Status |
|------|--------|
| `internal/api/handlers/notification_handler_test.go` | Needs review |
| `internal/api/handlers/certificate_handler_test.go` | Needs review |
| `internal/api/handlers/backup_handler_test.go` | Needs review |
| `internal/api/handlers/user_handler_test.go` | Needs review |
| `internal/api/handlers/settings_handler_test.go` | Needs review |
| `internal/api/handlers/domain_handler_test.go` | Needs review |

### 2.3 Tests That CANNOT Be Parallelized

**Environment Variable Tests:**

- `internal/config/config_test.go` - Uses `os.Setenv()` which affects global state

**Singleton/Global State Tests:**

- `internal/api/handlers/testdb_test.go::TestGetTemplateDB` - Tests singleton pattern
- Any test using global metrics registration

**Sequential Dependency Tests:**

- Integration tests in `backend/integration/` - Require Docker container state

### 2.4 Table-Driven Test Pattern Fix

For table-driven tests, ensure loop variable capture:

```go
// BEFORE (race condition in parallel)
for _, tc := range testCases {
    t.Run(tc.name, func(t *testing.T) {
        t.Parallel()
        // tc may have changed
    })
}

// AFTER (safe for parallel)
for _, tc := range testCases {
    tc := tc // capture loop variable
    t.Run(tc.name, func(t *testing.T) {
        t.Parallel()
        // tc is safely captured
    })
}
```

**Files needing this pattern (search for `for.*range.*testCases`):**

- `internal/security/url_validator_test.go`
- `internal/network/safeclient_test.go`
- `internal/crowdsec/hub_sync_test.go`

---

## Phase 3: Database Optimization

### Objective

Replace full database setup/teardown with transaction rollbacks for faster test isolation.

### 3.1 Current Database Test Pattern

**File:** `internal/api/handlers/testdb_test.go`

Current helper functions:

- `GetTemplateDB()` - Singleton template database
- `OpenTestDB(t)` - Creates new in-memory SQLite per test
- `OpenTestDBWithMigrations(t)` - Creates DB with full schema

### 3.2 Files Using Database Setup

| File | Pattern | Optimization |
|------|---------|--------------|
| `internal/cerberus/cerberus_test.go` | `setupTestDB(t)` / `setupFullTestDB(t)` | Transaction rollback |
| `internal/cerberus/cerberus_isenabled_test.go` | `setupDBForTest(t)` | Transaction rollback |
| `internal/cerberus/cerberus_middleware_test.go` | `setupDB(t)` | Transaction rollback |
| `internal/crowdsec/console_enroll_test.go` | `openConsoleTestDB(t)` | Transaction rollback |
| `internal/utils/url_test.go` | `setupTestDB(t)` | Transaction rollback |
| `internal/services/backup_service_test.go` | File-based setup | Keep as-is (file I/O) |
| `internal/database/database_test.go` | Direct DB tests | Keep as-is (testing DB layer) |

### 3.3 Proposed Transaction Rollback Helper

**New File:** `internal/testutil/db.go`

```go
package testutil

import (
    "testing"
    "gorm.io/gorm"
)

// WithTx runs a test function within a transaction that is always rolled back.
// This provides test isolation without the overhead of creating new databases.
func WithTx(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB)) {
    t.Helper()
    tx := db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
        tx.Rollback()
    }()
    fn(tx)
}

// GetTestTx returns a transaction that will be rolled back when the test completes.
func GetTestTx(t *testing.T, db *gorm.DB) *gorm.DB {
    t.Helper()
    tx := db.Begin()
    t.Cleanup(func() {
        tx.Rollback()
    })
    return tx
}
```

### 3.4 Migration Pattern

**Before:**

```go
func TestSomething(t *testing.T) {
    db := setupTestDB(t) // Creates new in-memory DB
    db.Create(&models.Setting{Key: "test", Value: "value"})
    // ... test logic
}
```

**After:**

```go
var sharedTestDB *gorm.DB
var once sync.Once

func getSharedDB(t *testing.T) *gorm.DB {
    once.Do(func() {
        sharedTestDB = setupTestDB(t)
    })
    return sharedTestDB
}

func TestSomething(t *testing.T) {
    t.Parallel()
    tx := testutil.GetTestTx(t, getSharedDB(t))
    tx.Create(&models.Setting{Key: "test", Value: "value"})
    // ... test logic using tx instead of db
}
```

---

## Phase 4: Short Mode

### Objective

Enable fast feedback with `-short` flag by skipping heavy integration tests.

### 4.1 Current Short Mode Usage

Only 2 tests currently support `-short`:

| File | Test |
|------|------|
| `internal/utils/url_connectivity_test.go` | Comprehensive SSRF test |
| `internal/services/mail_service_test.go` | SMTP integration test |

### 4.2 Tests to Add Short Mode Skip

**Integration Tests (all in `backend/integration/`):**

```go
func TestCrowdsecStartup(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    // ... existing test
}
```

Apply to:

- `crowdsec_decisions_integration_test.go` - Both tests
- `crowdsec_integration_test.go`
- `coraza_integration_test.go`
- `cerberus_integration_test.go`
- `waf_integration_test.go`
- `rate_limit_integration_test.go`

**Heavy Unit Tests:**

| File | Tests to Skip | Reason |
|------|--------------|--------|
| `internal/crowdsec/hub_sync_test.go` | HTTP-based tests | Network I/O |
| `internal/network/safeclient_test.go` | `TestNewSafeHTTPClient_*` | Network I/O |
| `internal/services/mail_service_test.go` | All | SMTP connection |
| `internal/api/handlers/crowdsec_pull_apply_integration_test.go` | All | External deps |

### 4.3 Update VS Code Tasks

**File:** `.vscode/tasks.json`

Add quick test task:

```jsonc
{
    "label": "Test: Backend Unit (Quick)",
    "type": "shell",
    "command": "cd backend && gotestsum --format pkgname -- -short ./...",
    "group": "test",
    "problemMatcher": []
}
```

### 4.4 Update Skill Scripts

**File:** `.github/skills/test-backend-unit-scripts/run.sh`

Add `-short` support via environment variable:

```bash
SHORT_FLAG=""
if [[ "${CHARON_TEST_SHORT:-false}" == "true" ]]; then
    SHORT_FLAG="-short"
    log_info "Running in short mode (skipping integration tests)"
fi

if gotestsum --format pkgname -- $SHORT_FLAG "$@" ./...; then
```

---

## Implementation Order

### Week 1: Phase 1 (gotestsum)

1. Install gotestsum in development environment
2. Update skill scripts with gotestsum support
3. Update legacy scripts
4. Verify CI compatibility

### Week 2: Phase 2 (t.Parallel)

1. Add `t.Parallel()` to Priority 1 files (network, security, metrics)
2. Add `t.Parallel()` to Priority 2 files (cerberus, database)
3. Fix table-driven test patterns
4. Run race detector to verify no issues

### Week 3: Phase 3 (Database)

1. Create `internal/testutil/db.go` helper
2. Migrate cerberus tests to transaction pattern
3. Migrate crowdsec tests to transaction pattern
4. Benchmark before/after

### Week 4: Phase 4 (Short Mode)

1. Add `-short` skips to integration tests
2. Add `-short` skips to heavy unit tests
3. Update VS Code tasks
4. Document usage in CONTRIBUTING.md

---

## Expected Impact

| Metric | Current | After Phase 1 | After Phase 2 | After Phase 4 |
|--------|---------|--------------|--------------|--------------|
| **Test visibility** | None | Real-time | Real-time | Real-time |
| **Parallel execution** | ~30% | ~30% | ~70% | ~70% |
| **Full suite time** | ~90s | ~85s | ~50s | ~50s |
| **Quick feedback** | N/A | N/A | N/A | ~15s |

---

## Validation Checklist

- [ ] All tests pass with `go test -race ./...`
- [ ] Coverage remains above 85% threshold
- [ ] No new race conditions detected
- [ ] gotestsum output is readable in CI logs
- [ ] `-short` mode completes in under 20 seconds
- [ ] Transaction rollback tests properly isolate data

---

## Files Changed Summary

| Phase | Files Modified | Files Created |
|-------|---------------|---------------|
| Phase 1 | 4 | 0 |
| Phase 2 | ~40 | 0 |
| Phase 3 | ~10 | 1 |
| Phase 4 | ~15 | 0 |

---

## Rollback Plan

If any phase causes issues:

1. Phase 1: Remove gotestsum wrapper, revert to `go test`
2. Phase 2: Remove `t.Parallel()` calls (can be done file-by-file)
3. Phase 3: Revert to per-test database creation
4. Phase 4: Remove `-short` skips

All changes are additive and backward-compatible.
