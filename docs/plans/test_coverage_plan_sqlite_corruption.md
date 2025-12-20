# Test Coverage Plan - SQLite Corruption Guardrails

**Target**: 85%+ coverage across all files
**Current Status**: 72.16% patch coverage (27 lines missing)
**Date**: December 17, 2025

## Executive Summary

Codecov reports 72.16% patch coverage with 27 lines missing across 4 files:

1. `backup_service.go` - 60.71% (6 missing, 5 partials)
2. `database.go` - 28.57% (5 missing, 5 partials)
3. `db_health_handler.go` - 86.95% (2 missing, 1 partial)
4. `errors.go` - 86.95% (2 missing, 1 partial)

**Root Cause**: Missing test coverage for error paths, logger calls, partial conditionals, and edge cases.

---

## 1. backup_service.go (Target: 85%+)

### Current Coverage: 60.71%

**Missing**: 6 lines | **Partial**: 5 lines

### Uncovered Code Paths

#### A. NewBackupService Constructor Error Paths

**Lines**: 36-37, 49-50

```go
if err := os.MkdirAll(backupDir, 0o755); err != nil {
    logger.Log().WithError(err).Error("Failed to create backup directory")
}
...
if err != nil {
    logger.Log().WithError(err).Error("Failed to schedule backup")
}
```

**Analysis**:

- Constructor logs errors but doesn't return them
- Tests never trigger these error paths
- No verification that logging actually occurs

#### B. RunScheduledBackup Error Branching

**Lines**: 61-71 (partial coverage on conditionals)

```go
if name, err := s.CreateBackup(); err != nil {
    logger.Log().WithError(err).Error("Scheduled backup failed")
} else {
    logger.Log().WithField("backup", name).Info("Scheduled backup created")

    if deleted, err := s.CleanupOldBackups(DefaultBackupRetention); err != nil {
        logger.Log().WithError(err).Warn("Failed to cleanup old backups")
    } else if deleted > 0 {
        logger.Log().WithField("deleted_count", deleted).Info("Cleaned up old backups")
    }
}
```

**Analysis**:

- Test only covers success path
- Failure path (backup creation fails) not tested
- Cleanup failure path not tested
- No verification of deleted = 0 branch

#### C. CleanupOldBackups Edge Cases

**Lines**: 98-103

```go
if err := s.DeleteBackup(backup.Filename); err != nil {
    logger.Log().WithError(err).WithField("filename", backup.Filename).Warn("Failed to delete old backup")
    continue
}
deleted++
logger.Log().WithField("filename", backup.Filename).Debug("Deleted old backup")
```

**Analysis**:

- Tests don't cover partial deletion failure (some succeed, some fail)
- Logger.Debug() call never exercised

#### D. GetLastBackupTime Error Path

**Lines**: 112-113

```go
if err != nil {
    return time.Time{}, err
}
```

**Analysis**: Error path when ListBackups fails (directory read error) not tested

#### E. CreateBackup Caddy Directory Warning

**Lines**: 186-188

```go
if err := s.addDirToZip(w, caddyDir, "caddy"); err != nil {
    logger.Log().WithError(err).Warn("Warning: could not backup caddy dir")
}
```

**Analysis**: Warning path never triggered (tests always have valid caddy dirs)

#### F. addToZip Error Handling

**Lines**: 192-202 (partial coverage)

```go
file, err := os.Open(srcPath)
if err != nil {
    if os.IsNotExist(err) {
        return nil  // Not covered
    }
    return err
}
defer func() {
    if err := file.Close(); err != nil {
        logger.Log().WithError(err).Warn("failed to close file after adding to zip")
    }
}()
```

**Analysis**:

- File not found path returns nil (silent skip) - not tested
- File close error in defer not tested
- File open error (other than not found) not tested

### Required Tests

#### Test 1: NewBackupService_BackupDirCreationError

```go
func TestNewBackupService_BackupDirCreationError(t *testing.T)
```

**Setup**:

