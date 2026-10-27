package models_test

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

// captureModuleLogs routes the global logger into a buffer for the test and returns the decoded JSON entries
// written while fn runs. The logger is not parallel-safe, so callers must not call t.Parallel.
func captureModuleLogs(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	fn()

	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		entries = append(entries, entry)
	}
	return entries
}

func TestSecurityModuleLogs_ActorWithControlCharactersIsSanitized(t *testing.T) {
	db := newModuleTestDB(t)
	forged := "198.51.100.7\n{\"level\":\"error\",\"msg\":\"forged\"}\r\x1b"
	want := "198.51.100.7 {\"level\":\"error\",\"msg\":\"forged\"} "

	entries := captureModuleLogs(t, func() {
		require.NoError(t, models.SetModuleState(db, models.SecurityWAFEnabledSettingKey, true, forged))
		require.NoError(t, models.ClearModuleKey(db, models.SecurityWAFEnabledSettingKey, forged))
	})

	require.Len(t, entries, 2, "one write line and one clear line")
	for _, entry := range entries {
		assert.Equal(t, want, entry["actor"])
		assert.Equal(t, "info", entry["level"])
	}
}

func TestSecurityModuleLogs_NormalActorIsUnchangedAndLongActorIsBounded(t *testing.T) {
	db := newModuleTestDB(t)
	long := strings.Repeat("a", 300)

	entries := captureModuleLogs(t, func() {
		require.NoError(t, models.SetModuleState(db, models.SecurityACLEnabledSettingKey, false, "user:42"))
		require.NoError(t, models.SetModuleState(db, models.SecurityACLEnabledSettingKey, false, long))
	})

	require.Len(t, entries, 2)
	assert.Equal(t, "user:42", entries[0]["actor"])
	assert.Equal(t, strings.Repeat("a", 128), entries[1]["actor"])
	assert.Equal(t, models.SecurityACLEnabledSettingKey, entries[0]["setting_key"])
	assert.Equal(t, "false", entries[0]["value"])
}

func TestSecurityModuleLogs_WriteMessagesDescribeTheAction(t *testing.T) {
	db := newModuleTestDB(t)

	entries := captureModuleLogs(t, func() {
		require.NoError(t, models.SetModuleState(db, models.SecurityACLEnabledSettingKey, true, "user:1"))
		require.NoError(t, models.ClearModuleKey(db, models.SecurityACLEnabledSettingKey, "user:1"))
	})

	require.Len(t, entries, 2)
	assert.Equal(t, "security module switch written", entries[0]["msg"])
	assert.Equal(t, "security module switch key removed", entries[1]["msg"])
}

func TestReconcileWAFDetectionMode_LogMessageDescribesTheChange(t *testing.T) {
	db := newModuleTestDB(t)
	setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "true")
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: ""}).Error)

	entries := captureModuleLogs(t, func() {
		changed, err := models.ReconcileWAFDetectionMode(db)
		require.NoError(t, err)
		require.Equal(t, int64(1), changed)
	})

	require.Len(t, entries, 1)
	assert.Equal(t, "WAF switch is on with no WAF mode set: default row set to monitor", entries[0]["msg"])
}
