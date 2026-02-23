# Database Corruption Guardrails Implementation Plan

**Status:** 📋 Planning
**Date:** 2024-12-17
**Priority:** High
**Epic:** Database Resilience

## Overview

This plan implements proactive guardrails to detect, prevent, and recover from SQLite database corruption. The implementation builds on existing patterns in the codebase and integrates with the current backup infrastructure.

---

## 1. Startup Integrity Check

**Location:** `backend/internal/database/database.go`

### Design

Add `PRAGMA quick_check` after database connection is established. This is a faster variant of `integrity_check` suitable for startup—it verifies B-tree page structure without checking row data.

### Implementation

#### Modify `Connect()` function in `database.go`

```go
// After line 53 (after WAL mode verification):

// Run quick integrity check on startup
var integrityResult string
if err := db.Raw("PRAGMA quick_check").Scan(&integrityResult).Error; err != nil {
    logger.Log().WithError(err).Error("Failed to run database integrity check")
} else if integrityResult != "ok" {
    logger.Log().WithFields(logrus.Fields{
        "result":       integrityResult,
        "database":     dbPath,
        "action":       "startup_integrity_check",
        "severity":     "critical",
    }).Error("⚠️ DATABASE CORRUPTION DETECTED - Run db-recovery.sh to repair")
} else {
    logger.Log().Info("Database integrity check passed")
}
```

### Behavior

- **If OK:** Log info and continue normally
- **If NOT OK:** Log critical error with structured fields, DO NOT block startup
- **Error running check:** Log warning, continue startup

### Test Requirements

Create `backend/internal/database/database_test.go`:

```go
func TestConnect_IntegrityCheckLogged(t *testing.T) {
    // Test that integrity check is performed on valid DB
}

func TestConnect_CorruptedDBWarnsButContinues(t *testing.T) {
    // Create intentionally corrupted DB, verify warning logged but startup succeeds
}
```

---

## 2. Corruption Sentinel Logging

**Location:** `backend/internal/database/errors.go` (new file)

### Design

Create a helper that wraps database errors, detects corruption signatures, emits structured logs, and optionally triggers a one-time integrity check.

### New File: `backend/internal/database/errors.go`

```go
package database

import (
    "strings"
    "sync"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/sirupsen/logrus"
    "gorm.io/gorm"
)

// Corruption error signatures
var corruptionSignatures = []string{
    "database disk image is malformed",
    "database or disk is full",
    "file is encrypted or is not a database",
    "disk I/O error",
}

// Singleton to track if we've already triggered integrity check
var (
    integrityCheckTriggered bool
    integrityCheckMutex     sync.Mutex
)

// CorruptionContext provides structured context for corruption errors
type CorruptionContext struct {
    Table     string
    Operation string
    MonitorID string
    HostID    string
    Extra     map[string]interface{}
}

// WrapDBError checks for corruption errors and logs them with context.
// Returns the original error unchanged.
func WrapDBError(err error, ctx CorruptionContext) error {
    if err == nil {
        return nil
    }

    errStr := err.Error()
    for _, sig := range corruptionSignatures {
        if strings.Contains(strings.ToLower(errStr), strings.ToLower(sig)) {
            logCorruptionError(err, ctx)
            triggerOneTimeIntegrityCheck()
            return err
        }
    }
    return err
}

// IsCorruptionError checks if an error indicates database corruption
func IsCorruptionError(err error) bool {
    if err == nil {
        return false
    }
    errStr := strings.ToLower(err.Error())
    for _, sig := range corruptionSignatures {
        if strings.Contains(errStr, strings.ToLower(sig)) {
            return true
        }
    }
    return false
}

func logCorruptionError(err error, ctx CorruptionContext) {
    fields := logrus.Fields{
        "error":      err.Error(),
        "severity":   "critical",
        "event_type": "database_corruption",
    }

    if ctx.Table != "" {
        fields["table"] = ctx.Table
    }
    if ctx.Operation != "" {
        fields["operation"] = ctx.Operation
    }
    if ctx.MonitorID != "" {
        fields["monitor_id"] = ctx.MonitorID
    }
    if ctx.HostID != "" {
        fields["host_id"] = ctx.HostID
    }
    for k, v := range ctx.Extra {
        fields[k] = v
    }

    logger.Log().WithFields(fields).Error("🔴 DATABASE CORRUPTION ERROR - Run scripts/db-recovery.sh")
}

var integrityCheckDB *gorm.DB

// SetIntegrityCheckDB sets the DB instance for integrity checks
func SetIntegrityCheckDB(db *gorm.DB) {
    integrityCheckDB = db
}

func triggerOneTimeIntegrityCheck() {
    integrityCheckMutex.Lock()
    defer integrityCheckMutex.Unlock()

    if integrityCheckTriggered || integrityCheckDB == nil {
        return
    }
    integrityCheckTriggered = true

    go func() {
        logger.Log().Info("Triggering integrity check after corruption detection...")
        var result string
        if err := integrityCheckDB.Raw("PRAGMA integrity_check").Scan(&result).Error; err != nil {
            logger.Log().WithError(err).Error("Integrity check failed to run")
            return
        }

        if result != "ok" {
            logger.Log().WithField("result", result).Error("🔴 INTEGRITY CHECK FAILED - Database requires recovery")
        } else {
            logger.Log().Info("Integrity check passed (corruption may be in specific rows)")
        }
    }()
}

// ResetIntegrityCheckFlag resets the one-time flag (for testing)
func ResetIntegrityCheckFlag() {
    integrityCheckMutex.Lock()
    integrityCheckTriggered = false
    integrityCheckMutex.Unlock()
}
```

