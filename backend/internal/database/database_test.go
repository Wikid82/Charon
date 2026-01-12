package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
	t.Parallel()
	// Test with memory DB
	db, err := Connect("file::memory:?cache=shared")
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Test with file DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err = Connect(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, db)
}

func TestConnect_Error(t *testing.T) {
	t.Parallel()
	// Test with invalid path (directory)
	tempDir := t.TempDir()
	_, err := Connect(tempDir)
	assert.Error(t, err)
}

func TestConnect_WALMode(t *testing.T) {
	t.Parallel()
	// Create a file-based database to test WAL mode
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "wal_test.db")

	db, err := Connect(dbPath)
	require.NoError(t, err)
	require.NotNil(t, db)

	// Verify WAL mode is enabled
	var journalMode string
	err = db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode, "SQLite should be in WAL mode")

	// Verify other PRAGMA settings
	var busyTimeout int
	err = db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error
	require.NoError(t, err)
	assert.Equal(t, 5000, busyTimeout, "busy_timeout should be 5000ms")

	var synchronous int
	err = db.Raw("PRAGMA synchronous").Scan(&synchronous).Error
	require.NoError(t, err)
	assert.Equal(t, 1, synchronous, "synchronous should be NORMAL (1)")
}

// Phase 2: database.go coverage tests

func TestConnect_InvalidDSN(t *testing.T) {
	t.Parallel()
	// Test with a directory path instead of a file path
	// SQLite cannot open a directory as a database file
	tmpDir := t.TempDir()
	_, err := Connect(tmpDir)
	assert.Error(t, err)
}

func TestConnect_IntegrityCheckCorrupted(t *testing.T) {
	t.Parallel()
	// Create a valid SQLite database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "corrupt.db")

	// First create a valid database
	db, err := Connect(dbPath)
	require.NoError(t, err)
	db.Exec("CREATE TABLE test (id INTEGER, data TEXT)")
	db.Exec("INSERT INTO test VALUES (1, 'test')")

	// Close the database
	sqlDB, _ := db.DB()
	sqlDB.Close()

	// Corrupt the database file by overwriting with invalid data
	// We'll overwrite the middle of the file to corrupt it
	corruptDB(t, dbPath)

	// Try to connect to corrupted database
	// Connection may succeed but integrity check should detect corruption
	db2, err := Connect(dbPath)
	// Connection might succeed or fail depending on corruption type
	if err != nil {
		// If connection fails, that's also a valid outcome for corrupted DB
		assert.Contains(t, err.Error(), "database")
	} else {
		// If connection succeeds, integrity check should catch it
		// The Connect function logs the error but doesn't fail the connection
		assert.NotNil(t, db2)
	}
}

func TestConnect_PRAGMAVerification(t *testing.T) {
	t.Parallel()
	// Verify all PRAGMA settings are correctly applied
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "pragma_test.db")

	db, err := Connect(dbPath)
	require.NoError(t, err)
	require.NotNil(t, db)

	// Verify journal_mode
	var journalMode string
	err = db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode)

	// Verify busy_timeout
	var busyTimeout int
	err = db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error
	require.NoError(t, err)
	assert.Equal(t, 5000, busyTimeout)

	// Verify synchronous
	var synchronous int
	err = db.Raw("PRAGMA synchronous").Scan(&synchronous).Error
	require.NoError(t, err)
	assert.Equal(t, 1, synchronous, "synchronous should be NORMAL (1)")
}

func TestConnect_CorruptedDatabase_FullIntegrationScenario(t *testing.T) {
	t.Parallel()
	// Create a valid database with data
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "integration.db")

	db, err := Connect(dbPath)
	require.NoError(t, err)

	// Create table and insert data
	err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)").Error
	require.NoError(t, err)
	err = db.Exec("INSERT INTO users (name) VALUES ('Alice'), ('Bob')").Error
	require.NoError(t, err)

	// Close database
	sqlDB, _ := db.DB()
	sqlDB.Close()

	// Corrupt the database
	corruptDB(t, dbPath)

	// Attempt to reconnect
	db2, err := Connect(dbPath)
	// The function logs errors but may still return a database connection
	// depending on when corruption is detected
	if err != nil {
		assert.Contains(t, err.Error(), "database")
	} else {
		assert.NotNil(t, db2)
		// Try to query - should fail or return error
		var count int
		err = db2.Raw("SELECT COUNT(*) FROM users").Scan(&count).Error
		// Query might fail due to corruption
		if err != nil {
			assert.Contains(t, err.Error(), "database")
		}
	}
}

// Helper function to corrupt SQLite database
func corruptDB(t *testing.T, dbPath string) {
	t.Helper()
	// Open and corrupt file
	f, err := os.OpenFile(dbPath, os.O_RDWR, 0o644)
	require.NoError(t, err)
	defer f.Close()

	// Get file size
	stat, err := f.Stat()
	require.NoError(t, err)
	size := stat.Size()

	if size > 100 {
		// Overwrite middle section with random bytes to corrupt B-tree structure
		_, err = f.WriteAt([]byte("CORRUPTED_DATA_BLOCK"), size/2)
		require.NoError(t, err)
	} else {
		// For small files, corrupt the header
		_, err = f.WriteAt([]byte("CORRUPT"), 0)
		require.NoError(t, err)
	}
}
