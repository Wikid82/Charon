package handlers

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"

    "github.com/Wikid82/charon/backend/internal/services"
    "github.com/gin-gonic/gin"
)

// Use a real BackupService, but point it at tmpDir for isolation

func TestBackupHandlerQuick(t *testing.T) {
    gin.SetMode(gin.TestMode)
    tmpDir := t.TempDir()
    // prepare a fake "database" so CreateBackup can find it
    dbPath := filepath.Join(tmpDir, "db.sqlite")
    if err := os.WriteFile(dbPath, []byte("db"), 0o644); err != nil {
        t.Fatalf("failed to create tmp db: %v", err)
    }

    svc := &services.BackupService{DataDir: tmpDir, BackupDir: tmpDir, DatabaseName: "db.sqlite", Cron: nil}
    h := NewBackupHandler(svc)

    r := gin.New()
    // register routes used
    r.GET("/backups", h.List)
    r.POST("/backups", h.Create)
    r.DELETE("/backups/:filename", h.Delete)
    r.GET("/backups/:filename", h.Download)
    r.POST("/backups/:filename/restore", h.Restore)

    // List
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/backups", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK { t.Fatalf("expected 200, got %d", w.Code) }

    // Create (backup)
    w2 := httptest.NewRecorder()
    req2 := httptest.NewRequest(http.MethodPost, "/backups", nil)
    r.ServeHTTP(w2, req2)
    if w2.Code != http.StatusCreated { t.Fatalf("create expected 201 got %d", w2.Code) }

    var createResp struct{ Filename string `json:"filename"` }
    if err := json.Unmarshal(w2.Body.Bytes(), &createResp); err != nil {
        t.Fatalf("invalid create json: %v", err)
    }

    // Delete missing
    w3 := httptest.NewRecorder()
    req3 := httptest.NewRequest(http.MethodDelete, "/backups/missing", nil)
    r.ServeHTTP(w3, req3)
    if w3.Code != http.StatusNotFound { t.Fatalf("delete missing expected 404 got %d", w3.Code) }

    // Download missing
    w4 := httptest.NewRecorder()
    req4 := httptest.NewRequest(http.MethodGet, "/backups/missing", nil)
    r.ServeHTTP(w4, req4)
    if w4.Code != http.StatusNotFound { t.Fatalf("download missing expected 404 got %d", w4.Code) }

    // Download present (use filename returned from create)
    w5 := httptest.NewRecorder()
    req5 := httptest.NewRequest(http.MethodGet, "/backups/"+createResp.Filename, nil)
    r.ServeHTTP(w5, req5)
    if w5.Code != http.StatusOK { t.Fatalf("download expected 200 got %d", w5.Code) }

    // Restore missing
    w6 := httptest.NewRecorder()
    req6 := httptest.NewRequest(http.MethodPost, "/backups/missing/restore", nil)
    r.ServeHTTP(w6, req6)
    if w6.Code != http.StatusNotFound { t.Fatalf("restore missing expected 404 got %d", w6.Code) }

    // Restore ok
    w7 := httptest.NewRecorder()
    req7 := httptest.NewRequest(http.MethodPost, "/backups/"+createResp.Filename+"/restore", nil)
    r.ServeHTTP(w7, req7)
    if w7.Code != http.StatusOK { t.Fatalf("restore expected 200 got %d", w7.Code) }
}
