package handlers

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "encoding/json"
)

func TestImportUploadSanitizesFilename(t *testing.T) {
    gin.SetMode(gin.TestMode)
    tmpDir := t.TempDir()
    // set up in-memory DB for handler
    db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    if err != nil {
        t.Fatalf("failed to open in-memory db: %v", err)
    }
    svc := NewImportHandler(db, "/usr/bin/caddy", tmpDir, "")

    router := gin.New()
    router.POST("/import/upload", svc.Upload)

    buf := &bytes.Buffer{}
    logger.Init(true, buf)

    maliciousFilename := "../evil\nfile.caddy"
    payload := map[string]interface{}{"filename": maliciousFilename, "content": "site { respond \"ok\" }"}
    bodyBytes, _ := json.Marshal(payload)
    req := httptest.NewRequest(http.MethodPost, "/import/upload", bytes.NewReader(bodyBytes))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    out := buf.String()
    if strings.Contains(out, "\n") {
        t.Fatalf("log contained raw newline in filename: %s", out)
    }
    if strings.Contains(out, "..") {
        t.Fatalf("log contained path traversal in filename: %s", out)
    }
}
