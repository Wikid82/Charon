package testutil

import (
	"testing"

	"gorm.io/gorm"
)

// WithTx runs a test function within a transaction that is always rolled back.
// This provides test isolation without the overhead of creating new databases.
//
// Usage Example:
//
//	func TestSomething(t *testing.T) {
//	    sharedDB := setupSharedDB(t) // Create once per package
//	    testutil.WithTx(t, sharedDB, func(tx *gorm.DB) {
//	        // Use tx for all DB operations in this test
//	        tx.Create(&models.User{Name: "test"})
//	        // Transaction automatically rolled back at end
//	    })
//	}
func WithTx(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB)) {
	t.Helper()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
		tx.Rollback()
	}()
	fn(tx)
}

// GetTestTx returns a transaction that will be rolled back when the test completes.
// This is useful for tests that need to pass the transaction to multiple functions.
//
// Usage Example:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel() // Safe to run in parallel with transaction isolation
//	    sharedDB := getSharedDB(t)
//	    tx := testutil.GetTestTx(t, sharedDB)
//	    // Use tx for all DB operations
//	    tx.Create(&models.User{Name: "test"})
//	    // Transaction automatically rolled back via t.Cleanup()
//	}
//
// Note: When using GetTestTx with t.Parallel(), ensure the shared DB is safe for
// concurrent access (e.g., using ?cache=shared for SQLite).
func GetTestTx(t *testing.T, db *gorm.DB) *gorm.DB {
	t.Helper()
	tx := db.Begin()
	t.Cleanup(func() {
		tx.Rollback()
	})
	return tx
}

// Best Practices for Transaction-Based Testing:
//
// 1. Create a shared DB once per test package (not per test):
//    var sharedDB *gorm.DB
//    func init() {
//        db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
//        db.AutoMigrate(&models.User{}, &models.Setting{})
//        sharedDB = db
//    }
//
// 2. Use transactions for test isolation:
//    func TestUser(t *testing.T) {
//        t.Parallel()
//        tx := testutil.GetTestTx(t, sharedDB)
//        // Test operations using tx
//    }
//
// 3. When NOT to use transaction rollbacks:
//    - Tests that need specific DB schemas per test
//    - Tests that intentionally test transaction behavior
//    - Tests that require nil DB values
//    - Tests using in-memory :memory: (already fast enough)
//    - Complex tests with custom setup/teardown logic
//
// 4. Benefits of transaction rollbacks:
//    - Faster than creating new databases (especially for disk-based DBs)
//    - Automatic cleanup (no manual teardown needed)
//    - Enables safe use of t.Parallel() for concurrent test execution
//    - Reduces disk I/O and memory usage in CI environments
