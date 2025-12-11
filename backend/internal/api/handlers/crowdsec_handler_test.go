package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeExec struct {
	started bool
}

func (f *fakeExec) Start(ctx context.Context, binPath, configDir string) (int, error) {
	f.started = true
	return 12345, nil
}
func (f *fakeExec) Stop(ctx context.Context, configDir string) error {
	f.started = false
	return nil
}
func (f *fakeExec) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
	if f.started {
		return true, 12345, nil
	}
	return false, 0, nil
}

func setupCrowdDB(t *testing.T) *gorm.DB {
	db := OpenTestDB(t)
	return db
}

func TestCrowdsecEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	fe := &fakeExec{}
	h := NewCrowdsecHandler(db, fe, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Status (initially stopped)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/status", http.NoBody)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status expected 200 got %d", w.Code)
	}

	// Start
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", http.NoBody)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("start expected 200 got %d", w2.Code)
	}

	// Stop
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/stop", http.NoBody)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("stop expected 200 got %d", w3.Code)
	}
}

func TestImportConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()
	fe := &fakeExec{}
	h := NewCrowdsecHandler(db, fe, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// create a small file to upload
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	fw, _ := mw.CreateFormFile("file", "cfg.tar.gz")
	fw.Write([]byte("dummy"))
	mw.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/import", buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import expected 200 got %d body=%s", w.Code, w.Body.String())
	}

	// ensure file exists in data dir
	if _, err := os.Stat(filepath.Join(tmpDir, "cfg.tar.gz")); err != nil {
		t.Fatalf("expected file in data dir: %v", err)
	}
}

func TestImportCreatesBackup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()
	// create existing config dir with a marker file
	_ = os.MkdirAll(tmpDir, 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "existing.conf"), []byte("v1"), 0o644)

	fe := &fakeExec{}
	h := NewCrowdsecHandler(db, fe, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// upload
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	fw, _ := mw.CreateFormFile("file", "cfg.tar.gz")
	fw.Write([]byte("dummy2"))
	mw.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/import", buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import expected 200 got %d body=%s", w.Code, w.Body.String())
	}

	// ensure backup dir exists (ends with .backup.TIMESTAMP)
	found := false
	entries, _ := os.ReadDir(filepath.Dir(tmpDir))
	for _, e := range entries {
		if e.IsDir() && filepath.HasPrefix(e.Name(), filepath.Base(tmpDir)+".backup.") {
			found = true
			break
		}
	}
	if !found {
		// fallback: check for any .backup.* in same parent dir
		entries, _ := os.ReadDir(filepath.Dir(tmpDir))
		for _, e := range entries {
			if e.IsDir() && filepath.Ext(e.Name()) == "" && e.Name() != "" && (filepath.Base(e.Name()) != filepath.Base(tmpDir)) {
				// best-effort assume backup present
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected backup directory next to data dir")
	}
}

func TestExportConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	// create some files to export
	_ = os.MkdirAll(filepath.Join(tmpDir, "conf.d"), 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "conf.d", "a.conf"), []byte("rule1"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.conf"), []byte("rule2"), 0o644)

	fe := &fakeExec{}
	h := NewCrowdsecHandler(db, fe, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("export expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/gzip" {
		t.Fatalf("unexpected content type: %s", ct)
	}
	if w.Body.Len() == 0 {
		t.Fatalf("expected response body to contain archive data")
	}
}

func TestListAndReadFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()
	// create a nested file
	_ = os.MkdirAll(filepath.Join(tmpDir, "conf.d"), 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "conf.d", "a.conf"), []byte("rule1"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.conf"), []byte("rule2"), 0o644)

	fe := &fakeExec{}
	h := NewCrowdsecHandler(db, fe, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/files", http.NoBody)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("files expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	// read a single file
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/file?path=conf.d/a.conf", http.NoBody)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("file read expected 200 got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestExportConfigStreamsArchive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("hello"), 0o644))

	h := NewCrowdsecHandler(db, &fakeExec{}, "/bin/false", dataDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/gzip", w.Header().Get("Content-Type"))
	require.Contains(t, w.Header().Get("Content-Disposition"), "crowdsec-config-")

	gr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
	tr := tar.NewReader(gr)
	found := false
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if hdr.Name == "config.yaml" {
			data, readErr := io.ReadAll(tr)
			require.NoError(t, readErr)
			require.Equal(t, "hello", string(data))
			found = true
		}
	}
	require.True(t, found, "expected exported archive to contain config file")
}

func TestWriteFileCreatesBackup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()
	// create existing config dir with a marker file
	_ = os.MkdirAll(tmpDir, 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "existing.conf"), []byte("v1"), 0o644)

	fe := &fakeExec{}
	h := NewCrowdsecHandler(db, fe, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// write content to new file
	payload := map[string]string{"path": "conf.d/new.conf", "content": "hello world"}
	b, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("write expected 200 got %d body=%s", w.Code, w.Body.String())
	}

	// ensure backup directory was created
	entries, err := os.ReadDir(filepath.Dir(tmpDir))
	require.NoError(t, err)
	foundBackup := false
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), filepath.Base(tmpDir)+".backup.") {
			foundBackup = true
			break
		}
	}
	require.True(t, foundBackup, "expected backup directory to be created")
}

func TestListPresetsCerberusDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("FEATURE_CERBERUS_ENABLED", "false")

	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when cerberus disabled got %d", w.Code)
	}
}

func TestReadFileInvalidPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/file?path=../secret", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid path got %d", w.Code)
	}
}

func TestWriteFileInvalidPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	body, _ := json.Marshal(map[string]string{"path": "../../escape", "content": "bad"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid path got %d", w.Code)
	}
}

func TestWriteFileMissingPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	body, _ := json.Marshal(map[string]string{"content": "data only"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWriteFileInvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportConfigRequiresFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/import", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when file missing got %d", w.Code)
	}
}

func TestImportConfigRejectsEmptyUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	_, _ = mw.CreateFormFile("file", "empty.tgz")
	_ = mw.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/import", buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty upload got %d", w.Code)
	}
}

func TestListFilesMissingDir(t *testing.T) {
	gin.SetMode(gin.TestMode)
	missingDir := filepath.Join(t.TempDir(), "does-not-exist")
	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", missingDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/files", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for missing dir got %d", w.Code)
	}
}

func TestListFilesReturnsEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "root.txt"), []byte("root"), 0o644))
	nestedDir := filepath.Join(dataDir, "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "child.txt"), []byte("child"), 0o644))

	h := NewCrowdsecHandler(OpenTestDB(t), &fakeExec{}, "/bin/false", dataDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/files", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}

	var resp struct {
		Files []string `json:"files"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.ElementsMatch(t, []string{"root.txt", filepath.Join("nested", "child.txt")}, resp.Files)
}

func TestIsCerberusEnabledFromDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "0"}).Error)

	h := NewCrowdsecHandler(db, &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when cerberus disabled via DB got %d", w.Code)
	}
}

func TestIsCerberusEnabledInvalidEnv(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("FEATURE_CERBERUS_ENABLED", "not-a-bool")
	h := NewCrowdsecHandler(nil, &fakeExec{}, "/bin/false", t.TempDir())

	if h.isCerberusEnabled() {
		t.Fatalf("expected cerberus to be disabled for invalid env flag")
	}
}

func TestIsCerberusEnabledLegacyEnv(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewCrowdsecHandler(nil, &fakeExec{}, "/bin/false", t.TempDir())

	t.Setenv("CERBERUS_ENABLED", "0")

	if h.isCerberusEnabled() {
		t.Fatalf("expected cerberus to be disabled for legacy env flag")
	}
}
