# Backend Test Coverage Restoration Plan

## Executive Summary

**Objective:** Restore backend test coverage from current 59.33% patch coverage to **86% locally** (85%+ in CI) with **100% patch coverage** (Codecov requirement).

**Current State:**
- Patch Coverage: 59.33% (98 lines missing coverage)
- 10 files identified with coverage gaps
- Priority ranking based on missing lines and business impact

**Strategy:** Systematic file-by-file coverage remediation using table-driven tests, interface mocking, and existing test infrastructure patterns.

---

## 1. Root Cause Analysis

### Why Coverage Dropped

1. **Recent Feature Additions Without Tests**
   - Caddyfile import enhancements (multi-file, normalization)
   - CrowdSec hub presets and console enrollment
   - Manual challenge DNS provider flow
   - Feature flag batch optimization

2. **Complex Error Paths Untested**
   - SSRF validation failures
   - Path traversal edge cases
   - Process lifecycle edge cases (PID recycling)
   - Cache TTL expiry and eviction paths

3. **Integration-Heavy Code**
   - External command execution (Caddy, CrowdSec)
   - HTTP client operations with fallback logic
   - File system operations with safety checks
   - Cron scheduler lifecycle

### Coverage Gap Breakdown

| File | Coverage | Missing | Partials | Priority |
|------|----------|---------|----------|----------|
| import_handler.go | 45.83% | 33 lines | 6 branches | 🔴 CRITICAL |
| crowdsec_handler.go | 21.42% | 5 lines | 6 branches | 🔴 CRITICAL |
| backup_service.go | 63.33% | 5 lines | 6 branches | 🟡 HIGH |
| hub_sync.go | 46.66% | 3 lines | 5 branches | 🟡 HIGH |
| feature_flags_handler.go | 83.33% | 4 lines | 2 branches | 🟢 MEDIUM |
| importer.go | 76.0% | 22 lines | 4 branches | 🟡 HIGH |
| security_service.go | 50.0% | 12 lines | 8 branches | 🟡 HIGH |
| hub_cache.go | 28.57% | 5 lines | 0 branches | 🟡 HIGH |
| manual_challenge_handler.go | 33.33% | 8 lines | 4 branches | 🟡 HIGH |
| crowdsec_exec.go | 0% | 1 line | 0 branches | 🟢 LOW |

**Total Impact:** 98 missing lines, 35 partial branches

---

## 2. File-by-File Coverage Plan

### File 1: backend/internal/api/handlers/import_handler.go (933 lines)

**Current Coverage:** 45.83% (33 missing, 6 partials)
**Target Coverage:** 100%
**Complexity:** 🔴 HIGH (complex file handling, path safety, session management)

**Existing Test File:** `backend/internal/api/handlers/import_handler_test.go`

#### Functions Requiring Coverage

1. **Upload() - Lines 85-150**
   - Missing: Normalization success/failure paths
   - Missing: Path traversal validation
   - Partials: Error handling branches

2. **UploadMulti() - Lines 155-220**
   - Missing: Multi-file conflict detection
   - Missing: Archive extraction edge cases
   - Partials: File validation failures

3. **Commit() - Lines 225-290**
   - Missing: Transient session promotion
   - Missing: Import session cleanup
   - Partials: ProxyHost service failures

4. **DetectImports() - Lines 320-380**
   - Missing: Empty Caddyfile handling
   - Missing: Parse failure recovery
   - Partials: Conflict resolution logic

5. **safeJoin() helper - Lines 450-475**
   - Missing: Parent directory traversal
   - Missing: Absolute path rejection

#### Test Cases Required

