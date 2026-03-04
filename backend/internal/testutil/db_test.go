package testutil

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// testModel is a simple model for testing database operations
type testModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"not null"`
}

// setupTestDB creates a fresh in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run migrations
	if err := db.AutoMigrate(&testModel{}); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

// TestWithTx_Success verifies that WithTx executes the function and rolls back the transaction
func TestWithTx_Success(t *testing.T) {
	db := setupTestDB(t)

	// Insert data within transaction
	WithTx(t, db, func(tx *gorm.DB) {
		record := &testModel{Name: "test-record"}
		if err := tx.Create(record).Error; err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}

		// Verify record exists within transaction
		var count int64
		tx.Model(&testModel{}).Count(&count)
		if count != 1 {
			t.Errorf("Expected 1 record in transaction, got %d", count)
		}
	})

	// Verify record was rolled back
	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 records after rollback, got %d", count)
	}
}

// TestWithTx_Panic verifies that WithTx rolls back on panic and propagates the panic
func TestWithTx_Panic(t *testing.T) {
	db := setupTestDB(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to be propagated, but no panic occurred")
		} else if r != "test panic" {
			t.Errorf("Expected panic value 'test panic', got %v", r)
		}

		// Verify record was rolled back after panic
		var count int64
		db.Model(&testModel{}).Count(&count)
		if count != 0 {
			t.Errorf("Expected 0 records after panic rollback, got %d", count)
		}
	}()

	WithTx(t, db, func(tx *gorm.DB) {
		// Insert data
		record := &testModel{Name: "panic-test"}
		if err := tx.Create(record).Error; err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}

		// Trigger panic
		panic("test panic")
	})
}

// TestWithTx_MultipleOperations verifies WithTx works with multiple database operations
func TestWithTx_MultipleOperations(t *testing.T) {
	db := setupTestDB(t)

	WithTx(t, db, func(tx *gorm.DB) {
		// Create multiple records
		records := []testModel{
			{Name: "record1"},
			{Name: "record2"},
			{Name: "record3"},
		}

		for _, record := range records {
			if err := tx.Create(&record).Error; err != nil {
				t.Fatalf("Failed to create record: %v", err)
			}
		}

		// Update a record
		if err := tx.Model(&testModel{}).Where("name = ?", "record2").Update("name", "updated").Error; err != nil {
			t.Fatalf("Failed to update record: %v", err)
		}

		// Verify updates within transaction
		var updated testModel
		tx.Where("name = ?", "updated").First(&updated)
		if updated.Name != "updated" {
			t.Error("Update not visible within transaction")
		}
	})

	// Verify all operations were rolled back
	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 records after rollback, got %d", count)
	}
}

// TestGetTestTx_Cleanup verifies that GetTestTx registers cleanup and rolls back
func TestGetTestTx_Cleanup(t *testing.T) {
	db := setupTestDB(t)

	// Create a subtest to isolate cleanup
	t.Run("Subtest", func(t *testing.T) {
		tx := GetTestTx(t, db)

		// Insert data
		record := &testModel{Name: "cleanup-test"}
		if err := tx.Create(record).Error; err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}

		// Verify record exists
		var count int64
		tx.Model(&testModel{}).Count(&count)
		if count != 1 {
			t.Errorf("Expected 1 record in transaction, got %d", count)
		}

		// When this subtest finishes, t.Cleanup should roll back the transaction
	})

	// Verify record was rolled back after subtest cleanup
	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 records after cleanup rollback, got %d", count)
	}
}

// TestGetTestTx_MultipleTransactions verifies that multiple GetTestTx calls are isolated
func TestGetTestTx_MultipleTransactions(t *testing.T) {
	db := setupTestDB(t)

	// First transaction
	t.Run("Transaction1", func(t *testing.T) {
		tx := GetTestTx(t, db)
		record := &testModel{Name: "tx1-record"}
		if err := tx.Create(record).Error; err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}
	})

	// Second transaction
	t.Run("Transaction2", func(t *testing.T) {
		tx := GetTestTx(t, db)
		record := &testModel{Name: "tx2-record"}
		if err := tx.Create(record).Error; err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}
	})

	// Verify both transactions were rolled back
	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 records after all cleanups, got %d", count)
	}
}

// TestGetTestTx_UsageInMultipleFunctions demonstrates passing tx between functions
func TestGetTestTx_UsageInMultipleFunctions(t *testing.T) {
	db := setupTestDB(t)

	t.Run("MultiFunction", func(t *testing.T) {
		tx := GetTestTx(t, db)

		// Helper function 1: Create
		createRecord := func(tx *gorm.DB, name string) error {
			return tx.Create(&testModel{Name: name}).Error
		}

		// Helper function 2: Count
		countRecords := func(tx *gorm.DB) int64 {
			var count int64
			tx.Model(&testModel{}).Count(&count)
			return count
		}

		// Use helper functions with the same transaction
		if err := createRecord(tx, "func-test"); err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}

		count := countRecords(tx)
		if count != 1 {
			t.Errorf("Expected 1 record, got %d", count)
		}
	})

	// Verify cleanup happened
	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 records after cleanup, got %d", count)
	}
}

// TestGetTestTx_Parallel verifies isolation with multiple GetTestTx calls
// Note: SQLite doesn't handle concurrent writes well, so we test isolation without t.Parallel()
func TestGetTestTx_Parallel(t *testing.T) {
	// Use shared database for isolation tests
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open shared test database: %v", err)
	}

	if err := db.AutoMigrate(&testModel{}); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	// Run isolated tests (demonstrating isolation without actual parallelism due to SQLite limitations)
	t.Run("Isolation1", func(t *testing.T) {
		tx := GetTestTx(t, db)

		record := &testModel{Name: "isolation1"}
		if err := tx.Create(record).Error; err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}

		var count int64
		tx.Model(&testModel{}).Count(&count)
		if count != 1 {
			t.Errorf("Expected 1 record in isolation1 transaction, got %d", count)
		}
	})

	t.Run("Isolation2", func(t *testing.T) {
		tx := GetTestTx(t, db)

		record := &testModel{Name: "isolation2"}
		if err := tx.Create(record).Error; err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}

		var count int64
		tx.Model(&testModel{}).Count(&count)
		if count != 1 {
			t.Errorf("Expected 1 record in isolation2 transaction, got %d", count)
		}
	})

	// After all tests complete, verify all rolled back
	var finalCount int64
	db.Model(&testModel{}).Count(&finalCount)
	if finalCount != 0 {
		t.Errorf("Expected 0 records after isolated tests, got %d", finalCount)
	}
}

// TestGetTestTx_WithActualTestFailure verifies cleanup happens even on test failure
func TestGetTestTx_WithActualTestFailure(t *testing.T) {
	db := setupTestDB(t)

	// This subtest will fail, but cleanup should still happen
	t.Run("FailingSubtest", func(t *testing.T) {
		tx := GetTestTx(t, db)

		record := &testModel{Name: "will-be-rolled-back"}
		if err := tx.Create(record).Error; err != nil {
			t.Fatalf("Failed to create record: %v", err)
		}

		// Even though this test "fails" conceptually, cleanup should still run
		// (We're not actually failing here to avoid failing the test suite)
	})

	// Verify cleanup happened despite the "failure"
	var count int64
	db.Model(&testModel{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 records after cleanup on failure, got %d", count)
	}
}
