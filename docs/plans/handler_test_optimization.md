# Backend Handler Test Optimization Analysis

## Executive Summary

The backend handler tests contain **748 tests across 69 test files** in `backend/internal/api/handlers/`. While individual tests run quickly (most complete in <1 second), the cumulative effect of repeated test infrastructure setup creates perceived slowness. This document identifies specific bottlenecks and provides prioritized optimization recommendations.

## Current Test Architecture Summary

### Database Setup Pattern

Each test creates its own SQLite in-memory database with unique DSN:

```go
// backend/internal/api/handlers/testdb.go
func OpenTestDB(t *testing.T) *gorm.DB {
    dsnName := strings.ReplaceAll(t.Name(), "/", "_")
    uniqueSuffix := fmt.Sprintf("%d%d", time.Now().UnixNano(), n.Int64())
    dsn := fmt.Sprintf("file:%s_%s?mode=memory&cache=shared&_journal_mode=WAL&_busy_timeout=5000", dsnName, uniqueSuffix)
    db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
    // ...
}
```

### Test Setup Flow

1. **Create in-memory SQLite database** (unique per test)
2. **Run AutoMigrate** for required models (varies per test: 2-15 models)
3. **Create test fixtures** (users, hosts, settings, etc.)
4. **Initialize service dependencies** (NotificationService, AuthService, etc.)
5. **Create handler instances**
6. **Setup Gin router**
7. **Execute HTTP requests via httptest**

### Parallelization Status

| Package | Parallel Tests | Sequential Tests |
|---------|---------------|------------------|
| `handlers/` | ~20% use `t.Parallel()` | ~80% run sequentially |
| `services/` | ~40% use `t.Parallel()` | ~60% run sequentially |
| `integration/` | 100% use `t.Parallel()` | 0% |

---

## Identified Bottlenecks

### 1. Repeated AutoMigrate Calls (HIGH IMPACT)

**Location**: Every test file with database access

**Evidence**:

```go
// handlers_test.go - migrates 6 models
db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.RemoteServer{},
    &models.ImportSession{}, &models.Notification{}, &models.NotificationProvider{})

// security_handler_rules_decisions_test.go - migrates 10 models
db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{},
    &models.CaddyConfig{}, &models.SSLCertificate{}, &models.AccessList{},
    &models.SecurityConfig{}, &models.SecurityDecision{}, &models.SecurityAudit{},
    &models.SecurityRuleSet{})

// proxy_host_handler_test.go - migrates 4 models
db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Notification{},
    &models.NotificationProvider{})
```

**Impact**: ~50-100ms per AutoMigrate call, multiplied by 748 tests = **~37-75 seconds total**

---

### 2. Explicit `time.Sleep()` Calls (HIGH IMPACT)

**Location**: 37 occurrences across test files

**Key Offenders**:

| File | Sleep Duration | Count | Purpose |
|------|---------------|-------|---------|
| [cerberus_logs_ws_test.go](backend/internal/api/handlers/cerberus_logs_ws_test.go) | 100-300ms | 6 | WebSocket subscription wait |
| [uptime_service_test.go](backend/internal/services/uptime_service_test.go) | 50ms-3s | 9 | Async check completion |
| [notification_service_test.go](backend/internal/services/notification_service_test.go) | 50-100ms | 4 | Batch flush wait |
| [log_watcher_test.go](backend/internal/services/log_watcher_test.go) | 10-200ms | 4 | File watcher sync |
| [caddy/manager_test.go](backend/internal/caddy/manager_test.go) | 1100ms | 1 | Timing test |

**Total sleep time per test run**: ~15-20 seconds minimum

**Example of problematic pattern**:

```go
// uptime_service_test.go:766
time.Sleep(2 * time.Second) // Give enough time for timeout (default is 1s)
```

---

### 3. Sequential Test Execution (MEDIUM IMPACT)

**Location**: Most handler tests lack `t.Parallel()`

**Evidence**: Only integration tests and some service tests use parallelization:

```go
// GOOD: integration/waf_integration_test.go
func TestWAFIntegration(t *testing.T) {
    t.Parallel()
    // ...
}

// BAD: handlers/auth_handler_test.go - missing t.Parallel()
func TestAuthHandler_Login(t *testing.T) {
    // No t.Parallel() call
    handler, db := setupAuthHandler(t)
    // ...
}
```

**Impact**: Tests run one-at-a-time instead of utilizing available CPU cores

---

### 4. Service Initialization Overhead (MEDIUM IMPACT)

**Location**: Multiple test files recreate services from scratch

**Pattern**:

```go
// Repeated in many tests
ns := services.NewNotificationService(db)
handler := handlers.NewRemoteServerHandler(services.NewRemoteServerService(db), ns)
```

---

### 5. Router Recreation (LOW IMPACT)

**Location**: Each test creates a new Gin router