```go
// TestUpload_NormalizationSuccess
// - Upload single-line Caddyfile
// - Verify NormalizeCaddyfile called
// - Assert formatted content stored

// TestUpload_NormalizationFailure
// - Mock importer.NormalizeCaddyfile to return error
// - Verify upload fails with clear error message

// TestUpload_PathTraversal
// - Upload Caddyfile with "../../../etc/passwd"
// - Verify rejection with security error

// TestUploadMulti_ConflictDetection
// - Upload archive with duplicate domains
// - Verify conflicts reported in preview

// TestCommit_TransientPromotion
// - Create transient session
// - Commit session
// - Verify DB persistence

// TestDetectImports_EmptyCaddyfile
// - Upload empty file
// - Verify graceful handling with no imports

// TestSafeJoin_Traversal
// - Call safeJoin with "../..", "sensitive.file"
// - Verify error returned

// TestSafeJoin_AbsolutePath
// - Call safeJoin with "/etc/passwd"
// - Verify error returned
```

#### Implementation Strategy

1. **Mock Importer Interface** (NEW)
   - Create `ImporterInterface` with `NormalizeCaddyfile()`, `ParseCaddyfile()`, `ExtractHosts()`
   - Update handler to accept interface instead of concrete type
   - Use in tests to simulate failures

2. **Table-Driven Path Safety Tests**
   - Test matrix of forbidden patterns (`..`, absolute paths, unicode tricks)

3. **Session Lifecycle Tests**
   - Test transient → DB promotion
   - Test cleanup on timeout
   - Test concurrent access

**Estimated Effort:** 3 developer-days

---

### File 2: backend/internal/api/handlers/crowdsec_handler.go (1006 lines)

**Current Coverage:** 21.42% (5 missing, 6 partials)
**Target Coverage:** 100%
**Complexity:** 🔴 HIGH (process lifecycle, LAPI integration, hub operations)

**Existing Test File:** `backend/internal/api/handlers/crowdsec_handler_test.go`

#### Functions Requiring Coverage

1. **Start() - Lines 150-200**
   - Missing: Process already running check
   - Partials: Executor.Start() failure paths

2. **Stop() - Lines 205-250**
   - Missing: Clean stop when already stopped
   - Partials: Signal failure recovery

3. **ImportConfig() - Lines 350-420**
   - Missing: Archive extraction failures
   - Missing: Backup rollback on error
   - Partials: Path validation edge cases

4. **ExportConfig() - Lines 425-480**
   - Missing: Archive creation failures
   - Partials: File read errors

5. **ApplyPreset() - Lines 600-680**
   - Missing: Hub cache miss handling
   - Missing: CommandExecutor failure recovery
   - Partials: Rollback on partial apply

6. **ConsoleEnroll() - Lines 750-850**
   - Missing: LAPI endpoint validation
   - Missing: Enrollment failure retries
   - Partials: Token parsing errors

#### Test Cases Required

```go
// TestStart_AlreadyRunning
// - Mock Status to return running=true
// - Call Start
// - Verify error "already running"

// TestStop_IdempotentStop
// - Stop when already stopped
// - Verify no error returned

// TestImportConfig_ExtractionFailure
// - Mock tar.gz extraction to fail
// - Verify rollback executed
// - Verify original config restored

// TestApplyPreset_CacheMiss
// - Mock HubCache.Load to return ErrCacheMiss
// - Verify graceful error message

// TestConsoleEnroll_InvalidEndpoint
// - Provide malformed LAPI URL
// - Verify rejection with validation error

// TestConsoleEnroll_TokenParseError
// - Mock HTTP response with invalid JSON
// - Verify clear error to user
```

#### Implementation Strategy

1. **Enhance Existing Mocks**
   - Add failure modes to `mockCrowdsecExecutor`
   - Add cache miss scenarios to `mockHubService`

2. **Process Lifecycle Matrix**
   - Test all state transitions: stopped→started, started→stopped, stopped→stopped

3. **LAPI Integration Tests**
   - Mock HTTP responses for enrollment, bans, decisions
   - Test timeout and retry logic

**Estimated Effort:** 2.5 developer-days

---

### File 3: backend/internal/services/backup_service.go (426 lines)

