# Test Coverage Plan - Achieving 100% Coverage

**Created:** December 16, 2025
**Goal:** Add unit tests to achieve 100% coverage for 6 files with missing coverage
**Target Coverage:** 100% line coverage, 100% branch coverage

---

## Executive Summary

This plan outlines the strategy to achieve 100% test coverage for 6 backend files that currently have insufficient test coverage. The focus is on:

1. **crowdsec_handler.go** - 62.62% (30 missing lines, 7 partial branches)
2. **log_watcher.go** - 56.25% (14 missing lines, 7 partial branches)
3. **console_enroll.go** - 79.59% (11 missing lines, 9 partial branches)
4. **crowdsec_startup.go** - 94.73% (5 missing lines, 1 partial branch)
5. **routes.go** - 69.23% (2 missing lines, 2 partial branches)
6. **crowdsec_exec.go** - 92.85% (2 missing lines)

---

## File 1: backend/internal/api/handlers/crowdsec_handler.go

**Current Coverage:** 62.62% (30 missing lines, 7 partial branches)
**Existing Test File:** `backend/internal/api/handlers/crowdsec_handler_test.go`
**Priority:** HIGH (most complex, most missing coverage)

### Coverage Analysis

#### Missing Coverage Areas

1. **Start Handler - LAPI Readiness Polling (Lines 247-276)**
   - LAPI readiness check loop with timeout
   - Error handling when LAPI fails to become ready
   - Warning response when LAPI not ready after timeout

2. **GetLAPIDecisions Handler (Lines 629-749)**
   - HTTP request creation errors
   - LAPI authentication handling
   - Non-200 response codes (401 Unauthorized)
   - Content-type validation (HTML from proxy vs JSON)
   - Empty/null response handling
   - JSON parsing errors
   - LAPI decision conversion

3. **CheckLAPIHealth Handler (Lines 752-791)**
   - Health endpoint unavailable fallback to decisions endpoint
   - Decision endpoint HEAD request for health check
   - Various HTTP status codes (401 indicates LAPI running but auth required)

4. **ListDecisions Handler (Lines 794-828)**
   - cscli command execution errors
   - Empty/null output handling
   - JSON parsing errors

5. **BanIP Handler (Lines 835-869)**
   - Empty IP validation
   - cscli command execution errors

6. **UnbanIP Handler (Lines 872-895)**
   - Missing IP parameter validation
   - cscli command execution errors

7. **RegisterBouncer Handler (Lines 898-946)**
   - Script not found error path
   - Bouncer registration verification with cscli query

8. **GetAcquisitionConfig Handler (Lines 949-967)**
   - File not found handling
   - File read errors

9. **UpdateAcquisitionConfig Handler (Lines 970-1012)**
   - Invalid payload handling
   - Backup creation errors
   - File write errors
   - Backup restoration on error

### Test Cases to Add

#### Test Suite: Start Handler LAPI Polling

```go
TestCrowdsecStart_LAPINotReadyTimeout(t *testing.T)
// Arrange: Mock executor that starts but cscli lapi status always fails
// Act: Call Start handler
// Assert: Returns 200 but with lapi_ready=false and warning message

TestCrowdsecStart_LAPIBecomesReadyAfterRetries(t *testing.T)
// Arrange: Mock executor where lapi status fails 2 times, succeeds on 3rd
// Act: Call Start handler
// Assert: Returns 200 with lapi_ready=true after polling

TestCrowdsecStart_ConfigFileNotFound(t *testing.T)
// Arrange: DataDir without config.yaml
// Act: Call Start handler
// Assert: LAPI check runs without -c flag
```

#### Test Suite: GetLAPIDecisions Handler

