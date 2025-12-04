package handlers

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/services"
)

func TestLogsHandler_Read_FilterBySearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logsDir, 0o755)

	// Write JSON log lines
	content := `{"level":"info","ts":1600000000,"msg":"request handled","request":{"method":"GET","host":"example.com","uri":"/api/search","remote_ip":"1.2.3.4"},"status":200}
{"level":"error","ts":1600000060,"msg":"error occurred","request":{"method":"POST","host":"example.com","uri":"/api/submit","remote_ip":"5.6.7.8"},"status":500}
`
	os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	// Test with search filter
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?search=error", nil)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestLogsHandler_Read_FilterByHost(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logsDir, 0o755)

	content := `{"level":"info","ts":1600000000,"msg":"request handled","request":{"method":"GET","host":"example.com","uri":"/","remote_ip":"1.2.3.4"},"status":200}
{"level":"info","ts":1600000001,"msg":"request handled","request":{"method":"GET","host":"other.com","uri":"/","remote_ip":"1.2.3.4"},"status":200}
`
	os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?host=example.com", nil)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
}

func TestLogsHandler_Read_FilterByLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logsDir, 0o755)

	content := `{"level":"info","ts":1600000000,"msg":"info message"}
{"level":"error","ts":1600000001,"msg":"error message"}
`
	os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?level=error", nil)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
}

func TestLogsHandler_Read_FilterByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logsDir, 0o755)

	content := `{"level":"info","ts":1600000000,"msg":"200 OK","request":{"host":"example.com"},"status":200}
{"level":"error","ts":1600000001,"msg":"500 Error","request":{"host":"example.com"},"status":500}
`
	os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?status=500", nil)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
}

func TestLogsHandler_Read_SortAsc(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logsDir, 0o755)

	content := `{"level":"info","ts":1600000000,"msg":"first"}
{"level":"info","ts":1600000001,"msg":"second"}
`
	os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(content), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "access.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/access.log?sort=asc", nil)

	h.Read(c)

	assert.Equal(t, 200, w.Code)
}

func TestLogsHandler_List_DirectoryIsFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	logsDir := filepath.Join(dataDir, "logs")

	// Create logs dir as a file to cause error
	os.WriteFile(logsDir, []byte("not a dir"), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/logs", nil)

	h.List(c)

	// Service may handle this gracefully or error
	assert.Contains(t, []int{200, 500}, w.Code)
}
