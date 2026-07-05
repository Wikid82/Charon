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

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupLogsTest(t *testing.T) (*gin.Engine, string) {
	t.Helper()

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "cpm-logs-test")
	require.NoError(t, err)

	// LogService expects LogDir to be .../data/logs
	// It derives it from cfg.DatabasePath

	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory
	require.NoError(t, err)

	dbPath := filepath.Join(dataDir, "charon.db")

	// Create logs dir
	logsDir := filepath.Join(dataDir, "logs")
	err = os.MkdirAll(logsDir, 0o750) // #nosec G301 -- test directory
	require.NoError(t, err)

	// Create dummy log files with JSON content
	log1 := `{"level":"info","ts":1600000000,"msg":"request handled","request":{"method":"GET","host":"example.com","uri":"/","remote_ip":"1.2.3.4"},"status":200}`
	log2 := `{"level":"error","ts":1600000060,"msg":"error handled","request":{"method":"POST","host":"api.example.com","uri":"/submit","remote_ip":"5.6.7.8"},"status":500}`

	err = os.WriteFile(filepath.Join(logsDir, "access.log"), []byte(log1+"\n"+log2+"\n"), 0o600) // #nosec G306 -- test fixture
	require.NoError(t, err)
	// Write a charon.log and create a cpmp.log symlink to it for backward compatibility (cpmp is legacy)
	err = os.WriteFile(filepath.Join(logsDir, "charon.log"), []byte("app log line 1\napp log line 2"), 0o600) // #nosec G306 -- test fixture
	require.NoError(t, err)
	// Create legacy cpmp log symlink (cpmp is a legacy name for Charon)
	_ = os.Symlink(filepath.Join(logsDir, "charon.log"), filepath.Join(logsDir, "cpmp.log"))
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

	return r, tmpDir
}

func TestLogsLifecycle(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 1. List logs
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var logs []services.LogFile
	err := json.Unmarshal(resp.Body.Bytes(), &logs)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(logs), 2)

	hasAccess := false
	hasCharon := false
	for _, l := range logs {
		if l.Name == "access.log" {
			hasAccess = true
			require.Greater(t, l.Size, int64(0))
		}
		if l.Name == "charon.log" {
			hasCharon = true
			require.Greater(t, l.Size, int64(0))
		}
	}
	require.True(t, hasAccess)
	require.True(t, hasCharon)

	// 2. Read log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?limit=2", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var content struct {
		Filename string `json:"filename"`
		Logs     []any  `json:"logs"`
		Total    int    `json:"total"`
	}
	err = json.Unmarshal(resp.Body.Bytes(), &content)
	require.NoError(t, err)
	require.Len(t, content.Logs, 2)

	// 3. Download log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log/download", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "request handled")

	// 4. Read non-existent log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/missing.log", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// 5. Download non-existent log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/missing.log/download", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// 6. List logs error (delete directory)
	_ = os.RemoveAll(filepath.Join(tmpDir, "data", "logs"))
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	// ListLogs returns empty list if dir doesn't exist, so it should be 200 OK with empty list
	require.Equal(t, http.StatusOK, resp.Code)
	var emptyLogs []services.LogFile
	err = json.Unmarshal(resp.Body.Bytes(), &emptyLogs)
	require.NoError(t, err)
	require.Empty(t, emptyLogs)
}