```go
TestGetLAPIDecisions_RequestCreationError(t *testing.T)
// Arrange: Invalid context that fails http.NewRequestWithContext
// Act: Call handler
// Assert: Returns 500 error

TestGetLAPIDecisions_Unauthorized(t *testing.T)
// Arrange: Mock LAPI returns 401
// Act: Call handler
// Assert: Returns 401 with proper error message

TestGetLAPIDecisions_NonOKStatus(t *testing.T)
// Arrange: Mock LAPI returns 500
// Act: Call handler
// Assert: Falls back to ListDecisions (cscli)

TestGetLAPIDecisions_NonJSONContentType(t *testing.T)
// Arrange: Mock LAPI returns text/html (proxy error page)
// Act: Call handler
// Assert: Falls back to ListDecisions (cscli)

TestGetLAPIDecisions_NullResponse(t *testing.T)
// Arrange: Mock LAPI returns "null" or empty body
// Act: Call handler
// Assert: Returns empty decisions array

TestGetLAPIDecisions_JSONParseError(t *testing.T)
// Arrange: Mock LAPI returns invalid JSON
// Act: Call handler
// Assert: Returns 500 with parse error

TestGetLAPIDecisions_WithQueryParams(t *testing.T)
// Arrange: Request with ?ip=192.168.1.1&scope=ip&type=ban
// Act: Call handler
// Assert: Query params passed to LAPI URL
```

#### Test Suite: CheckLAPIHealth Handler

```go
TestCheckLAPIHealth_HealthEndpointUnavailable(t *testing.T)
// Arrange: Mock LAPI where /health fails, /v1/decisions HEAD succeeds
// Act: Call handler
// Assert: Returns healthy=true with note about fallback

TestCheckLAPIHealth_DecisionEndpoint401(t *testing.T)
// Arrange: Mock LAPI where /health fails, /v1/decisions returns 401
// Act: Call handler
// Assert: Returns healthy=true (401 means LAPI running)

TestCheckLAPIHealth_DecisionEndpointUnexpectedStatus(t *testing.T)
// Arrange: Mock LAPI where both endpoints return 500
// Act: Call handler
// Assert: Returns healthy=false with status code
```

#### Test Suite: ListDecisions Handler

```go
TestListDecisions_CscliNotAvailable(t *testing.T)
// Arrange: Mock executor that returns error for cscli command
// Act: Call handler
// Assert: Returns 200 with empty array and error message

TestListDecisions_EmptyOutput(t *testing.T)
// Arrange: Mock executor returns empty byte array
// Act: Call handler
// Assert: Returns empty decisions array

TestListDecisions_NullOutput(t *testing.T)
// Arrange: Mock executor returns "null" or "null\n"
// Act: Call handler
// Assert: Returns empty decisions array

TestListDecisions_InvalidJSON(t *testing.T)
// Arrange: Mock executor returns malformed JSON
// Act: Call handler
// Assert: Returns 500 with parse error
```

#### Test Suite: BanIP Handler

```go
TestBanIP_EmptyIP(t *testing.T)
// Arrange: Request with empty/whitespace IP
// Act: Call handler
// Assert: Returns 400 error

TestBanIP_CscliExecutionError(t *testing.T)
// Arrange: Mock executor returns error
// Act: Call handler
// Assert: Returns 500 error
```

#### Test Suite: UnbanIP Handler

```go
TestUnbanIP_MissingIPParameter(t *testing.T)
// Arrange: Request without :ip parameter
// Act: Call handler
// Assert: Returns 400 error

TestUnbanIP_CscliExecutionError(t *testing.T)
// Arrange: Mock executor returns error
// Act: Call handler
// Assert: Returns 500 error
```

#### Test Suite: RegisterBouncer Handler

```go
TestRegisterBouncer_CscliCheckSuccess(t *testing.T)
// Arrange: Mock successful registration + cscli bouncers list shows caddy-bouncer
// Act: Call handler
// Assert: Returns registered=true
```

#### Test Suite: GetAcquisitionConfig Handler

```go
TestGetAcquisitionConfig_ReadError(t *testing.T)
// Arrange: File exists but read fails (permissions)
// Act: Call handler
// Assert: Returns 500 error
```

#### Test Suite: UpdateAcquisitionConfig Handler

```go
TestUpdateAcquisitionConfig_InvalidPayload(t *testing.T)
// Arrange: Non-JSON request body
// Act: Call handler
// Assert: Returns 400 error

TestUpdateAcquisitionConfig_BackupError(t *testing.T)
// Arrange: Existing file but backup (rename) fails
// Act: Call handler
// Assert: Continues anyway (logs warning)

TestUpdateAcquisitionConfig_WriteErrorRestoresBackup(t *testing.T)
// Arrange: File write fails after backup created
// Act: Call handler
// Assert: Restores backup, returns 500 error
```

### Mock Requirements

- **MockCommandExecutor**: Already exists in test file, extend to support error injection
- **MockHTTPServer**: For testing LAPI HTTP client behavior
- **MockFileSystem**: For testing acquisition config file operations