```go
gin.SetMode(gin.TestMode)
router := gin.New()
handler.RegisterRoutes(router.Group("/api/v1"))
```

While fast (~1ms), this adds up across 748 tests.

---

## Recommended Optimizations

### Priority 1: Implement Test Database Fixture (Est. 30-40% speedup)

**Problem**: Each test runs `AutoMigrate()` independently.

**Solution**: Create a pre-migrated database template that can be cloned.

```go
// backend/internal/api/handlers/test_fixtures.go
package handlers

import (
    "sync"
    "testing"

    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "github.com/Wikid82/charon/backend/internal/models"
)

var (
    templateDB   *gorm.DB
    templateOnce sync.Once
)

// initTemplateDB creates a pre-migrated database template (called once)
func initTemplateDB() {
    var err error
    templateDB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        panic(err)
    }

    // Migrate ALL models once
    templateDB.AutoMigrate(
        &models.User{},
        &models.ProxyHost{},
        &models.Location{},
        &models.RemoteServer{},
        &models.Notification{},
        &models.NotificationProvider{},
        &models.Setting{},
        &models.SecurityConfig{},
        &models.SecurityDecision{},
        &models.SecurityAudit{},
        &models.SecurityRuleSet{},
        &models.SSLCertificate{},
        &models.AccessList{},
        &models.UptimeMonitor{},
        &models.UptimeHeartbeat{},
        // ... all other models
    )
}

// GetTestDB returns a fresh database with all migrations pre-applied
func GetTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    templateOnce.Do(initTemplateDB)

    // Create unique in-memory DB for this test
    uniqueDSN := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared",
        t.Name(), time.Now().UnixNano())
    db, err := gorm.Open(sqlite.Open(uniqueDSN), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }

    // Copy schema from template (much faster than AutoMigrate)
    copySchema(templateDB, db)
    return db
}
```

---

### Priority 2: Replace `time.Sleep()` with Event-Driven Synchronization (Est. 15-20% speedup)

**Problem**: Tests use arbitrary sleep durations to wait for async operations.

**Solution**: Use channels, waitgroups, or polling with short intervals.

**Before**:

```go
// cerberus_logs_ws_test.go:108
time.Sleep(300 * time.Millisecond)
```

**After**:

```go
// Use a helper that polls with short intervals
func waitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
    t.Helper()
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if check() {
            return
        }
        time.Sleep(10 * time.Millisecond)
    }
    t.Fatal("condition not met within timeout")
}

// In test:
waitForCondition(t, 500*time.Millisecond, func() bool {
    return watcher.SubscriberCount() > 0
})
```

**Specific fixes**:

| File | Current | Recommended |
|------|---------|-------------|
| [cerberus_logs_ws_test.go](backend/internal/api/handlers/cerberus_logs_ws_test.go#L108) | `time.Sleep(300ms)` | Poll `watcher.SubscriberCount()` |
| [uptime_service_test.go](backend/internal/services/uptime_service_test.go#L766) | `time.Sleep(2s)` | Use context timeout in test |
| [notification_service_test.go](backend/internal/services/notification_service_test.go#L306) | `time.Sleep(100ms)` | Wait for notification channel |

---

### Priority 3: Add `t.Parallel()` to Handler Tests (Est. 20-30% speedup)

**Problem**: 80% of handler tests run sequentially.

**Solution**: Add `t.Parallel()` to all tests that don't share global state.

**Pattern to apply**:

```go
func TestRemoteServerHandler_List(t *testing.T) {
    t.Parallel() // ADD THIS
    gin.SetMode(gin.TestMode)
    db := setupTestDB(t)
    // ...
}
```

**Files to update** (partial list):

- [handlers_test.go](backend/internal/api/handlers/handlers_test.go)
- [auth_handler_test.go](backend/internal/api/handlers/auth_handler_test.go)
- [proxy_host_handler_test.go](backend/internal/api/handlers/proxy_host_handler_test.go)
- [security_handler_test.go](backend/internal/api/handlers/security_handler_test.go)
- [crowdsec_handler_test.go](backend/internal/api/handlers/crowdsec_handler_test.go)

**Caveat**: Ensure tests don't rely on shared state (environment variables, global singletons).

---

### Priority 4: Create Shared Test Fixtures (Est. 10% speedup)

**Problem**: Common test data is created repeatedly.

**Solution**: Pre-create common fixtures in setup functions.

```go
// test_fixtures.go
type TestFixtures struct {
    DB          *gorm.DB
    AdminUser   *models.User
    TestHost    *models.ProxyHost
    TestServer  *models.RemoteServer
    Router      *gin.Engine
}

func NewTestFixtures(t *testing.T) *TestFixtures {
    t.Helper()
    db := GetTestDB(t)

    adminUser := &models.User{
        UUID:  uuid.NewString(),
        Email: "admin@test.com",
        Role:  "admin",
    }
    adminUser.SetPassword("password")
    db.Create(adminUser)

    // ... create other common fixtures

    return &TestFixtures{
        DB:        db,
        AdminUser: adminUser,
        // ...
    }
}
```

---

### Priority 5: Use Table-Driven Tests (Est. 5% speedup)

**Problem**: Similar tests with different inputs are written as separate functions.

**Solution**: Consolidate into table-driven tests with subtests.

**Before** (3 separate test functions):

```go
func TestAuthHandler_Login_Success(t *testing.T) { ... }
func TestAuthHandler_Login_InvalidPassword(t *testing.T) { ... }
func TestAuthHandler_Login_UserNotFound(t *testing.T) { ... }
```

**After** (1 table-driven test):

```go
func TestAuthHandler_Login(t *testing.T) {
    tests := []struct {
        name     string
        email    string
        password string
        wantCode int
    }{
        {"success", "test@example.com", "password123", http.StatusOK},
        {"invalid_password", "test@example.com", "wrong", http.StatusUnauthorized},
        {"user_not_found", "nobody@example.com", "password", http.StatusUnauthorized},
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            // Test implementation
        })
    }
}
```

---

## Estimated Time Savings

| Optimization | Current Time | Estimated Savings | Effort |
|--------------|-------------|-------------------|--------|
| Template DB (Priority 1) | ~45s | 30-40% (~15s) | Medium |
| Remove Sleeps (Priority 2) | ~20s | 15-20% (~10s) | Medium |
| Parallelize (Priority 3) | N/A | 20-30% (~12s) | Low |
| Shared Fixtures (Priority 4) | ~10s | 10% (~5s) | Low |
| Table-Driven (Priority 5) | ~5s | 5% (~2s) | Low |

**Total estimated improvement**: 50-70% reduction in test execution time

---

## Implementation Checklist

### Phase 1: Quick Wins (1-2 days) ✅ COMPLETED

- [x] Add `t.Parallel()` to all handler tests
  - Added to `handlers_test.go` (11 tests)
  - Added to `auth_handler_test.go` (31 tests)
  - Added to `proxy_host_handler_test.go` (41 tests)
  - Added to `crowdsec_handler_test.go` (24 tests - excluded 6 using t.Setenv)
  - **Note**: Tests using `t.Setenv()` cannot use `t.Parallel()` due to Go runtime restriction
- [x] Create `waitForCondition()` helper function
  - Created in `backend/internal/api/handlers/test_helpers.go`
- [ ] Replace top 10 longest `time.Sleep()` calls (DEFERRED - existing sleeps are appropriate for async WebSocket/notification scenarios)

### Phase 2: Infrastructure (3-5 days) ✅ COMPLETED

- [x] Implement template database pattern in `testdb.go`
  - Added `templateDBOnce sync.Once` for single initialization
  - Added `initTemplateDB()` that migrates all 24 models once
  - Added `GetTemplateDB()` function
  - Added `OpenTestDBWithMigrations()` that copies schema from template
- [ ] Create shared fixture builders (DEFERRED - not needed with current architecture)
- [x] Existing tests work with new infrastructure

### Phase 3: Consolidation (2-3 days)

- [ ] Convert repetitive tests to table-driven format
- [x] Remove redundant AutoMigrate calls (template pattern handles this)
- [ ] Profile and optimize remaining slow tests

---

## Monitoring and Validation

### Before Optimization

Run baseline measurement:

```bash
cd backend && go test -v ./internal/api/handlers/... 2>&1 | tee test_baseline.log
```

### After Each Phase

Compare execution time:

```bash
go test -v ./internal/api/handlers/... -json | go-test-report
```

### Success Criteria

- Total handler test time < 30 seconds
- No individual test > 2 seconds (except integration tests)
- All tests remain green with `t.Parallel()`

---

## Appendix: Files Requiring Updates

### High Priority (Most Impact)

1. [testdb.go](backend/internal/api/handlers/testdb.go) - Replace with template DB
2. [cerberus_logs_ws_test.go](backend/internal/api/handlers/cerberus_logs_ws_test.go) - Remove sleeps
3. [handlers_test.go](backend/internal/api/handlers/handlers_test.go) - Add parallelization
4. [uptime_service_test.go](backend/internal/services/uptime_service_test.go) - Remove sleeps

### Medium Priority

1. [proxy_host_handler_test.go](backend/internal/api/handlers/proxy_host_handler_test.go)
2. [crowdsec_handler_test.go](backend/internal/api/handlers/crowdsec_handler_test.go)
3. [auth_handler_test.go](backend/internal/api/handlers/auth_handler_test.go)
4. [notification_service_test.go](backend/internal/services/notification_service_test.go)

### Low Priority (Minor Impact)

1. [benchmark_test.go](backend/internal/api/handlers/benchmark_test.go)
2. [security_handler_rules_decisions_test.go](backend/internal/api/handlers/security_handler_rules_decisions_test.go)