func TestRead_SortByParam(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Fixture has status 200 (ts=1600000000) and 500 (ts=1600000060).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?sort_by=status&sort=asc", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		Logs []struct {
			Status int `json:"status"`
		} `json:"logs"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Len(t, body.Logs, 2)
	require.Equal(t, 200, body.Logs[0].Status)
	require.Equal(t, 500, body.Logs[1].Status)

	// Same field descending.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?sort_by=status&sort=desc", http.NoBody)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Len(t, body.Logs, 2)
	require.Equal(t, 500, body.Logs[0].Status)
}

func TestRead_InvalidSortBy(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?sort_by=password", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, "invalid sort_by: must be one of ts, level, method, uri, status", body.Error)
}

func TestRead_SkippedLinesInResponse(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// One valid JSON line, one corrupted line (NUL byte, not valid JSON).
	logsDir := filepath.Join(tmpDir, "data", "logs")
	content := "{\"level\":\"info\",\"ts\":1,\"msg\":\"ok\"}\ncorrupt\x00line\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "corrupt.log"), []byte(content), 0o600)) // #nosec G306 -- test fixture

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/corrupt.log", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		Total        int64 `json:"total"`
		SkippedLines int64 `json:"skipped_lines"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, int64(1), body.Total)
	require.Equal(t, int64(1), body.SkippedLines)
}

// Compat pin (spec §5.1.2 change 1): non-numeric limit was silently discarded
// (empty page); it is now an explicit 400.
func TestRead_LimitNonNumeric_Returns400(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?limit=abc", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Contains(t, resp.Body.String(), "error")
}

// Compat pin (spec §5.1.2 change 2): limit=0 used to produce an empty page;
// it is now treated as unset and defaults to 50.
func TestRead_LimitZero_DefaultsTo50(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?limit=0", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		Limit int   `json:"limit"`
		Total int64 `json:"total"`
		Logs  []any `json:"logs"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, 50, body.Limit)
	require.Len(t, body.Logs, 2)
}

func TestRead_LimitClampedTo500(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log?limit=9999", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		Limit int `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, 500, body.Limit)
}

func TestLogsHandler_PathTraversal(t *testing.T) {
	_, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Manually invoke handler to bypass Gin router cleaning
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "../access.log"}}

	cfg := &config.Config{
		DatabasePath: filepath.Join(tmpDir, "data", "charon.db"),
	}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	h.Download(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid filename")
}

// Sentinel mapping (spec §5.3.2): the service returns a wrapped
// ErrInvalidFilename and the Read handler maps it via errors.Is -> 400
// (no string matching).
func TestRead_InvalidFilename_400_ViaErrorsIs(t *testing.T) {
	_, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "../access.log"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/logs/x", http.NoBody)

	svc := services.NewLogService(&config.Config{DatabasePath: filepath.Join(tmpDir, "data", "charon.db")})
	h := NewLogsHandler(svc)

	h.Read(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid filename")
}

func TestDownload_InvalidFilename_400_ViaErrorsIs(t *testing.T) {
	_, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "..%2Fsecret"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/logs/x/download", http.NoBody)

	svc := services.NewLogService(&config.Config{DatabasePath: filepath.Join(tmpDir, "data", "charon.db")})
	h := NewLogsHandler(svc)

	h.Download(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid filename")
}

func TestDownload_ContentDispositionQuoted(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log/download", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, `attachment; filename="access.log"`, resp.Header().Get("Content-Disposition"))
}

func TestDownload_ContentType(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/access.log/download", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
}

func TestDownload_TraversalRejected(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Unknown (not in the raw log-directory listing) -> 404.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/unknown.log/download", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// Traversal-shaped via direct invocation (router would clean the path).
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "/etc/passwd"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/logs/x/download", http.NoBody)

	svc := services.NewLogService(&config.Config{DatabasePath: filepath.Join(tmpDir, "data", "charon.db")})
	h := NewLogsHandler(svc)
	h.Download(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// Alias servability end-to-end: both names of the charon.log/cpmp.log symlink
// pair must download successfully (allowlist is built pre-symlink-dedup).
func TestDownload_SymlinkAliasesBothServable(t *testing.T) {
	router, tmpDir := setupLogsTest(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for _, name := range []string{"charon.log", "cpmp.log"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/"+name+"/download", http.NoBody)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "alias %q must be servable", name)
		require.Contains(t, resp.Body.String(), "app log line 1")
	}
}