- Create parent directory as read-only (chmod 0444)
- Attempt to initialize service
**Assert**:
- Service still returns (error is logged, not returned)
- Verify logging occurred (use test logger hook or check it doesn't panic)

#### Test 2: NewBackupService_CronScheduleError

```go
func TestNewBackupService_CronScheduleError(t *testing.T)
```

**Setup**:

- Use invalid cron expression (requires modifying code or mocking cron)
- Alternative: Just verify current code doesn't panic
**Assert**:
- Service initializes without panic
- Cron error is logged

#### Test 3: RunScheduledBackup_CreateBackupFails

```go
func TestRunScheduledBackup_CreateBackupFails(t *testing.T)
```

**Setup**:

- Delete database file after service creation
- Call RunScheduledBackup()
**Assert**:
- No panic occurs
- Backup failure is logged
- CleanupOldBackups is NOT called

#### Test 4: RunScheduledBackup_CleanupFails

```go
func TestRunScheduledBackup_CleanupFails(t *testing.T)
```

**Setup**:

- Create valid backup
- Make backup directory read-only before cleanup
- Call RunScheduledBackup()
**Assert**:
- Backup creation succeeds
- Cleanup warning is logged
- Service continues running

#### Test 5: RunScheduledBackup_CleanupDeletesZero

```go
func TestRunScheduledBackup_CleanupDeletesZero(t *testing.T)
```

**Setup**:

- Create only 1 backup (below DefaultBackupRetention)
- Call RunScheduledBackup()
**Assert**:
- deleted = 0
- No deletion log message (only when deleted > 0)

#### Test 6: CleanupOldBackups_PartialFailure

```go
func TestCleanupOldBackups_PartialFailure(t *testing.T)
```

**Setup**:

- Create 10 backups
- Make 3 of them read-only (chmod 0444 on parent dir or file)
- Call CleanupOldBackups(3)
**Assert**:
- Returns deleted count < expected
- Logs warning for each failed deletion
- Continues with other deletions

#### Test 7: GetLastBackupTime_ListBackupsError

```go
func TestGetLastBackupTime_ListBackupsError(t *testing.T)
```

**Setup**:

- Set BackupDir to a file instead of directory
- Call GetLastBackupTime()
**Assert**:
- Returns error
- Returns zero time

#### Test 8: CreateBackup_CaddyDirMissing

```go
func TestCreateBackup_CaddyDirMissing(t *testing.T)
```

**Setup**:

- Create DB but no caddy directory
- Call CreateBackup()
**Assert**:
- Backup succeeds (warning logged)
- Zip contains DB but not caddy/

#### Test 9: CreateBackup_CaddyDirUnreadable

```go
func TestCreateBackup_CaddyDirUnreadable(t *testing.T)
```

**Setup**:

- Create caddy dir with no read permissions (chmod 0000)
- Call CreateBackup()
**Assert**:
- Logs warning about caddy dir
- Backup still succeeds with DB only

#### Test 10: addToZip_FileNotFound

```go
func TestBackupService_addToZip_FileNotFound(t *testing.T)
```

**Setup**:

- Directly call addToZip with non-existent file path
- Mock zip.Writer
**Assert**:
- Returns nil (silent skip)
- No error logged

#### Test 11: addToZip_FileOpenError

```go
func TestBackupService_addToZip_FileOpenError(t *testing.T)
```

**Setup**:

- Create file with no read permissions (chmod 0000)
- Call addToZip
**Assert**:
- Returns permission denied error
- Does NOT return nil

#### Test 12: addToZip_FileCloseError

```go
func TestBackupService_addToZip_FileCloseError(t *testing.T)
```

**Setup**:

- Mock file.Close() to return error (requires refactoring or custom closer)
- Alternative: Test with actual bad file descriptor scenario
**Assert**:
- Logs close error warning
- Still succeeds in adding to zip

---

## 2. database.go (Target: 85%+)

### Current Coverage: 28.57%

**Missing**: 5 lines | **Partial**: 5 lines

### Uncovered Code Paths

#### A. Connect Error Paths

**Lines**: 36-37, 42-43

```go
if err != nil {
    return nil, fmt.Errorf("open database: %w", err)
}
...
if err != nil {
    return nil, fmt.Errorf("get underlying db: %w", err)
}
```

**Analysis**:

- Test `TestConnect_Error` only tests invalid directory
- Doesn't test GORM connection failure
- Doesn't test sqlDB.DB() failure

#### B. Journal Mode Verification Warning

**Lines**: 49-50

```go
if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
    logger.Log().WithError(err).Warn("Failed to verify SQLite journal mode")
}
```

**Analysis**: Error path not tested (PRAGMA query fails)

#### C. Integrity Check on Startup Warnings

**Lines**: 57-58, 63-65

```go
if err := db.Raw("PRAGMA quick_check").Scan(&quickCheckResult).Error; err != nil {
    logger.Log().WithError(err).Warn("Failed to run SQLite integrity check on startup")
} else if quickCheckResult == "ok" {
    logger.Log().Info("SQLite database integrity check passed")
} else {
    logger.Log().WithField("quick_check_result", quickCheckResult).
        WithField("error_type", "database_corruption").
        Error("SQLite database integrity check failed - database may be corrupted")
}
```

**Analysis**:

- PRAGMA failure path not tested
- Corruption detected path (quickCheckResult != "ok") not tested
- Only success path tested in TestConnect_WALMode

### Required Tests

#### Test 13: Connect_InvalidDSN

```go
func TestConnect_InvalidDSN(t *testing.T)
```

**Setup**:

- Use completely invalid DSN (e.g., empty string or malformed path)
- Call Connect()
**Assert**:
- Returns error wrapped with "open database:"
- Database is nil

#### Test 14: Connect_PRAGMAJournalModeError

```go
func TestConnect_PRAGMAJournalModeError(t *testing.T)
```

**Setup**:

- Create corrupted database file (invalid SQLite header)
- Call Connect() - it may succeed connection but fail PRAGMA
**Assert**:
- Connection may succeed (GORM doesn't validate immediately)
- Warning logged for journal mode verification failure
- Function still returns database (doesn't fail on PRAGMA)

#### Test 15: Connect_IntegrityCheckError

```go
func TestConnect_IntegrityCheckError(t *testing.T)
```

**Setup**:

- Mock or create scenario where PRAGMA quick_check query fails
- Alternative: Use read-only database with corrupted WAL file
**Assert**:
- Warning logged for integrity check failure
- Connection still returns successfully (non-blocking)

#### Test 16: Connect_IntegrityCheckCorrupted

```go
func TestConnect_IntegrityCheckCorrupted(t *testing.T)
```

**Setup**:

- Create SQLite DB and intentionally corrupt it (truncate file, modify header)
- Call Connect()
**Assert**:
- PRAGMA quick_check returns non-"ok" result
- Error logged with "database_corruption" type
- Connection still returns (non-fatal during startup)

#### Test 17: Connect_PRAGMAVerification

```go
func TestConnect_PRAGMAVerification(t *testing.T)
```

**Setup**:

- Create normal database
- Verify all PRAGMA settings applied correctly
**Assert**:
- journal_mode = "wal"
- busy_timeout = 5000
- synchronous = NORMAL (1)
- Info log message contains "WAL mode enabled"

#### Test 18: Connect_CorruptedDatabase_FullIntegrationScenario

```go
func TestConnect_CorruptedDatabase_FullIntegrationScenario(t *testing.T)
```

**Setup**:

- Create valid DB with tables/data
- Corrupt the database file (overwrite with random bytes in middle)
- Attempt Connect()
**Assert**:
- Connection may succeed initially
- quick_check detects corruption
- Appropriate error logged with corruption details
- Function returns database anyway (allows recovery attempts)

---

## 3. db_health_handler.go (Target: 90%+)

### Current Coverage: 86.95%

**Missing**: 2 lines | **Partial**: 1 line

### Uncovered Code Paths

#### A. Corrupted Database Response

**Lines**: 69-71

```go
} else {
    response.Status = "corrupted"
    c.JSON(http.StatusServiceUnavailable, response)
}
```

**Analysis**: All tests use healthy in-memory databases; corruption path never tested

#### B. Backup Service GetLastBackupTime Error

**Lines**: 56-58 (partial coverage)

```go
if h.backupService != nil {
    if lastBackup, err := h.backupService.GetLastBackupTime(); err == nil && !lastBackup.IsZero() {
        response.LastBackup = &lastBackup
    }
}
```

**Analysis**: Error case (err != nil) or lastBackup.IsZero() not tested

### Required Tests

#### Test 19: DBHealthHandler_Check_CorruptedDatabase

```go
func TestDBHealthHandler_Check_CorruptedDatabase(t *testing.T)
```

**Setup**:

- Create file-based SQLite database
- Corrupt the database file (truncate or write invalid data)
- Create handler with corrupted DB
- Call Check endpoint
**Assert**:
- Returns 503 Service Unavailable
- response.Status = "corrupted"
- response.IntegrityOK = false
- response.IntegrityResult contains error details

#### Test 20: DBHealthHandler_Check_BackupServiceError

```go
func TestDBHealthHandler_Check_BackupServiceError(t *testing.T)
```

**Setup**:

- Create handler with backup service
- Make backup directory unreadable (trigger GetLastBackupTime error)
- Call Check endpoint
**Assert**:
- Handler still succeeds (error is swallowed)
- response.LastBackup = nil
- Response status remains "healthy" (independent of backup error)

#### Test 21: DBHealthHandler_Check_BackupTimeZero

```go
func TestDBHealthHandler_Check_BackupTimeZero(t *testing.T)
```

**Setup**:

- Create handler with backup service but empty backup directory
- Call Check endpoint
**Assert**:
- response.LastBackup = nil (not set when zero time)
- No error
- Status remains "healthy"

---

## 4. errors.go (Target: 90%+)

### Current Coverage: 86.95%

**Missing**: 2 lines | **Partial**: 1 line

### Uncovered Code Paths

#### A. LogCorruptionError with Empty Context

**Lines**: Not specifically visible, but likely the context iteration logic

```go
for key, value := range context {
    entry = entry.WithField(key, value)
}
```

**Analysis**: Tests call with nil and with context, but may not cover empty map {}

#### B. CheckIntegrity Error Path Details

**Lines**: Corruption message path

```go
return false, result
```

**Analysis**: Test needs actual corruption scenario (not just mocked)

### Required Tests

#### Test 22: LogCorruptionError_EmptyContext

```go
func TestLogCorruptionError_EmptyContext(t *testing.T)
```

**Setup**:

- Call LogCorruptionError with empty map {}
- Verify doesn't panic
**Assert**:
- No panic
- Error is logged with base fields only

#### Test 23: CheckIntegrity_ActualCorruption

```go
func TestCheckIntegrity_ActualCorruption(t *testing.T)
```

**Setup**:

- Create SQLite database
- Insert data
- Corrupt the database file (overwrite bytes)
- Attempt to reconnect
- Call CheckIntegrity
**Assert**:
- Returns healthy=false
- message contains corruption details (not just "ok")
- Message includes specific SQLite error

#### Test 24: CheckIntegrity_PRAGMAError

```go
func TestCheckIntegrity_PRAGMAError(t *testing.T)
```

**Setup**:

- Close database connection
- Call CheckIntegrity on closed DB
**Assert**:
- Returns healthy=false
- message contains "failed to run integrity check:" + error
- Error describes connection/query failure

---

## Implementation Priority

### Phase 1: Critical Coverage Gaps (Target: +10% coverage)

1. **Test 19**: DBHealthHandler_Check_CorruptedDatabase (closes 503 status path)
2. **Test 16**: Connect_IntegrityCheckCorrupted (closes database.go corruption path)
3. **Test 23**: CheckIntegrity_ActualCorruption (closes errors.go corruption path)
4. **Test 3**: RunScheduledBackup_CreateBackupFails (closes backup failure branch)

**Impact**: Covers all "corrupted database" scenarios - the core feature functionality

### Phase 2: Error Path Coverage (Target: +8% coverage)

1. **Test 7**: GetLastBackupTime_ListBackupsError
2. **Test 20**: DBHealthHandler_Check_BackupServiceError
3. **Test 14**: Connect_PRAGMAJournalModeError
4. **Test 15**: Connect_IntegrityCheckError

**Impact**: Covers error handling paths that log warnings but don't fail

### Phase 3: Edge Cases (Target: +5% coverage)

1. **Test 5**: RunScheduledBackup_CleanupDeletesZero
2. **Test 21**: DBHealthHandler_Check_BackupTimeZero
3. **Test 6**: CleanupOldBackups_PartialFailure
4. **Test 8**: CreateBackup_CaddyDirMissing

**Impact**: Handles edge cases and partial failures

### Phase 4: Constructor & Initialization (Target: +2% coverage)

1. **Test 1**: NewBackupService_BackupDirCreationError
2. **Test 2**: NewBackupService_CronScheduleError
3. **Test 17**: Connect_PRAGMAVerification

**Impact**: Tests initialization edge cases

### Phase 5: Deep Coverage (Final +3%)

1. **Test 10**: addToZip_FileNotFound
2. **Test 11**: addToZip_FileOpenError
3. **Test 9**: CreateBackup_CaddyDirUnreadable
4. **Test 22**: LogCorruptionError_EmptyContext
5. **Test 24**: CheckIntegrity_PRAGMAError

**Impact**: Achieves 90%+ coverage with comprehensive edge case testing

---

## Testing Utilities Needed

### 1. Database Corruption Helper

```go
// helper_test.go
func corruptSQLiteDB(t *testing.T, dbPath string) {
    t.Helper()
    // Open and corrupt file at specific offset
    // Overwrite SQLite header or page data
    f, err := os.OpenFile(dbPath, os.O_RDWR, 0644)
    require.NoError(t, err)
    defer f.Close()

    // Corrupt SQLite header magic number
    _, err = f.WriteAt([]byte("CORRUPT"), 0)
    require.NoError(t, err)
}
```

### 2. Directory Permission Helper

```go
func makeReadOnly(t *testing.T, path string) func() {
    t.Helper()
    original, err := os.Stat(path)
    require.NoError(t, err)

    err = os.Chmod(path, 0444)
    require.NoError(t, err)

    return func() {
        os.Chmod(path, original.Mode())
    }
}
```

### 3. Test Logger Hook

```go
type TestLoggerHook struct {
    Entries []*logrus.Entry
    mu      sync.Mutex
}

func (h *TestLoggerHook) Fire(entry *logrus.Entry) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.Entries = append(h.Entries, entry)
    return nil
}

func (h *TestLoggerHook) Levels() []logrus.Level {
    return logrus.AllLevels
}

func (h *TestLoggerHook) HasMessage(msg string) bool {
    h.mu.Lock()
    defer h.mu.Unlock()
    for _, e := range h.Entries {
        if strings.Contains(e.Message, msg) {
            return true
        }
    }
    return false
}
```

### 4. Mock Backup Service

```go
type MockBackupService struct {
    GetLastBackupTimeErr    error
    GetLastBackupTimeReturn time.Time
}

func (m *MockBackupService) GetLastBackupTime() (time.Time, error) {
    return m.GetLastBackupTimeReturn, m.GetLastBackupTimeErr
}
```

---

## Coverage Verification Commands

After implementing tests, run:

```bash
# Backend coverage
./scripts/go-test-coverage.sh

# Specific file coverage
go test -coverprofile=coverage.out ./backend/internal/services
go tool cover -func=coverage.out | grep backup_service.go

# HTML report for visual verification
go tool cover -html=coverage.out -o coverage.html
```

**Target Output**:

```
backup_service.go:       87.5%
database.go:             88.2%
db_health_handler.go:    92.3%
errors.go:               91.7%
```

---

## Success Criteria

✅ **All 24 tests implemented**
✅ **Codecov patch coverage ≥ 85%**
✅ **All pre-commit checks pass**
✅ **No failing tests in CI**
✅ **Coverage report shows green on all 4 files**

## Notes

- Some tests require actual file system manipulation (corruption, permissions)
- Logger output verification may need test hooks (logrus has built-in test hooks)
- Defer error paths are difficult to test - may need refactoring for testability
- GORM/SQLite integration tests require real database files (not just mocks)
- Consider adding integration tests that combine multiple failure scenarios
- Tests for `addToZip` may need to use temporary wrapper or interface for better testability
- Some error paths (like cron schedule errors) may require code refactoring to be fully testable

---

*Plan created: December 17, 2025*
