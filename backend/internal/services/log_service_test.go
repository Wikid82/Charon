package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogService(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cpm-log-service-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	logsDir := filepath.Join(dataDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	// Create logs
	err = os.WriteFile(filepath.Join(logsDir, "test.log"), []byte("line1\nline2\nline3"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(logsDir, "other.txt"), []byte("ignore me"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{DatabasePath: filepath.Join(dataDir, "cpm.db")}
	service := NewLogService(cfg)

	// Test List
	logs, err := service.ListLogs()
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "test.log", logs[0].Name)

	// Test Read
	lines, err := service.ReadLog("test.log", 2)
	require.NoError(t, err)
	assert.Len(t, lines, 2)
	assert.Equal(t, "line2", lines[0])
	assert.Equal(t, "line3", lines[1])

	// Test Read non-existent
	_, err = service.ReadLog("missing.log", 10)
	assert.Error(t, err)
}
