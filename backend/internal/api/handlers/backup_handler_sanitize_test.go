package handlers

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "path/filepath"
    "testing"
    "strings"
    "os"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/Wikid82/charon/backend/internal/services"
    "github.com/gin-gonic/gin"
)

func TestBackupHandlerSanitizesFilename(t *testing.T) {
    gin.SetMode(gin.TestMode)
    tmpDir := t.TempDir()
    // prepare a fake "database"
    dbPath := filepath.Join(tmpDir, "db.sqlite")
    if err := os.WriteFile(dbPath, []byte("db"), 0o644); err != nil {
        t.Fatalf("failed to create tmp db: %v", err)
    }

    svc := &services.BackupService{DataDir: tmpDir, BackupDir: tmpDir, DatabaseName: "db.sqlite", Cron: nil}
    h := NewBackupHandler(svc)

    router := gin.New()
    router.GET("/backups/:filename/restore", h.Restore)

    // initialize logger to buffer
    buf := &bytes.Buffer{}
    logger.Init(true, buf)

    // Create a malicious filename with newline and path components
    malicious := "../evil\nname"
    // Use path escape to send as URL
    req := httptest.NewRequest(http.MethodPost, "/backups/"+strings.ReplaceAll(malicious, "\n", "%0A")+"/restore", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    out := buf.String()
    if strings.Contains(out, "\n") {
        t.Fatalf("log contained raw newline in filename: %s", out)
    }
    if strings.Contains(out, "..") {
        t.Fatalf("log contained path traversals in filename: %s", out)
    }
}
