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

	// Create dummy log files with JSON content
	log1 := `{"level":"info","ts":1600000000,"msg":"request handled","request":{"method":"GET","host":"example.com","uri":"/","remote_ip":"1.2.3.4"},"status":200}`
	log2 := `{"level":"error","ts":1600000060,"msg":"error handled","request":{"method":"POST","host":"api.example.com","uri":"/submit","remote_ip":"5.6.7.8"},"status":500}`

	err = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(log1+"\n"+log2+"\n"), 0644)
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
	logs.GET("/:filename/download", h.Download)

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
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?limit=2", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var content struct {
		Filename string        `json:"filename"`
		Logs     []interface{} `json:"logs"`
		Total    int           `json:"total"`
	}
	err = json.Unmarshal(resp.Body.Bytes(), &content)
	require.NoError(t, err)
	require.Len(t, content.Logs, 2)

	// 3. Download log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log/download", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "request handled")

	// 4. Read non-existent log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/missing.log", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// 5. Download non-existent log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/missing.log/download", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// 6. List logs error (delete directory)
	os.RemoveAll(filepath.Join(tmpDir, "data", "logs"))
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	// ListLogs returns empty list if dir doesn't exist, so it should be 200 OK with empty list
	require.Equal(t, http.StatusOK, resp.Code)
	var emptyLogs []services.LogFile
	err = json.Unmarshal(resp.Body.Bytes(), &emptyLogs)
	require.NoError(t, err)
	require.Empty(t, emptyLogs)
}
