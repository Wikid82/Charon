// Package database handles database connections, migrations, and error detection.
package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// SQLite corruption error indicators
var corruptionPatterns = []string{
	"malformed",
	"corrupt",
	"disk I/O error",
	"database disk image is malformed",
	"file is not a database",
	"file is encrypted or is not a database",
	"database or disk is full",
}

// IsCorruptionError checks if the given error indicates SQLite database corruption.
// It detects errors like "database disk image is malformed", "corrupt", and related I/O errors.
func IsCorruptionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	for _, pattern := range corruptionPatterns {
		if strings.Contains(errStr, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// LogCorruptionError logs a database corruption error with structured context.
// The context map can include fields like "operation", "table", "query", "monitor_id", etc.
func LogCorruptionError(err error, context map[string]any) {
	if err == nil {
		return
	}

	entry := logger.Log().WithError(err)

	// Add all context fields (range over nil map is safe)
	for key, value := range context {
		entry = entry.WithField(key, value)
	}

	// Mark as corruption error for alerting/monitoring
	entry = entry.WithField("error_type", "database_corruption")

	entry.Error("SQLite database corruption detected")
}

// CheckIntegrity runs PRAGMA quick_check and returns whether the database is healthy.
// Returns (healthy, message): healthy is true if database passes integrity check,
// message contains "ok" on success or the error/corruption message on failure.
func CheckIntegrity(db *gorm.DB) (healthy bool, message string) {
	var result string
	if err := db.Raw("PRAGMA quick_check").Scan(&result).Error; err != nil {
		return false, "failed to run integrity check: " + err.Error()
	}

	// SQLite returns "ok" if the database passes integrity check
	if strings.EqualFold(result, "ok") {
		return true, "ok"
	}

	return false, result
}

// CheckIntegrityDedicated runs PRAGMA quick_check on its own dedicated
// connection to dbPath (opened directly via sqlite.DriverName), never the
// caller's shared *gorm.DB pool (spec §2.5d/§3.9 — Async Backup/Restore
// Jobs). This mirrors runQuickCheck's rationale (database.go:109-113): the
// main application connection is capped at SetMaxOpenConns(1), so running a
// scan on it — which this package's own comments already note "can take
// well over a minute on larger databases" — would serialize every other
// request behind the scan for its full duration. DBHealthHandler.Check
// previously called CheckIntegrity(h.db) directly against that shared pool;
// this is the fix.
func CheckIntegrityDedicated(dbPath string) (healthy bool, message string) {
	checkDB, err := sql.Open(sqlite.DriverName, dbPath)
	if err != nil {
		return false, fmt.Sprintf("failed to open database for integrity check: %v", err)
	}
	defer func() {
		_ = checkDB.Close()
	}()

	var result string
	if err := checkDB.QueryRow("PRAGMA quick_check").Scan(&result); err != nil {
		return false, "failed to run integrity check: " + err.Error()
	}

	if strings.EqualFold(strings.TrimSpace(result), "ok") {
		return true, "ok"
	}

	return false, result
}