### Expected Assertions

- HTTP status codes (200, 400, 401, 500)
- JSON response structure
- Error messages contain expected text
- Function call counts on mocks
- Database state after operations
- File existence/content after operations

---

## File 2: backend/internal/services/log_watcher.go

**Current Coverage:** 56.25% (14 missing lines, 7 partial branches)
**Existing Test File:** `backend/internal/services/log_watcher_test.go`
**Priority:** MEDIUM (well-tested core, missing edge cases)

### Coverage Analysis

#### Missing Coverage Areas

1. **readLoop Function - EOF Handling (Lines 130-142)**
   - When `reader.ReadString` returns EOF
   - 100ms sleep before retry
   - File rotation detection

2. **readLoop Function - Non-EOF Errors (Lines 143-146)**
   - When `reader.ReadString` returns error other than EOF
   - Log and return to trigger file reopen

3. **detectSecurityEvent Function - WAF Detection (Lines 176-194)**
   - hasHeader checks for X-Coraza-Id
   - hasHeader checks for X-Coraza-Rule-Id
   - Early return when WAF detected

4. **detectSecurityEvent Function - CrowdSec Detection (Lines 196-210)**
   - hasHeader checks for X-Crowdsec-Decision
   - hasHeader checks for X-Crowdsec-Origin
   - Early return when CrowdSec detected

5. **detectSecurityEvent Function - ACL Detection (Lines 212-218)**
   - hasHeader checks for X-Acl-Denied
   - hasHeader checks for X-Blocked-By-Acl

6. **detectSecurityEvent Function - Rate Limit Headers (Lines 220-234)**
   - Extraction of X-Ratelimit-Remaining
   - Extraction of X-Ratelimit-Reset
   - Extraction of X-Ratelimit-Limit

7. **detectSecurityEvent Function - 403 Generic (Lines 236-242)**
   - 403 without specific security headers
   - Falls back to "cerberus" source

### Test Cases to Add

#### Test Suite: readLoop EOF Handling

```go
TestLogWatcher_ReadLoop_EOFRetry(t *testing.T)
// Arrange: Mock file that returns EOF on first read, then data
// Act: Start watcher, write log after brief delay
// Assert: Watcher continues reading after EOF, receives new entry

TestLogWatcher_ReadLoop_FileRotation(t *testing.T)
// Arrange: Create log file, write entry, rename/recreate file, write new entry
// Act: Start watcher before rotation
// Assert: Watcher detects rotation and continues with new file
```

#### Test Suite: readLoop Error Handling

```go
TestLogWatcher_ReadLoop_ReadError(t *testing.T)
// Arrange: Mock reader that returns non-EOF error
// Act: Trigger error during read
// Assert: Watcher logs error and reopens file
```

#### Test Suite: detectSecurityEvent - WAF

```go
TestDetectSecurityEvent_WAFWithCorazaId(t *testing.T)
// Arrange: Caddy log with X-Coraza-Id header
// Act: Parse log entry
// Assert: source=waf, blocked=true, rule_id extracted

TestDetectSecurityEvent_WAFWithCorazaRuleId(t *testing.T)
// Arrange: Caddy log with X-Coraza-Rule-Id header
// Act: Parse log entry
// Assert: source=waf, blocked=true, rule_id extracted

TestDetectSecurityEvent_WAFLoggerName(t *testing.T)
// Arrange: Caddy log with logger=http.handlers.waf
// Act: Parse log entry
// Assert: source=waf, blocked=true
```

#### Test Suite: detectSecurityEvent - CrowdSec

```go
TestDetectSecurityEvent_CrowdSecWithDecisionHeader(t *testing.T)
// Arrange: Caddy log with X-Crowdsec-Decision header
// Act: Parse log entry
// Assert: source=crowdsec, blocked=true, decision extracted

TestDetectSecurityEvent_CrowdSecWithOriginHeader(t *testing.T)
// Arrange: Caddy log with X-Crowdsec-Origin header
// Act: Parse log entry
// Assert: source=crowdsec, blocked=true, origin extracted

TestDetectSecurityEvent_CrowdSecLoggerName(t *testing.T)
// Arrange: Caddy log with logger=http.handlers.crowdsec
// Act: Parse log entry
// Assert: source=crowdsec, blocked=true
```

