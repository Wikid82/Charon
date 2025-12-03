package handlers

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB creates a SQLite in-memory DB unique per test and applies
// a busy timeout and WAL journal mode to reduce SQLITE locking during parallel tests.
func OpenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Append a timestamp/random suffix to ensure uniqueness even across parallel runs
	dsnName := strings.ReplaceAll(t.Name(), "/", "_")
	rand.Seed(time.Now().UnixNano())
	uniqueSuffix := fmt.Sprintf("%d%d", time.Now().UnixNano(), rand.Intn(10000))
	dsn := fmt.Sprintf("file:%s_%s?mode=memory&cache=shared&_journal_mode=WAL&_busy_timeout=5000", dsnName, uniqueSuffix)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}
