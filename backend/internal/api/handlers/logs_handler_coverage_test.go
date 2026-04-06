package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/services"
)

func TestLogsHandler_Read_FilterBySearch(t *testing.T) {

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	_ = os.MkdirAll(logsDir, 0o750) // #nosec G301 -- test directory

	// Write JSON log lines
	content := `{"level":"info","ts":1600000000,"msg":"request handled","request":{"method":"GET","host":"example.com","uri":"/api/search","remote_ip":"1.2.3.4"},"status":200}
{"level":"error","ts":1600000060,"msg":"error occurred","request":{"method":"POST","host":"example.com","uri":"/api/submit","remote_ip":"5.6.7.8"},"status":500}
`
	_ = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	// Test with search filter
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?search=error", http.NoBody)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestLogsHandler_Read_FilterByHost(t *testing.T) {

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	_ = os.MkdirAll(logsDir, 0o750) // #nosec G301 -- test directory

	content := `{"level":"info","ts":1600000000,"msg":"request handled","request":{"method":"GET","host":"example.com","uri":"/","remote_ip":"1.2.3.4"},"status":200}
{"level":"info","ts":1600000001,"msg":"request handled","request":{"method":"GET","host":"other.com","uri":"/","remote_ip":"1.2.3.4"},"status":200}
`
	_ = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?host=example.com", http.NoBody)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
}

func TestLogsHandler_Read_FilterByLevel(t *testing.T) {

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	_ = os.MkdirAll(logsDir, 0o750) // #nosec G301 -- test directory

	content := `{"level":"info","ts":1600000000,"msg":"info message"}
{"level":"error","ts":1600000001,"msg":"error message"}
`
	_ = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?level=error", http.NoBody)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
}

func TestLogsHandler_Read_FilterByStatus(t *testing.T) {

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	_ = os.MkdirAll(logsDir, 0o750) // #nosec G301 -- test directory

	content := `{"level":"info","ts":1600000000,"msg":"200 OK","request":{"host":"example.com"},"status":200}
{"level":"error","ts":1600000001,"msg":"500 Error","request":{"host":"example.com"},"status":500}
`
	_ = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?status=500", http.NoBody)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
}

func TestLogsHandler_Read_SortAsc(t *testing.T) {

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	_ = os.MkdirAll(logsDir, 0o750) // #nosec G301 -- test directory

	content := `{"level":"info","ts":1600000000,"msg":"first"}
{"level":"info","ts":1600000001,"msg":"second"}
`
	_ = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?sort=asc", http.NoBody)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
}

func TestLogsHandler_List_DirectoryIsFile(t *testing.T) {

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")

	// Create logs dir as a file to cause error
	_ = os.WriteFile(logsDir, []byte("not a dir"), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/logs", http.NoBody)

	h.List(c)

	// Service may handle this gracefully or error
	assert.Contains(t, []int{200, 500}, w.Code)
}

func TestLogsHandler_Download_TempFileError(t *testing.T) {

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	logsDir := filepath.Join(dataDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o750)) // #nosec G301 -- test directory

	dbPath := filepath.Join(dataDir, "charon.db")
	logPath := filepath.Join(logsDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte("log line"), 0o600)) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	originalCreateTemp := createTempFile
	createTempFile = func(dir, pattern string) (*os.File, error) {
		return nil, fmt.Errorf("boom")
	}
	t.Cleanup(func() {
		createTempFile = originalCreateTemp
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log", http.NoBody)

	h.Download(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