#### Test Suite: detectSecurityEvent - ACL

```go
TestDetectSecurityEvent_ACLDeniedHeader(t *testing.T)
// Arrange: Caddy log with X-Acl-Denied header
// Act: Parse log entry
// Assert: source=acl, blocked=true

TestDetectSecurityEvent_ACLBlockedHeader(t *testing.T)
// Arrange: Caddy log with X-Blocked-By-Acl header
// Act: Parse log entry
// Assert: source=acl, blocked=true
```

#### Test Suite: detectSecurityEvent - Rate Limiting

```go
TestDetectSecurityEvent_RateLimitAllHeaders(t *testing.T)
// Arrange: 429 with X-Ratelimit-Remaining, Reset, Limit headers
// Act: Parse log entry
// Assert: All rate limit metadata extracted to Details map

TestDetectSecurityEvent_RateLimitPartialHeaders(t *testing.T)
// Arrange: 429 with only X-Ratelimit-Remaining header
// Act: Parse log entry
// Assert: Only available headers in Details map
```

#### Test Suite: detectSecurityEvent - Generic 403

```go
TestDetectSecurityEvent_403WithoutHeaders(t *testing.T)
// Arrange: 403 response with no security headers
// Act: Parse log entry
// Assert: source=cerberus, blocked=true, block_reason="Access denied"
```

### Mock Requirements

- **MockFile**: For simulating EOF and read errors
- **MockReader**: For controlled error injection

### Expected Assertions

- Entry fields match expected values
- Headers are extracted correctly
- Details map contains expected keys/values
- Blocked flag is set correctly
- BlockReason is populated
- Source is identified correctly

---

## File 3: backend/internal/crowdsec/console_enroll.go

**Current Coverage:** 79.59% (11 missing lines, 9 partial branches)
**Existing Test File:** `backend/internal/crowdsec/console_enroll_test.go`
**Priority:** MEDIUM (critical feature, mostly covered, need edge cases)

### Coverage Analysis

#### Missing Coverage Areas

1. **Enroll Function - Invalid Agent Name (Lines 117-119)**
   - Agent name regex validation failure
   - Error message formatting

2. **Enroll Function - Invalid Tenant Name (Lines 121-123)**
   - Tenant name regex validation failure
   - Error message formatting

3. **ensureCAPIRegistered Function - Standard Layout (Lines 198-201)**
   - Checking config/online_api_credentials.yaml (standard layout)
   - Fallback to root online_api_credentials.yaml

4. **ensureCAPIRegistered Function - CAPI Register Error (Lines 212-214)**
   - CAPI register command failure
   - Error message with command output

5. **findConfigPath Function - Standard Layout (Lines 218-222)**
   - Checking config/config.yaml (standard layout)
   - Fallback to root config.yaml
   - Return empty string if neither exists

6. **statusFromModel Function - Nil Check (Lines 268-270)**
   - When model is nil, return not_enrolled status

7. **normalizeEnrollmentKey Function - Invalid Format (Lines 374-376)**
   - When key doesn't match any valid pattern
   - Error message

### Test Cases to Add

#### Test Suite: Enroll Input Validation

```go
TestEnroll_InvalidAgentNameCharacters(t *testing.T)
// Arrange: Agent name with special chars like "agent@name!"
// Act: Call Enroll
// Assert: Returns error "may only include letters, numbers, dot, dash, underscore"

TestEnroll_AgentNameTooLong(t *testing.T)
// Arrange: Agent name with 65 characters
// Act: Call Enroll
// Assert: Returns error (regex max 64)

TestEnroll_InvalidTenantNameCharacters(t *testing.T)
// Arrange: Tenant with special chars like "tenant$name"
// Act: Call Enroll
// Assert: Returns error "may only include letters, numbers, dot, dash, underscore"
```

#### Test Suite: ensureCAPIRegistered

```go
TestEnsureCAPIRegistered_StandardLayoutExists(t *testing.T)
// Arrange: Create dataDir/config/online_api_credentials.yaml
// Act: Call ensureCAPIRegistered
// Assert: Returns nil (no registration needed)

TestEnsureCAPIRegistered_RootLayoutExists(t *testing.T)
// Arrange: Create dataDir/online_api_credentials.yaml (not in config/)
// Act: Call ensureCAPIRegistered
// Assert: Returns nil (no registration needed)

TestEnsureCAPIRegistered_RegisterError(t *testing.T)
// Arrange: No credentials file, mock executor returns error
// Act: Call ensureCAPIRegistered
// Assert: Returns error with command output
```

