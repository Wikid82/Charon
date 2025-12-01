package handlers

import (
    "fmt"
    "strings"
    "testing"

    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

// openTestDB creates a SQLite in-memory DB unique per test and applies
// a busy timeout and WAL journal mode to reduce SQLITE locking during parallel tests.
func OpenTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    dsnName := strings.ReplaceAll(t.Name(), "/", "_")
    dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_journal_mode=WAL&_busy_timeout=5000", dsnName)
    db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
    if err != nil {
        t.Fatalf("failed to open test db: %v", err)
    }
    return db
}