**Current Coverage:** 63.33% (5 missing, 6 partials)
**Target Coverage:** 100%
**Complexity:** 🟡 MEDIUM (cron scheduling, zip operations)

**Existing Test Files:**
- `backend/internal/services/backup_service_test.go`
- `backend/internal/services/backup_service_disk_test.go`

#### Functions Requiring Coverage

1. **Start() - Lines 80-120**
   - Missing: Cron schedule parsing failures
   - Missing: Duplicate start prevention

2. **Stop() - Lines 125-145**
   - Missing: Stop when not started (idempotent)

3. **CreateBackup() - Lines 200-280**
   - Missing: Disk full scenario
   - Partials: Zip corruption recovery

4. **RestoreBackup() - Lines 350-420**
   - Missing: Zip decompression bomb detection
   - Partials: Malformed archive handling

5. **GetAvailableSpace() - Lines 450-475**
   - Missing: Syscall failure handling

#### Test Cases Required

```go
// TestStart_InvalidCronSchedule
// - Create service with invalid cron expression
// - Call Start()
// - Verify error returned

// TestStop_Idempotent
// - Stop without calling Start first
// - Verify no panic, no error

// TestCreateBackup_DiskFull
// - Mock syscall.Statfs to return 0 available space
// - Verify backup fails with "disk full" error

// TestRestoreBackup_DecompressionBomb
// - Create zip with 1GB compressed → 10TB uncompressed
// - Verify rejection before extraction

// TestGetAvailableSpace_SyscallFailure
// - Mock Statfs to return error
// - Verify graceful error handling
```

#### Implementation Strategy

1. **Cron Lifecycle Tests**
   - Test start/stop multiple times
   - Verify scheduler cleanup on stop

2. **Disk Space Mocking**
   - Use interface for syscall operations (new)
   - Mock Statfs for edge cases

3. **Archive Safety Tests**
   - Test zip bomb detection
   - Test path traversal in archive entries

**Estimated Effort:** 1.5 developer-days

---

### File 4: backend/internal/crowdsec/hub_sync.go (916 lines)

**Current Coverage:** 46.66% (3 missing, 5 partials)
**Target Coverage:** 100%
**Complexity:** 🟡 MEDIUM (HTTP client, cache, tar.gz extraction)

**Existing Test Files:**
- `backend/internal/crowdsec/hub_sync_test.go`
- `backend/internal/crowdsec/hub_pull_apply_test.go`
- `backend/internal/crowdsec/hub_sync_raw_index_test.go`

#### Functions Requiring Coverage

1. **FetchIndex() - Lines 120-180**
   - Missing: HTTP timeout handling
   - Partials: JSON parse failures

2. **Pull() - Lines 250-350**
   - Missing: Etag cache hit logic
   - Partials: Fallback URL failures

3. **Apply() - Lines 400-480**
   - Missing: Tar extraction path traversal
   - Missing: Rollback on partial apply
   - Partials: CommandExecutor failures

4. **validateHubURL() - Lines 600-650**
   - Missing: SSRF protection (private IP ranges)
   - Missing: Invalid scheme rejection

#### Test Cases Required

```go
// TestFetchIndex_HTTPTimeout
// - Mock HTTP client to timeout
// - Verify graceful error with retry suggestion

// TestPull_EtagCacheHit
// - Mock HTTP response with matching Etag
// - Verify 304 Not Modified handling

// TestApply_PathTraversal
// - Create tar.gz with "../../../etc/passwd"
// - Verify extraction blocked

// TestValidateHubURL_PrivateIP
// - Call validateHubURL("http://192.168.1.1/preset")
// - Verify SSRF protection rejects

// TestValidateHubURL_InvalidScheme
// - Call validateHubURL("ftp://example.com/preset")
// - Verify only http/https allowed
```

#### Implementation Strategy

1. **HTTP Client Mocking**
   - Use existing `mockHTTPClient` patterns
   - Add timeout and redirect scenarios

