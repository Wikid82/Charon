package testutil

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	dsnCounter   atomic.Uint64
	dsnUnsafeRun = regexp.MustCompile(`[^A-Za-z0-9_]+`)
)

// MemoryDSN returns a shared-cache in-memory SQLite DSN whose name has never
// been returned before in this process. The name combines the sanitized test
// name with a process-wide counter, so it stays unique across -count, shuffling,
// parallel tests and subtests. Optional params (for example
// "_busy_timeout=5000") are appended as query parameters.
func MemoryDSN(t testing.TB, params ...string) string {
	t.Helper()
	name := dsnUnsafeRun.ReplaceAllString(strings.ReplaceAll(t.Name(), "/", "_"), "_")
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", name, dsnCounter.Add(1))
	for _, p := range params {
		dsn += "&" + p
	}
	return dsn
}

// OpenMemoryDB opens a unique in-memory SQLite database with a silent logger, a busy timeout and reference enforcement
// on every connection, as production runs. The connection pool is closed when the test finishes.
func OpenMemoryDB(t testing.TB) *gorm.DB {
	t.Helper()
	return openMemoryDBWithForeignKeys(t)
}

// OpenMemoryDBWithForeignKeys is OpenMemoryDB; it is kept for call sites that state the enforcement explicitly.
func OpenMemoryDBWithForeignKeys(t testing.TB) *gorm.DB {
	t.Helper()
	return openMemoryDBWithForeignKeys(t)
}

// openMemoryDBWithForeignKeys opens the database with reference enforcement on for every connection. The driver
// applies the parameter silently when it does not know it, so the pragma is read back and the open fails unless it
// reads 1.
func openMemoryDBWithForeignKeys(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(MemoryDSN(t, "_busy_timeout=5000", "_foreign_keys=1")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	var enforced int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&enforced).Error; err != nil {
		t.Fatalf("read PRAGMA foreign_keys: %v", err)
	}
	if enforced != 1 {
		t.Fatalf("PRAGMA foreign_keys = %d, want 1: the driver did not enable reference enforcement", enforced)
	}
	return db
}
