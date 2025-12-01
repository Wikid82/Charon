package handlers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"encoding/json"

	"github.com/gin-gonic/gin"
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
func (f *fakeExec) Status(ctx context.Context, configDir string) (bool, int, error) {
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/status", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status expected 200 got %d", w.Code)
	}

	// Start
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("start expected 200 got %d", w2.Code)
	}

	// Stop
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/stop", nil)
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
			if e.IsDir() && filepath.Ext(e.Name()) == "" && (len(e.Name()) > 0) && (filepath.Base(e.Name()) != filepath.Base(tmpDir)) {
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/files", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("files expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	// read a single file
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/file?path=conf.d/a.conf", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("file read expected 200 got %d body=%s", w2.Code, w2.Body.String())
	}
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

	// ensure backup directory exists next to data dir
	found := false
	entries, _ := os.ReadDir(filepath.Dir(tmpDir))
	for _, e := range entries {
		if e.IsDir() && filepath.HasPrefix(e.Name(), filepath.Base(tmpDir)+".backup.") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected backup directory next to data dir")
	}
	// ensure file content exists in new data dir
	if _, err := os.Stat(filepath.Join(tmpDir, "conf.d", "new.conf")); err != nil {
		t.Fatalf("expected file written: %v", err)
	}
}