#### Test Suite: findConfigPath

```go
TestFindConfigPath_StandardLayout(t *testing.T)
// Arrange: Create dataDir/config/config.yaml
// Act: Call findConfigPath
// Assert: Returns path to config/config.yaml

TestFindConfigPath_RootLayout(t *testing.T)
// Arrange: Create dataDir/config.yaml (not in config/)
// Act: Call findConfigPath
// Assert: Returns path to config.yaml

TestFindConfigPath_NeitherExists(t *testing.T)
// Arrange: Empty dataDir
// Act: Call findConfigPath
// Assert: Returns empty string
```

#### Test Suite: statusFromModel

```go
TestStatusFromModel_NilModel(t *testing.T)
// Arrange: Pass nil model
// Act: Call statusFromModel
// Assert: Returns ConsoleEnrollmentStatus with status=not_enrolled

TestStatusFromModel_EmptyStatus(t *testing.T)
// Arrange: Model with empty Status field
// Act: Call statusFromModel
// Assert: Returns status=not_enrolled (default)
```

#### Test Suite: normalizeEnrollmentKey

```go
TestNormalizeEnrollmentKey_InvalidCharacters(t *testing.T)
// Arrange: Key with special characters "abc@123#def"
// Act: Call normalizeEnrollmentKey
// Assert: Returns error "invalid enrollment key"

TestNormalizeEnrollmentKey_TooShort(t *testing.T)
// Arrange: Key with only 5 characters "ab123"
// Act: Call normalizeEnrollmentKey
// Assert: Returns error (regex requires min 10)

TestNormalizeEnrollmentKey_NonMatchingFormat(t *testing.T)
// Arrange: Random text "this is not a key"
// Act: Call normalizeEnrollmentKey
// Assert: Returns error "invalid enrollment key"
```

### Mock Requirements

- **MockFileSystem**: For testing config/credentials file detection
- **StubEnvExecutor**: Already exists in test file, extend for CAPI register errors

### Expected Assertions

- Error messages match expected patterns
- Status defaults are applied correctly
- File path resolution follows precedence order
- Regex validation catches all invalid inputs

---

## File 4: backend/internal/services/crowdsec_startup.go

**Current Coverage:** 94.73% (5 missing lines, 1 partial branch)
**Existing Test File:** `backend/internal/services/crowdsec_startup_test.go`
**Priority:** LOW (already excellent coverage, just a few edge cases)

### Coverage Analysis

#### Missing Coverage Areas

1. **ReconcileCrowdSecOnStartup - DB Error After Creating Config (Lines 78-80)**
   - After creating SecurityConfig, db.First fails on re-read
   - This is a rare edge case (DB commit succeeds but immediate read fails)

2. **ReconcileCrowdSecOnStartup - Settings Table Query Fallthrough (Lines 83-90)**
   - When db.Raw query fails (not just no rows)
   - Log field for setting_enabled

3. **ReconcileCrowdSecOnStartup - Decision Path When Settings Enabled (Lines 92-99)**
   - When SecurityConfig mode is NOT "local" but Settings enabled
   - Log indicating Settings override triggered start

### Test Cases to Add

#### Test Suite: Database Error Handling

```go
TestReconcileCrowdSecOnStartup_DBErrorAfterCreate(t *testing.T)
// Arrange: DB that succeeds Create but fails First (simulated with closed DB)
// Act: Call ReconcileCrowdSecOnStartup
// Assert: Function handles error gracefully, logs warning, returns early

TestReconcileCrowdSecOnStartup_SettingsQueryError(t *testing.T)
// Arrange: DB where Raw query returns error (not gorm.ErrRecordNotFound)
// Act: Call ReconcileCrowdSecOnStartup
// Assert: Function logs debug message and continues with SecurityConfig only
```

#### Test Suite: Settings Override Path