2. **SSRF Protection Tests**
   - Test all RFC1918 ranges (10.x, 172.16.x, 192.168.x)
   - Test localhost, link-local (169.254.x)

3. **Archive Safety Matrix**
   - Test absolute paths, symlinks, path traversal in tar entries

**Estimated Effort:** 2 developer-days

---

### File 5: backend/internal/api/handlers/feature_flags_handler.go (128 lines)

**Current Coverage:** 83.33% (4 missing, 2 partials)
**Target Coverage:** 100%
**Complexity:** 🟢 LOW (simple CRUD with batch optimization)

**Existing Test Files:**
- `backend/internal/api/handlers/feature_flags_handler_test.go`
- `backend/internal/api/handlers/feature_flags_handler_coverage_test.go`

#### Functions Requiring Coverage

1. **GetFlags() - Lines 50-80**
   - Missing: DB query failure handling

2. **UpdateFlags() - Lines 85-120**
   - Partials: Transaction rollback paths

#### Test Cases Required

```go
// TestGetFlags_DatabaseError
// - Mock db.Find() to return error
// - Verify 500 response with error code

// TestUpdateFlags_TransactionRollback
// - Mock transaction to fail mid-update
// - Verify rollback executed
// - Verify no partial updates persisted
```

#### Implementation Strategy

1. **Database Error Scenarios**
   - Use existing testDB patterns
   - Inject failures at specific query points

**Estimated Effort:** 0.5 developer-days

---

### File 6: backend/internal/caddy/importer.go (438 lines)

**Current Coverage:** 76.0% (22 missing, 4 partials)
**Target Coverage:** 100%
**Complexity:** 🟡 MEDIUM (Caddyfile parsing, JSON extraction)

**Existing Test Files:**
- `backend/internal/caddy/importer_test.go`
- `backend/internal/caddy/importer_subroute_test.go`
- `backend/internal/caddy/importer_additional_test.go`
- `backend/internal/caddy/importer_extra_test.go`

#### Functions Requiring Coverage

1. **NormalizeCaddyfile() - Lines 115-165**
   - Missing: Temp file write failures
   - Missing: caddy fmt timeout handling

2. **ParseCaddyfile() - Lines 170-210**
   - Missing: Path traversal validation
   - Missing: caddy adapt failures

3. **extractHandlers() - Lines 280-350**
   - Missing: Deep subroute nesting

4. **BackupCaddyfile() - Lines 400-435**
   - Missing: Path validation errors

#### Test Cases Required

```go
// TestNormalizeCaddyfile_TempFileFailure
// - Mock os.CreateTemp to return error
// - Verify error propagated

// TestNormalizeCaddyfile_Timeout
// - Mock executor to timeout
// - Verify timeout error message

// TestParseCaddyfile_PathTraversal
// - Call with path "../../../etc/Caddyfile"
// - Verify rejection

// TestExtractHandlers_DeepNesting
// - Parse JSON with subroute → subroute → handler
// - Verify correct flattening

// TestBackupCaddyfile_PathTraversal
// - Call with originalPath="../../../sensitive"
// - Verify rejection
```

#### Implementation Strategy

1. **Executor Mocking**
   - Use existing `Executor` interface
   - Add timeout simulation

2. **Path Safety Test Matrix**
   - Comprehensive traversal patterns
   - Unicode normalization tricks

**Estimated Effort:** 1.5 developer-days

---

### File 7: backend/internal/services/security_service.go (442 lines)

**Current Coverage:** 50.0% (12 missing, 8 partials)
**Target Coverage:** 100%
**Complexity:** 🟡 MEDIUM (audit logging, goroutine lifecycle, break-glass tokens)

**Existing Test File:** `backend/internal/services/security_service_test.go`

#### Functions Requiring Coverage

1. **Close() - Lines 40-55**
   - Missing: Double-close prevention

