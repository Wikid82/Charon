package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
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

	// Create sample JSON logs
	logEntry1 := models.CaddyAccessLog{
		Level:  "info",
		Ts:     1600000000,
		Msg:    "request handled",
		Status: 200,
	}
	logEntry1.Request.Method = "GET"
	logEntry1.Request.Host = "example.com"
	logEntry1.Request.URI = "/"
	logEntry1.Request.RemoteIP = "1.2.3.4"

	logEntry2 := models.CaddyAccessLog{
		Level:  "error",
		Ts:     1600000060,
		Msg:    "error handled",
		Status: 500,
	}
	logEntry2.Request.Method = "POST"
	logEntry2.Request.Host = "api.example.com"
	logEntry2.Request.URI = "/submit"
	logEntry2.Request.RemoteIP = "5.6.7.8"

	line1, _ := json.Marshal(logEntry1)
	line2, _ := json.Marshal(logEntry2)

	content := string(line1) + "\n" + string(line2) + "\n"

	err = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(logsDir, "other.txt"), []byte("ignore me"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{DatabasePath: filepath.Join(dataDir, "cpm.db")}
	service := NewLogService(cfg)

	// Test List
	logs, err := service.ListLogs()
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "access.log", logs[0].Name)

	// Test QueryLogs - All
	results, total, err := service.QueryLogs("access.log", models.LogFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, results, 2)
	// Should be reversed (newest first)
	assert.Equal(t, 500, results[0].Status)
	assert.Equal(t, 200, results[1].Status)

	// Test QueryLogs - Filter Status
	results, total, err = service.QueryLogs("access.log", models.LogFilter{Status: "5xx", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, results, 1)
	assert.Equal(t, 500, results[0].Status)

	// Test QueryLogs - Filter Host
	results, total, err = service.QueryLogs("access.log", models.LogFilter{Host: "api.example.com", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, results, 1)
	assert.Equal(t, "api.example.com", results[0].Request.Host)

	// Test QueryLogs - Search
	results, total, err = service.QueryLogs("access.log", models.LogFilter{Search: "submit", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, results, 1)
	assert.Equal(t, "/submit", results[0].Request.URI)

	// Test GetLogPath
	path, err := service.GetLogPath("access.log")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(logsDir, "access.log"), path)

	// Test GetLogPath non-existent
	_, err = service.GetLogPath("missing.log")
	assert.Error(t, err)

	// Test ListLogs - Directory Not Exist
	nonExistService := NewLogService(&config.Config{DatabasePath: filepath.Join(t.TempDir(), "missing", "cpm.db")})
	logs, err = nonExistService.ListLogs()
	require.NoError(t, err)
	assert.Empty(t, logs)

	// Test QueryLogs - Non-JSON Logs
	plainContent := "2023/10/27 10:00:00 Application started\nJust a plain line\n"
	err = os.WriteFile(filepath.Join(logsDir, "app.log"), []byte(plainContent), 0644)
	require.NoError(t, err)

	results, total, err = service.QueryLogs("app.log", models.LogFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	// Reverse order check
	assert.Equal(t, "Just a plain line", results[0].Msg)
	assert.Equal(t, "Application started", results[1].Msg)
	assert.Equal(t, "INFO", results[1].Level)

	// Test QueryLogs - Pagination
	// We have 2 logs in access.log
	results, total, err = service.QueryLogs("access.log", models.LogFilter{Limit: 1, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, 500, results[0].Status) // Newest first

	results, total, err = service.QueryLogs("access.log", models.LogFilter{Limit: 1, Offset: 1})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, 200, results[0].Status) // Second newest

	results, total, err = service.QueryLogs("access.log", models.LogFilter{Limit: 10, Offset: 5})
	require.NoError(t, err)
	assert.Empty(t, results)

	// Test QueryLogs - Exact Status Match
	results, total, err = service.QueryLogs("access.log", models.LogFilter{Status: "200", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 200, results[0].Status)

	// Test QueryLogs - Search Fields
	// Search Method
	results, total, err = service.QueryLogs("access.log", models.LogFilter{Search: "POST", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "POST", results[0].Request.Method)

	// Search RemoteIP
	results, total, err = service.QueryLogs("access.log", models.LogFilter{Search: "5.6.7.8", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "5.6.7.8", results[0].Request.RemoteIP)
}
