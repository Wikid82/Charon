package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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

	// Create a gin test context and use it to call handler directly
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Ensure request-scoped logger is present and writes to our buffer
	c.Set("logger", logger.WithFields(map[string]interface{}{"test": "1"}))

	// initialize logger to buffer
	buf := &bytes.Buffer{}
	logger.Init(true, buf)

	// Create a malicious filename with newline and path components
	malicious := "../evil\nname"
	c.Request = httptest.NewRequest(http.MethodGet, "/backups/"+strings.ReplaceAll(malicious, "\n", "%0A")+"/restore", nil)
	// Call handler directly with the test context
	h.Restore(c)

	out := buf.String()
	// Optionally we could assert on the response status code here if needed
	textRegex := regexp.MustCompile(`filename=?"?([^"\s]*)"?`)
	jsonRegex := regexp.MustCompile(`"filename":"([^"]*)"`)
	var loggedFilename string
	if m := textRegex.FindStringSubmatch(out); len(m) == 2 {
		loggedFilename = m[1]
	} else if m := jsonRegex.FindStringSubmatch(out); len(m) == 2 {
		loggedFilename = m[1]
	} else {
		t.Fatalf("could not extract filename from logs: %s", out)
	}

	if strings.Contains(loggedFilename, "\n") || strings.Contains(loggedFilename, "\r") {
		t.Fatalf("log filename contained raw newline: %q", loggedFilename)
	}
	if strings.Contains(loggedFilename, "..") {
		t.Fatalf("log filename contained path traversals in filename: %q", loggedFilename)
	}
}