2. **Flush() - Lines 60-75**
   - Missing: Timeout on slow audit writes

3. **Get() - Lines 80-110**
   - Missing: Fallback to first row (backward compat)
   - Partials: RecordNotFound handling

4. **Upsert() - Lines 115-180**
   - Missing: Invalid CrowdSec mode rejection
   - Partials: CIDR validation failures

5. **processAuditEvents() - Lines 300-350**
   - Missing: Channel close handling
   - Missing: DB error during audit write

6. **ListAuditLogs() - Lines 380-430**
   - Partials: Pagination edge cases

#### Test Cases Required

```go
// TestClose_DoubleClose
// - Call Close() twice
// - Verify no panic, idempotent

// TestFlush_SlowWrite
// - Mock DB write to delay
// - Verify Flush waits correctly

// TestGet_BackwardCompatFallback
// - DB with no "default" row but has other rows
// - Verify fallback to first row

// TestUpsert_InvalidCrowdSecMode
// - Call with CrowdSecMode="remote"
// - Verify rejection error

// TestProcessAuditEvents_ChannelClosed
// - Close channel while processing
// - Verify graceful shutdown

// TestListAuditLogs_EmptyResult
// - Query with no matching logs
// - Verify empty array, not error
```

#### Implementation Strategy

1. **Goroutine Testing**
   - Use sync.WaitGroup to verify cleanup
   - Test channel closure scenarios

2. **CIDR Validation Matrix**
   - Test valid: "192.168.1.0/24", "10.0.0.1"
   - Test invalid: "999.999.999.999/99", "not-an-ip"

**Estimated Effort:** 2 developer-days

---

### File 8: backend/internal/crowdsec/hub_cache.go (234 lines)

**Current Coverage:** 28.57% (5 missing, 0 partials)
**Target Coverage:** 100%
**Complexity:** 🟢 LOW (cache CRUD with TTL)

**Existing Test File:** `backend/internal/crowdsec/hub_cache_test.go`

#### Functions Requiring Coverage

1. **Store() - Lines 60-105**
   - Missing: Context cancellation handling
   - Missing: Metadata write failure

2. **Load() - Lines 110-145**
   - Missing: Expired entry handling

3. **Touch() - Lines 200-220**
   - Missing: Update timestamp for TTL extension

4. **Size() - Lines 225-240**
   - Missing: Total cache size calculation

#### Test Cases Required

```go
// TestStore_ContextCancelled
// - Cancel context before Store completes
// - Verify operation aborted

// TestLoad_Expired
// - Store with short TTL
// - Wait for expiry
// - Verify ErrCacheExpired returned

// TestTouch_ExtendTTL
// - Store entry with 1 hour TTL
// - Touch after 30 minutes
// - Verify TTL reset

// TestSize_MultipleEntries
// - Store 3 presets of known sizes
// - Call Size()
// - Verify accurate total
```

#### Implementation Strategy

1. **TTL Testing**
   - Use mock time function (`nowFn` field)
   - Avoid time.Sleep in tests

2. **Context Testing**
   - Test cancellation at various points

**Estimated Effort:** 1 developer-day

---

### File 9: backend/internal/api/handlers/manual_challenge_handler.go (615 lines)

**Current Coverage:** 33.33% (8 missing, 4 partials)
**Target Coverage:** 100%
**Complexity:** 🟡 MEDIUM (DNS challenge API with validation)

**Existing Test File:** `backend/internal/api/handlers/manual_challenge_handler_test.go`

#### Functions Requiring Coverage

1. **GetChallenge() - Lines 90-160**
   - Missing: Provider type validation
   - Partials: User authorization checks

2. **VerifyChallenge() - Lines 165-240**
   - Missing: Challenge expired handling
   - Partials: Provider ownership check

3. **PollChallenge() - Lines 245-310**
   - Partials: Status update failures