### Usage Example (uptime_service.go)

```go
// In GetMonitorHistory:
func (s *UptimeService) GetMonitorHistory(id string, limit int) ([]models.UptimeHeartbeat, error) {
    var heartbeats []models.UptimeHeartbeat
    result := s.DB.Where("monitor_id = ?", id).Order("created_at desc").Limit(limit).Find(&heartbeats)

    // Wrap error to detect and log corruption
    err := database.WrapDBError(result.Error, database.CorruptionContext{
        Table:     "uptime_heartbeats",
        Operation: "SELECT",
        MonitorID: id,
    })
    return heartbeats, err
}
```

### Test Requirements

Create `backend/internal/database/errors_test.go`:

```go
func TestIsCorruptionError(t *testing.T)
func TestWrapDBError_DetectsCorruption(t *testing.T)
func TestWrapDBError_NonCorruptionPassthrough(t *testing.T)
func TestTriggerOneTimeIntegrityCheck_OnlyOnce(t *testing.T)
```

---

## 3. Enhanced Auto-Backup Service

**Location:** `backend/internal/services/backup_service.go` (existing file)

### Design

The backup service already exists with daily 3 AM scheduling. We need to:

1. Add configurable retention (currently no cleanup implemented in scheduled backups)
2. Expose last backup time for health endpoint
3. Add backup retention cleanup

### Modifications to `backup_service.go`

#### Add retention cleanup after scheduled backup

```go
// Add constant at top of file
const DefaultBackupRetention = 7

// Modify RunScheduledBackup():
func (s *BackupService) RunScheduledBackup() {
    logger.Log().Info("Starting scheduled backup")
    if name, err := s.CreateBackup(); err != nil {
        logger.Log().WithError(err).Error("Scheduled backup failed")
    } else {
        logger.Log().WithField("backup", name).Info("Scheduled backup created")
        // Cleanup old backups
        s.cleanupOldBackups(DefaultBackupRetention)
    }
}

// Add new method:
func (s *BackupService) cleanupOldBackups(keep int) {
    backups, err := s.ListBackups()
    if err != nil {
        logger.Log().WithError(err).Warn("Failed to list backups for cleanup")
        return
    }

    // Backups are already sorted newest first
    if len(backups) <= keep {
        return
    }

    for _, backup := range backups[keep:] {
        if err := s.DeleteBackup(backup.Filename); err != nil {
            logger.Log().WithError(err).WithField("filename", backup.Filename).Warn("Failed to delete old backup")
        } else {
            logger.Log().WithField("filename", backup.Filename).Info("Deleted old backup")
        }
    }
}

// Add new method for health endpoint:
func (s *BackupService) GetLastBackupTime() (*time.Time, error) {
    backups, err := s.ListBackups()
    if err != nil {
        return nil, err
    }
    if len(backups) == 0 {
        return nil, nil
    }
    return &backups[0].Time, nil
}
```

