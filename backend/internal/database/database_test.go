package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