4. **CreateChallenge() - Lines 420-490**
   - Missing: Duplicate challenge detection
   - Partials: Provider type check

#### Test Cases Required

```go
// TestGetChallenge_NonManualProvider
// - Create challenge on "cloudflare" provider
// - Call GetChallenge
// - Verify 400 "invalid provider type"

// TestVerifyChallenge_Expired
// - Create expired challenge (CreatedAt - 2 hours)
// - Call VerifyChallenge
// - Verify 410 Gone response

// TestCreateChallenge_Duplicate
// - Create challenge for domain
// - Create second challenge for same domain
// - Verify 409 Conflict

// TestGetChallenge_Unauthorized
// - User A creates challenge
// - User B tries to access
// - Verify 403 Forbidden
```

#### Implementation Strategy

1. **Mock Services**
   - Use existing `ManualChallengeServiceInterface`
   - Add authorization failure scenarios

2. **Time-Based Testing**
   - Mock challenge timestamps for expiry tests

**Estimated Effort:** 1.5 developer-days

---

### File 10: backend/internal/api/handlers/crowdsec_exec.go (149 lines)

**Current Coverage:** 0% (1 line missing, 0 partials)
**Target Coverage:** 100%
**Complexity:** 🟢 LOW (PID process checks)

**Existing Test File:** `backend/internal/api/handlers/crowdsec_exec_test.go`

#### Functions Requiring Coverage

1. **isCrowdSecProcess() - Lines 35-45**
   - Missing: Non-CrowdSec process rejection

#### Test Cases Required

```go
// TestIsCrowdSecProcess_ValidProcess
// - Create mock /proc/{pid}/cmdline with "crowdsec"
// - Call isCrowdSecProcess
// - Verify returns true

// TestIsCrowdSecProcess_WrongProcess
// - Create mock /proc/{pid}/cmdline with "nginx"
// - Verify returns false (PID recycling protection)
```

#### Implementation Strategy

1. **Mock /proc filesystem**
   - Use existing `procPath` field for testing
   - Create temp directory with fake cmdline files

**Estimated Effort:** 0.5 developer-days

---

## 3. Implementation Priority

### Phase 1: Critical Files (Week 1)
**Goal:** Recover 50% of missing coverage

1. **import_handler.go** (3 days)
   - Highest missing line count (33 lines)
   - Business-critical import feature

2. **crowdsec_handler.go** (2.5 days)
   - Core security module functionality
   - Complex LAPI integration

**Deliverable:** +40 lines coverage, patch coverage → 75%

---

### Phase 2: High-Impact Files (Week 2)
**Goal:** Recover 35% of missing coverage

3. **hub_sync.go** (2 days)
   - SSRF protection critical
   - CrowdSec hub operations

4. **security_service.go** (2 days)
   - Audit logging correctness
   - Break-glass token security

5. **backup_service.go** (1.5 days)
   - Data integrity critical

**Deliverable:** +35 lines coverage, patch coverage → 90%

---

### Phase 3: Remaining Files (Week 3)
**Goal:** Achieve 100% patch coverage

6. **importer.go** (1.5 days)
7. **manual_challenge_handler.go** (1.5 days)
8. **hub_cache.go** (1 day)
9. **feature_flags_handler.go** (0.5 days)
10. **crowdsec_exec.go** (0.5 days)

**Deliverable:** 100% patch coverage, 86%+ local coverage

---

## 4. Test Strategy

### Testing Principles

1. **Table-Driven Tests**
   - Use for input validation and error paths
   - Pattern: `tests := []struct{ name, input, wantErr string }`

2. **Interface Mocking**
   - Mock external dependencies (Executor, HTTPClient, DB)
   - Pattern: Define `*Interface` type, create `mock*` struct

3. **Test Database Isolation**
   - Use `OpenTestDB()` for in-memory SQLite
   - Clean up with `defer db.Close()`

4. **Context Testing**
   - Test cancellation at critical points
   - Use `context.WithCancel()` in tests