```go
TestReconcileCrowdSecOnStartup_SettingsOverrideWithRemoteMode(t *testing.T)
// Arrange: SecurityConfig with mode="remote", Settings with enabled=true
// Act: Call ReconcileCrowdSecOnStartup
// Assert: CrowdSec is started (Settings wins), log shows Settings override

TestReconcileCrowdSecOnStartup_SettingsOverrideWithAPIMode(t *testing.T)
// Arrange: SecurityConfig with mode="api", Settings with enabled=true
// Act: Call ReconcileCrowdSecOnStartup
// Assert: CrowdSec is started (Settings wins)
```

### Mock Requirements

- **ClosedDB**: Database that returns errors after operations
- Existing mocks are sufficient for most cases

### Expected Assertions

- Log messages indicate override path taken
- Start is called when Settings override is active
- Error handling doesn't panic
- Function returns gracefully on DB errors

---

## File 5: backend/internal/api/routes/routes.go

**Current Coverage:** 69.23% (2 missing lines, 2 partial branches)
**Existing Test File:** `backend/internal/api/routes/routes_test.go`
**Priority:** LOW (simple registration logic, missing only error paths)

### Coverage Analysis

#### Missing Coverage Areas

1. **Register Function - Docker Service Unavailable (Line 182)**
   - When `services.NewDockerService()` returns error
   - Log message for unavailable Docker service

2. **Register Function - Caddy Ping Timeout (Line 344)**
   - When waiting for Caddy readiness times out after 30 seconds
   - Log warning about timeout

3. **Register Function - Caddy Config Apply Error (Line 350)**
   - When `caddyManager.ApplyConfig` returns error
   - Log error message

4. **Register Function - Caddy Config Apply Success (Line 352)**
   - When `caddyManager.ApplyConfig` succeeds
   - Log success message

### Test Cases to Add

#### Test Suite: Service Initialization Errors

```go
TestRegister_DockerServiceUnavailable(t *testing.T)
// Arrange: Environment where Docker socket is not accessible
// Act: Call Register
// Assert: Routes still registered, logs warning "Docker service unavailable"

TestRegister_CaddyPingTimeout(t *testing.T)
// Arrange: Mock Caddy that never responds to Ping
// Act: Call Register
// Assert: Function returns after 30s timeout, logs "Timeout waiting for Caddy"

TestRegister_CaddyApplyConfigError(t *testing.T)
// Arrange: Mock Caddy that returns error on ApplyConfig
// Act: Call Register
// Assert: Error logged "Failed to apply initial Caddy config"

TestRegister_CaddyApplyConfigSuccess(t *testing.T)
// Arrange: Mock Caddy that succeeds immediately
// Act: Call Register
// Assert: Success logged "Successfully applied initial Caddy config"
```

### Mock Requirements

- **MockCaddyManager**: For simulating Ping and ApplyConfig behavior
- **MockDockerService**: For simulating Docker unavailability

### Expected Assertions

- Log messages match expected patterns
- Routes are registered even when services fail
- Goroutine completes within expected timeframe

---

## File 6: backend/internal/api/handlers/crowdsec_exec.go

**Current Coverage:** 92.85% (2 missing lines)
**Existing Test File:** `backend/internal/api/handlers/crowdsec_exec_test.go`
**Priority:** LOW (excellent coverage, just error path)

### Coverage Analysis

#### Missing Coverage Areas

1. **Stop Function - Signal Send Error (Lines 76-79)**
   - When `proc.Signal(syscall.SIGTERM)` returns error other than ESRCH/ErrProcessDone
   - Error is returned to caller

### Test Cases to Add

#### Test Suite: Stop Error Handling

```go
TestDefaultCrowdsecExecutor_Stop_SignalError(t *testing.T)
// Arrange: Process that exists but signal fails with permission error
// Act: Call Stop
// Assert: Returns error (not ESRCH or ErrProcessDone)

TestDefaultCrowdsecExecutor_Stop_SignalErrorCleanup(t *testing.T)
// Arrange: Process with PID file, signal fails with unknown error
// Act: Call Stop
// Assert: Error returned but PID file NOT cleaned up
```

### Mock Requirements

- **MockProcess**: For simulating signal send errors
- May need OS-level permissions testing (use build tags for Linux-only tests)

### Expected Assertions

- Error is propagated to caller
- PID file cleanup behavior matches error type
- ESRCH is handled as success (process already dead)

---

## Configuration File Updates

### .gitignore

**Status:** ✅ NO CHANGES NEEDED

Current configuration already excludes:

