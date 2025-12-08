package handlers

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
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
	// Use crypto/rand for suffix generation in tests to avoid static analysis warnings
	n, _ := crand.Int(crand.Reader, big.NewInt(10000))
	uniqueSuffix := fmt.Sprintf("%d%d", time.Now().UnixNano(), n.Int64())
	dsn := fmt.Sprintf("file:%s_%s?mode=memory&cache=shared&_journal_mode=WAL&_busy_timeout=5000", dsnName, uniqueSuffix)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}