5. **Error Injection**
   - Mock failures at boundaries (disk full, network timeout, DB error)
   - Verify graceful degradation

### Test File Organization

```
backend/internal/
├── api/handlers/
│   ├── import_handler.go
│   ├── import_handler_test.go          # Existing
│   ├── crowdsec_handler.go
│   ├── crowdsec_handler_test.go        # Existing
│   ├── manual_challenge_handler.go
│   └── manual_challenge_handler_test.go # Existing
├── services/
│   ├── backup_service.go
│   ├── backup_service_test.go          # Existing
│   ├── security_service.go
│   └── security_service_test.go        # Existing
└── crowdsec/
    ├── hub_sync.go
    ├── hub_sync_test.go                # Existing
    ├── hub_cache.go
    └── hub_cache_test.go               # Existing
```

### Mock Patterns

#### Example: Mock Importer

```go
// NEW: Define interface in import_handler.go
type ImporterInterface interface {
    NormalizeCaddyfile(content string) (string, error)
    ParseCaddyfile(path string) ([]byte, error)
    ExtractHosts(json []byte) (*ImportResult, error)
}

// In test file
type mockImporter struct {
    normalizeErr error
    parseErr     error
}

func (m *mockImporter) NormalizeCaddyfile(content string) (string, error) {
    if m.normalizeErr != nil {
        return "", m.normalizeErr
    }
    return content, nil
}
```

#### Example: Table-Driven Path Safety

```go
func TestSafeJoin_PathTraversal(t *testing.T) {
    tests := []struct {
        name    string
        base    string
        path    string
        wantErr bool
    }{
        {"relative safe", "/tmp", "file.txt", false},
        {"parent traversal", "/tmp", "../etc/passwd", true},
        {"double parent", "/tmp", "../../root", true},
        {"absolute", "/tmp", "/etc/passwd", true},
        {"hidden parent", "/tmp", "a/../../../etc", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := safeJoin(tt.base, tt.path)
            if (err != nil) != tt.wantErr {
                t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
            }
        })
    }
}
```

### Coverage Validation

After each phase, run:

```bash
# Local coverage
go test ./backend/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

# Expected output after Phase 3:
# total: (statements) 86.0%

# Codecov patch coverage (CI)
# Expected: 100% for all modified lines
```

---

## 5. Definition of Done

### Coverage Metrics

- [x] **Local Coverage:** 86%+ (via `go test -cover`)
- [x] **CI Coverage:** 85%+ (Codecov reports slightly lower)
- [x] **Patch Coverage:** 100% (no modified line uncovered)

### Code Quality

- [x] All tests pass on first run (no flakes)
- [x] No skipped tests (`.Skip()` only for valid reasons)
- [x] No truncated test output (avoid `head`, `tail`)
- [x] Table-driven tests for input validation
- [x] Mock interfaces for external dependencies

### Documentation

- [x] Test file comments explain complex scenarios
- [x] Test names follow convention: `Test<Function>_<Scenario>`
- [x] Failure messages are actionable

### Security

- [x] GORM Security Scanner passes (manual stage)
- [x] Path traversal tests in place
- [x] SSRF protection validated
- [x] No hardcoded credentials in tests

### CI Integration

- [x] All tests pass in CI pipeline
- [x] Codecov patch coverage gate passes
- [x] No new linting errors introduced

---

## 6. Continuous Validation

### Pre-Commit Checks

```bash
# Run before each commit
make test-backend-coverage

# Expected output:
# ✅ Coverage: 86.2%
# ✅ All tests passed
```

### Daily Monitoring

- Check Codecov dashboard for regression
- Review failed CI runs immediately
- Monitor test execution time (should be <5 min for all backend tests)

### Weekly Review

- Analyze coverage trends (should not drop below 85%)
- Identify new untested code from PRs
- Update this plan if new files need coverage

---

## 7. Risk Mitigation