### Test Requirements

Add to `backend/internal/services/backup_service_test.go`:

```go
func TestCleanupOldBackups_KeepsRetentionCount(t *testing.T)
func TestGetLastBackupTime_ReturnsNewestBackup(t *testing.T)
func TestGetLastBackupTime_ReturnsNilWhenNoBackups(t *testing.T)
```

---

## 4. Database Health Endpoint

**Location:** `backend/internal/api/handlers/db_health_handler.go` (new file)

### Design

Add a new endpoint `GET /api/v1/health/db` that:

1. Runs `PRAGMA quick_check`
2. Returns 200 if healthy, 503 if corrupted
3. Includes last backup time in response

### New File: `backend/internal/api/handlers/db_health_handler.go`

```go
package handlers

import (
    "net/http"
    "time"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/Wikid82/charon/backend/internal/services"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

// DBHealthHandler handles database health check requests
type DBHealthHandler struct {
    db            *gorm.DB
    backupService *services.BackupService
}

// NewDBHealthHandler creates a new DBHealthHandler
func NewDBHealthHandler(db *gorm.DB, backupService *services.BackupService) *DBHealthHandler {
    return &DBHealthHandler{
        db:            db,
        backupService: backupService,
    }
}

// DBHealthResponse represents the response from the DB health check
type DBHealthResponse struct {
    Status          string  `json:"status"`
    IntegrityCheck  string  `json:"integrity_check"`
    LastBackupTime  *string `json:"last_backup_time"`
    BackupAvailable bool    `json:"backup_available"`
}

// Check performs a database integrity check and returns the health status.
// Returns 200 if healthy, 503 if corrupted.
func (h *DBHealthHandler) Check(c *gin.Context) {
    response := DBHealthResponse{
        Status:          "unknown",
        IntegrityCheck:  "pending",
        LastBackupTime:  nil,
        BackupAvailable: false,
    }

    // Run quick integrity check
    var integrityResult string
    if err := h.db.Raw("PRAGMA quick_check").Scan(&integrityResult).Error; err != nil {
        response.Status = "error"
        response.IntegrityCheck = err.Error()
        c.JSON(http.StatusInternalServerError, response)
        return
    }

    response.IntegrityCheck = integrityResult

    // Get last backup time
    if h.backupService != nil {
        lastBackup, err := h.backupService.GetLastBackupTime()
        if err == nil && lastBackup != nil {
            formatted := lastBackup.Format(time.RFC3339)
            response.LastBackupTime = &formatted
            response.BackupAvailable = true
        }
    }

    if integrityResult == "ok" {
        response.Status = "healthy"
        c.JSON(http.StatusOK, response)
    } else {
        response.Status = "corrupted"
        logger.Log().WithField("integrity_check", integrityResult).Warn("DB health check detected corruption")
        c.JSON(http.StatusServiceUnavailable, response)
    }
}
```

### Route Registration in `routes.go`

```go
// Add after backupService initialization (around line 110):
dbHealthHandler := handlers.NewDBHealthHandler(db, backupService)

// Add before api := router.Group("/api/v1") (around line 88):
// Public DB health endpoint (no auth required for monitoring tools)
router.GET("/api/v1/health/db", dbHealthHandler.Check)
```

### Test Requirements

Create `backend/internal/api/handlers/db_health_handler_test.go`:

```go
func TestDBHealthHandler_HealthyDatabase(t *testing.T)
func TestDBHealthHandler_CorruptedDatabase(t *testing.T)
func TestDBHealthHandler_IncludesBackupTime(t *testing.T)
func TestDBHealthHandler_NoBackupsAvailable(t *testing.T)
```

---

## 5. Integration Points Summary

