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

// OpenMemoryDB opens a unique in-memory SQLite database with a silent logger
// and a busy timeout. The connection pool is closed when the test finishes.
func OpenMemoryDB(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(MemoryDSN(t, "_busy_timeout=5000")), &gorm.Config{
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
	return db
}
