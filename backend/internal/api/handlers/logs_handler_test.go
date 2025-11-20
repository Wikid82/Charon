package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
)

func setupLogsTest(t *testing.T) (*gin.Engine, *services.LogService, string) {
	t.Helper()

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "cpm-logs-test")
	require.NoError(t, err)

	// LogService expects LogDir to be .../data/logs
	// It derives it from cfg.DatabasePath

	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	dbPath := filepath.Join(dataDir, "cpm.db")

	// Create logs dir
	logsDir := filepath.Join(dataDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	// Create dummy log files
	err = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte("line1\nline2\nline3"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(logsDir, "cpmp.log"), []byte("app log line 1\napp log line 2"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		DatabasePath: dbPath,
	}

	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")

	logs := api.Group("/logs")
	logs.GET("", h.List)
	logs.GET("/:filename", h.Read)

	return r, svc, tmpDir
}

func TestLogsLifecycle(t *testing.T) {
	router, _, tmpDir := setupLogsTest(t)
	defer os.RemoveAll(tmpDir)

	// 1. List logs
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var logs []services.LogFile
	err := json.Unmarshal(resp.Body.Bytes(), &logs)
	require.NoError(t, err)
	require.Len(t, logs, 2) // access.log and cpmp.log

	// Verify content of one log file
	found := false
	for _, l := range logs {
		if l.Name == "access.log" {
			found = true
			require.Greater(t, l.Size, int64(0))
		}
	}
	require.True(t, found)

	// 2. Read log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?lines=2", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var content struct {
		Filename string   `json:"filename"`
		Lines    []string `json:"lines"`
	}
	err = json.Unmarshal(resp.Body.Bytes(), &content)
	require.NoError(t, err)
	require.Len(t, content.Lines, 2)
	require.Equal(t, "line3", content.Lines[1]) // Last line
}