- Test output files (`*.out`, `*.cover`, `coverage/`)
- Coverage artifacts (`coverage*.txt`, `*.coverage.out`)
- All test-related temporary files

### .codecov.yml

**Status:** ✅ NO CHANGES NEEDED

Current configuration already excludes:

- All test files (`*_test.go`, `*.test.ts`)
- Integration tests (`**/integration/**`)
- Test utilities (`**/test/**`, `**/__tests__/**`)

The coverage targets and ignore patterns are comprehensive.

### .dockerignore

**Status:** ✅ NO CHANGES NEEDED

Current configuration already excludes:

- Test coverage artifacts (`coverage/`, `*.cover`)
- Test result files (`backend/test-output.txt`)
- All test-related files

---

## Testing Infrastructure Requirements

### Existing Infrastructure (✅ Already Available)

1. **Test Database Helper**: `OpenTestDB(t *testing.T)` in handlers tests
2. **Gin Test Mode**: `gin.SetMode(gin.TestMode)`
3. **HTTP Test Recorder**: `httptest.NewRecorder()`
4. **GORM SQLite**: In-memory database for tests
5. **Testify Assertions**: `require` and `assert` packages

### New Infrastructure Needed

1. **Mock HTTP Server** (for LAPI client tests)

   ```go
   // Add to crowdsec_handler_test.go
   type mockLAPIServer struct {
       responses []mockResponse
       callCount int
   }

   type mockResponse struct {
       status      int
       contentType string
       body        []byte
   }
   ```

2. **Mock File System** (for acquisition config tests)

   ```go
   // Add to test utilities
   type mockFileOps interface {
       ReadFile(path string) ([]byte, error)
       WriteFile(path string, data []byte) error
       Stat(path string) (os.FileInfo, error)
   }
   ```

3. **Time Travel Helper** (for LAPI polling timeout tests)

   ```go
   // Use testify's Eventually helper or custom sleep mock
   ```

4. **Log Capture Helper** (for verifying log messages)

   ```go
   // Capture logrus output to buffer for assertions
   type logCapture struct {
       buf *bytes.Buffer
       hook *logrus.Hook
   }
   ```

---

## Test Execution Strategy

### Phase 1: High-Priority Files (Week 1)

1. **crowdsec_handler.go** (2-3 days)
   - Implement all GetLAPIDecisions tests
   - Implement Start handler LAPI polling tests
   - Implement decision management tests
   - Implement acquisition config tests

2. **log_watcher.go** (1 day)
   - Implement detectSecurityEvent edge cases
   - Implement readLoop error handling
   - Verify all header detection paths

### Phase 2: Medium-Priority Files (Week 2)

1. **console_enroll.go** (1 day)
   - Implement input validation tests
   - Implement config path resolution tests
   - Implement CAPI registration error tests

### Phase 3: Low-Priority Files (Week 2)

1. **crowdsec_startup.go** (0.5 days)
   - Implement DB error handling tests
   - Implement Settings override path tests

2. **routes.go** (0.5 days)
   - Implement service initialization error tests
   - Implement Caddy startup sequence tests

3. **crowdsec_exec.go** (0.5 days)
   - Implement Stop signal error test

### Validation Strategy

1. **Run Tests After Each File**

   ```bash
   cd backend && go test -v ./internal/api/handlers -run TestCrowdsec
   cd backend && go test -v ./internal/services -run TestLogWatcher
   cd backend && go test -v ./internal/crowdsec -run TestConsole
   ```

2. **Generate Coverage Report**

   ```bash
   ./scripts/go-test-coverage.sh
   ```

3. **Verify 100% Coverage**

   ```bash
   go tool cover -html=coverage.out -o coverage.html
   # Open coverage.html and verify all target files show 100%
   ```

4. **Run Pre-Commit Hooks**

   ```bash
   pre-commit run --all-files
   ```

---

## Success Criteria

### Per-File Targets

- ✅ **crowdsec_handler.go**: 100.00% (0 missing lines, 0 partial branches)
- ✅ **log_watcher.go**: 100.00% (0 missing lines, 0 partial branches)
- ✅ **console_enroll.go**: 100.00% (0 missing lines, 0 partial branches)
- ✅ **crowdsec_startup.go**: 100.00% (0 missing lines, 0 partial branches)
- ✅ **routes.go**: 100.00% (0 missing lines, 0 partial branches)
- ✅ **crowdsec_exec.go**: 100.00% (0 missing lines, 0 partial branches)