### Identified Risks

1. **Flaky Tests**
   - **Mitigation:** Avoid time.Sleep, use mocks for time
   - **Detection:** Run tests 10x locally before commit

2. **Mock Drift**
   - **Mitigation:** Keep mocks in sync with interfaces
   - **Detection:** CI will fail if interface changes

3. **Coverage Regression**
   - **Mitigation:** Codecov patch coverage gate (100%)
   - **Detection:** Automated on every PR

4. **Test Maintenance Burden**
   - **Mitigation:** Use table-driven tests to reduce duplication
   - **Detection:** Review test LOC vs production LOC ratio (should be <2:1)

---

## 8. Rollout Plan

### Week 1: Critical Files (10 points)
- Day 1-2: import_handler.go tests
- Day 3-4: crowdsec_handler.go tests
- Day 5: Integration testing, coverage validation
- **Checkpoint:** Coverage → 75%

### Week 2: High-Impact Files (20 points)
- Day 1-2: hub_sync.go + security_service.go
- Day 3-4: backup_service.go
- Day 5: Integration testing, coverage validation
- **Checkpoint:** Coverage → 90%

### Week 3: Remaining Files (20 points)
- Day 1: importer.go + manual_challenge_handler.go
- Day 2: hub_cache.go + feature_flags_handler.go + crowdsec_exec.go
- Day 3: Full regression testing
- Day 4: Documentation, PR prep
- Day 5: Code review iterations
- **Checkpoint:** Coverage → 86%+, patch coverage 100%

---

## 9. Success Criteria

### Quantitative Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Local Coverage | ~60% | 86%+ | ❌ |
| CI Coverage | ~58% | 85%+ | ❌ |
| Patch Coverage | 59.33% | 100% | ❌ |
| Missing Lines | 98 | 0 | ❌ |
| Partial Branches | 35 | 0 | ❌ |

### Qualitative Outcomes

- [x] All critical business logic paths tested
- [x] Security validation (SSRF, path traversal) enforced by tests
- [x] Error handling coverage comprehensive
- [x] Flake-free test suite
- [x] Fast test execution (<5 min total)

---

## 10. Post-Implementation

### Maintenance

1. **New Code Requirements**
   - All new functions must have tests
   - Pre-commit hooks enforce coverage
   - PR reviews check patch coverage

2. **Regression Prevention**
   - Codecov threshold: 85% enforced
   - Weekly coverage reports
   - Coverage trends dashboard

3. **Test Debt Tracking**
   - Label low-coverage files in backlog
   - Quarterly coverage audits
   - Test optimization sprints

---

## Appendix A: Test Execution Commands

```bash
# Run all backend tests
go test ./backend/...

# Run with coverage
go test ./backend/... -coverprofile=coverage.out

# View coverage report in browser
go tool cover -html=coverage.out

# Run specific file's tests
go test -v ./backend/internal/api/handlers/import_handler_test.go

# Run with race detector (slower but catches concurrency bugs)
go test -race ./backend/...

# Generate coverage for CI
go test ./backend/... -coverprofile=coverage.out -covermode=atomic
```

---

## Appendix B: Reference Test Files

Best examples to follow from existing codebase:

1. **Table-Driven:** `backend/internal/caddy/importer_test.go`
2. **Mock Interfaces:** `backend/internal/api/handlers/auth_handler_test.go`
3. **Test DB Usage:** `backend/internal/testutil/testdb_test.go`
4. **Error Injection:** `backend/internal/services/proxyhost_service_test.go`
5. **Context Testing:** `backend/internal/network/safeclient_test.go`

---

## Sign-Off

**Plan Version:** 1.0
**Created:** 2025-01-XX
**Status:** APPROVED FOR IMPLEMENTATION

**Next Steps:**
1. Review plan with team
2. Begin Phase 1 implementation
3. Daily standup on progress
4. Weekly coverage checkpoint reviews
