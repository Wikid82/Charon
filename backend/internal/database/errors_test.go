package database

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCorruptionError(t *testing.T) {
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
	t.Run("nil error does not panic", func(t *testing.T) {
		// Should not panic
		LogCorruptionError(nil, nil)
	})

	t.Run("logs with context", func(t *testing.T) {
		// This just verifies it doesn't panic - actual log output is not captured
		err := errors.New("database disk image is malformed")
		ctx := map[string]interface{}{
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
