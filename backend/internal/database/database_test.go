package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
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
	// Test with invalid path (directory)
	tempDir := t.TempDir()
	_, err := Connect(tempDir)
	assert.Error(t, err)
}

func TestConnect_WALMode(t *testing.T) {
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