### File Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `backend/internal/database/database.go` | Modify | Add startup integrity check |
| `backend/internal/database/errors.go` | New | Corruption sentinel logging |
| `backend/internal/database/errors_test.go` | New | Tests for error handling |
| `backend/internal/services/backup_service.go` | Modify | Add retention cleanup, last backup time |
| `backend/internal/services/backup_service_test.go` | Modify | Add tests for new methods |
| `backend/internal/api/handlers/db_health_handler.go` | New | DB health check handler |
| `backend/internal/api/handlers/db_health_handler_test.go` | New | Tests for DB health endpoint |
| `backend/internal/api/routes/routes.go` | Modify | Register /api/v1/health/db route |

### Service Dependencies

```
routes.go
├── database.Connect() ──→ Startup integrity check
│   └── database.SetIntegrityCheckDB(db)
├── services.NewBackupService()
│   ├── CreateBackup()
│   ├── cleanupOldBackups() [new]
│   └── GetLastBackupTime() [new]
└── handlers.NewDBHealthHandler(db, backupService)
    └── Check() ──→ GET /api/v1/health/db
```

### Patterns to Follow

1. **Logging:** Use `logger.Log().WithFields()` for structured logs (see `logger.go`)
2. **Error wrapping:** Use `fmt.Errorf("context: %w", err)` (see copilot-instructions.md)
3. **Handler pattern:** Follow existing handler struct pattern (see `backup_handler.go`)
4. **Test pattern:** Table-driven tests with `httptest` (see `health_handler_test.go`)

---

## 6. Implementation Order

1. **Phase 1: Detection (Low Risk)**
   - [ ] `database/errors.go` - Corruption sentinel
   - [ ] `database/database.go` - Startup check
   - [ ] Unit tests for above

2. **Phase 2: Visibility (Low Risk)**
   - [ ] `handlers/db_health_handler.go` - DB health endpoint
   - [ ] `routes/routes.go` - Route registration
   - [ ] Unit tests for handler

3. **Phase 3: Prevention (Medium Risk)**
   - [ ] `services/backup_service.go` - Retention cleanup
   - [ ] Integration tests

---

## 7. API Response Formats

### `GET /api/v1/health/db`

**Healthy Response (200):**

```json
{
  "status": "healthy",
  "integrity_check": "ok",
  "last_backup_time": "2024-12-17T03:00:00Z",
  "backup_available": true
}
```

**Corrupted Response (503):**

```json
{
  "status": "corrupted",
  "integrity_check": "*** in database main ***\nPage 123: btree page count differs",
  "last_backup_time": "2024-12-17T03:00:00Z",
  "backup_available": true
}
```

**No Backups Response (200):**

```json
{
  "status": "healthy",
  "integrity_check": "ok",
  "last_backup_time": null,
  "backup_available": false
}
```

---

## 8. Monitoring & Alerting

The structured logs enable external monitoring tools to detect:

```json
{
  "level": "error",
  "event_type": "database_corruption",
  "severity": "critical",
  "table": "uptime_heartbeats",
  "operation": "SELECT",
  "monitor_id": "abc-123",
  "msg": "🔴 DATABASE CORRUPTION ERROR - Run scripts/db-recovery.sh"
}
```

Recommended alerts:

- **Critical:** Any log with `event_type: database_corruption`
- **Warning:** `integrity_check` != "ok" at startup
- **Info:** Backup creation success/failure

---

## 9. Related Documentation

- [docs/database-maintenance.md](../database-maintenance.md) - Manual recovery procedures
- [scripts/db-recovery.sh](../../scripts/db-recovery.sh) - Recovery script
- [docs/features.md](../features.md#database-health-monitoring) - User-facing docs (to update)

---

## Summary

This plan adds four layers of database corruption protection:

| Layer | Feature | Location | Risk |
|-------|---------|----------|------|
| 1 | Early Warning | Startup integrity check | Low |
| 2 | Real-time Detection | Corruption sentinel logs | Low |
| 3 | Recovery Readiness | Auto-backup with retention | Medium |
| 4 | Visibility | Health endpoint `/api/v1/health/db` | Low |

All changes follow existing codebase patterns and avoid blocking critical operations.