### Quality Gates

1. **All tests pass**: `go test ./... -v`
2. **Coverage >= 100%**: For each target file
3. **Pre-commit passes**: All linters and formatters
4. **No flaky tests**: Tests are deterministic and reliable
5. **Fast execution**: Test suite runs in < 30 seconds

---

## Risk Assessment

### Low Risk

- **crowdsec_exec.go**: Only 2 lines missing, straightforward error path
- **routes.go**: Simple logging branches, easy to test
- **crowdsec_startup.go**: Already 94.73%, just edge cases

### Medium Risk

- **log_watcher.go**: File I/O and error handling, but well-isolated
- **console_enroll.go**: Mostly covered, just input validation

### High Risk

- **crowdsec_handler.go**: Complex HTTP client behavior, LAPI interaction, multiple fallback paths
  - **Mitigation**: Use httptest for mock LAPI server, test each fallback path independently

---

## Implementation Checklist

### Pre-Implementation

- [ ] Review existing test patterns in each test file
- [ ] Set up mock HTTP server for LAPI tests
- [ ] Create test data fixtures (sample JSON responses)
- [ ] Document expected error messages for assertions

### During Implementation

- [ ] Write tests incrementally (one suite at a time)
- [ ] Run coverage after each test suite
- [ ] Verify no regression in existing tests
- [ ] Add comments explaining complex test scenarios

### Post-Implementation

- [ ] Generate and review final coverage report
- [ ] Run full test suite 3 times to check for flakes
- [ ] Update test documentation if patterns change
- [ ] Create PR with coverage improvement metrics

---

## Appendix: Example Test Patterns

### Pattern 1: Testing HTTP Handlers with Mock LAPI

```go
func TestGetLAPIDecisions_Success(t *testing.T) {
    // Create mock LAPI server
    mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode([]lapiDecision{
            {ID: 1, Value: "192.168.1.1", Type: "ban"},
        })
    }))
    defer mockLAPI.Close()

    // Create handler with mock LAPI URL
    h := setupHandlerWithLAPIURL(mockLAPI.URL)

    // Make request
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/lapi", http.NoBody)
    h.GetLAPIDecisions(w, req)

    // Assert
    require.Equal(t, http.StatusOK, w.Code)
    var resp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Equal(t, "lapi", resp["source"])
}
```

### Pattern 2: Testing File Operations

```go
func TestGetAcquisitionConfig_Success(t *testing.T) {
    // Create temp file
    tmpFile := filepath.Join(t.TempDir(), "acquis.yaml")
    content := "source: file\nfilenames:\n  - /var/log/test.log"
    require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

    // Override acquisPath in handler (use setter or constructor param)
    h := NewCrowdsecHandlerWithAcquisPath(tmpFile)

    // Make request
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/acquisition", http.NoBody)
    h.GetAcquisitionConfig(w, req)

    // Assert
    require.Equal(t, http.StatusOK, w.Code)
    var resp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Equal(t, content, resp["content"])
}
```

### Pattern 3: Testing Error Paths with Mocks

```go
func TestEnroll_ExecutorError(t *testing.T) {
    db := openConsoleTestDB(t)
    exec := &stubEnvExecutor{
        responses: []struct {
            out []byte
            err error
        }{
            {nil, nil}, // lapi status success
            {nil, nil}, // capi register success
            {[]byte("error output"), fmt.Errorf("enrollment failed")}, // enroll error
        },
    }

    svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

    status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{
        EnrollmentKey: "abc123",
        AgentName:     "test-agent",
    })

    require.Error(t, err)
    assert.Equal(t, "failed", status.Status)
    assert.Contains(t, status.LastError, "enrollment failed")
}
```

---

## Conclusion

This comprehensive plan provides a clear roadmap to achieve 100% test coverage for all identified files. The strategy prioritizes high-impact files first, uses proven test patterns, and includes detailed test case descriptions for every missing coverage area.

**Estimated Effort:** 4-5 days total
**Expected Outcome:** 100% line and branch coverage for all 6 target files
**Risk Level:** Medium (mostly straightforward, some complex HTTP client testing)

Implementation should proceed file-by-file, with continuous validation of coverage improvements after each test suite is added.
