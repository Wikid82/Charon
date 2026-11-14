package handlers

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestRestoreModuleState_FailureReachesSharedLogger describes a failed restore after a refused apply: the error line
// goes through the shared logger, so it lands in the same output (and file log) as the rest of the application.
func TestRestoreModuleState_FailureReachesSharedLogger(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	restoreModuleState(db, nil, models.ModuleStateSnapshot{})

	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		entries = append(entries, entry)
	}
	require.NotEmpty(t, entries, "restore failure should be logged through the shared logger")
	assert.Equal(t, "error", entries[0]["level"])
	assert.Contains(t, entries[0]["msg"], "Failed to restore")
	assert.NotEmpty(t, entries[0]["error"])
}
