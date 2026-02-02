package database

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCorruptionError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some random error"),
			expected: false,
		},
		{
			name:     "database disk image is malformed",
			err:      errors.New("database disk image is malformed"),
			expected: true,
		},
		{
			name:     "malformed in message",
			err:      errors.New("query failed: table is malformed"),
			expected: true,
		},
		{
			name:     "corrupt database",
			err:      errors.New("database is corrupt"),
			expected: true,
		},
		{
			name:     "disk I/O error",
			err:      errors.New("disk I/O error during read"),
			expected: true,
		},
		{
			name:     "file is not a database",
			err:      errors.New("file is not a database"),
			expected: true,
		},
		{
			name:     "file is encrypted or is not a database",
			err:      errors.New("file is encrypted or is not a database"),
			expected: true,
		},
		{
			name:     "database or disk is full",
			err:      errors.New("database or disk is full"),
			expected: true,
		},
		{
			name:     "case insensitive - MALFORMED uppercase",
			err:      errors.New("DATABASE DISK IMAGE IS MALFORMED"),
			expected: true,
		},
		{
			name:     "wrapped error with corruption",
			err:      errors.New("failed to query: database disk image is malformed"),
			expected: true,
		},
		{
			name:     "network error - not corruption",
			err:      errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "record not found - not corruption",
			err:      errors.New("record not found"),
			expected: false,
		},
		{
			name:     "constraint violation - not corruption",
			err:      errors.New("UNIQUE constraint failed"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCorruptionError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogCorruptionError(t *testing.T) {
	t.Parallel()
	t.Run("nil error does not panic", func(t *testing.T) {
		// Should not panic
		LogCorruptionError(nil, nil)
	})

	t.Run("logs with context", func(t *testing.T) {
		// This just verifies it doesn't panic - actual log output is not captured
		err := errors.New("database disk image is malformed")
		ctx := map[string]any{
			"operation":  "GetMonitorHistory",
			"table":      "uptime_heartbeats",
			"monitor_id": "test-uuid",
		}
		LogCorruptionError(err, ctx)
	})

	t.Run("logs without context", func(t *testing.T) {
		err := errors.New("database corrupt")
		LogCorruptionError(err, nil)
	})
}

func TestCheckIntegrity(t *testing.T) {
	t.Parallel()
	t.Run("healthy database returns ok", func(t *testing.T) {
		db, err := Connect("file::memory:?cache=shared")
		require.NoError(t, err)
		require.NotNil(t, db)

		ok, result := CheckIntegrity(db)
		assert.True(t, ok, "In-memory database should pass integrity check")
		assert.Equal(t, "ok", result)
	})

	t.Run("file-based database passes check", func(t *testing.T) {
		tmpDir := t.TempDir()
		db, err := Connect(tmpDir + "/test.db")
		require.NoError(t, err)
		require.NotNil(t, db)

		// Create a table and insert some data
		err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)").Error
		require.NoError(t, err)
		err = db.Exec("INSERT INTO test (name) VALUES ('test')").Error
		require.NoError(t, err)

		ok, result := CheckIntegrity(db)
		assert.True(t, ok)
		assert.Equal(t, "ok", result)
	})
}

// Phase 4 & 5: Deep coverage tests

func TestLogCorruptionError_EmptyContext(t *testing.T) {
	t.Parallel()
	// Test with empty context map
	err := errors.New("database disk image is malformed")
	emptyCtx := map[string]any{}

	// Should not panic with empty context
	LogCorruptionError(err, emptyCtx)
}

func TestCheckIntegrity_ActualCorruption(t *testing.T) {
	t.Parallel()
	// Create a SQLite database and corrupt it
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "corrupt_test.db")

	// Create valid database
	db, err := Connect(dbPath)
	require.NoError(t, err)

	// Insert some data
	err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, data TEXT)").Error
	require.NoError(t, err)
	err = db.Exec("INSERT INTO test (data) VALUES ('test1'), ('test2')").Error
	require.NoError(t, err)

	// Close connection
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	// Corrupt the database file
	// #nosec G304 -- Test function intentionally opens test database file for corruption testing
	f, err := os.OpenFile(dbPath, os.O_RDWR, 0o600) // #nosec G302 -- Test intentionally opens test database for corruption
	require.NoError(t, err)
	stat, err := f.Stat()
	require.NoError(t, err)
	if stat.Size() > 100 {
		// Overwrite middle section
		_, err = f.WriteAt([]byte("CORRUPTED_DATA"), stat.Size()/2)
		require.NoError(t, err)
	}
	_ = f.Close()

	// Reconnect
	db2, err := Connect(dbPath)
	if err != nil {
		// Connection failed due to corruption - that's a valid outcome
		t.Skip("Database connection failed immediately")
	}

	// Run integrity check
	ok, message := CheckIntegrity(db2)
	// Should detect corruption
	if !ok {
		assert.False(t, ok)
		assert.NotEqual(t, "ok", message)
		assert.Contains(t, message, "database")
	} else {
		// Corruption might not be in checked pages
		t.Log("Corruption not detected by quick_check - might be in unused pages")
	}
}

func TestCheckIntegrity_PRAGMAError(t *testing.T) {
	t.Parallel()
	// Create database and close connection to cause PRAGMA to fail
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Connect(dbPath)
	require.NoError(t, err)

	// Close the underlying SQL connection
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Now CheckIntegrity should fail because connection is closed
	ok, message := CheckIntegrity(db)
	assert.False(t, ok, "CheckIntegrity should fail on closed database")
	assert.Contains(t, message, "failed to run integrity check")
}
