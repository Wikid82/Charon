package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/gin-gonic/gin"
)

func TestImportUploadSanitizesFilename(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	// set up in-memory DB for handler
	db := OpenTestDB(t)
	// Create a fake caddy executable to avoid dependency on system binary
	fakeCaddy := filepath.Join(tmpDir, "caddy")
	os.WriteFile(fakeCaddy, []byte("#!/bin/sh\nexit 0"), 0755)
	svc := NewImportHandler(db, fakeCaddy, tmpDir, "")

	router := gin.New()
	router.Use(middleware.RequestID())
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

	// Extract the logged filename from either text or JSON log format
	textRegex := regexp.MustCompile(`filename=?"?([^"\s]*)"?`)
	jsonRegex := regexp.MustCompile(`"filename":"([^"]*)"`)
	var loggedFilename string
	if m := textRegex.FindStringSubmatch(out); len(m) == 2 {
		loggedFilename = m[1]
	} else if m := jsonRegex.FindStringSubmatch(out); len(m) == 2 {
		loggedFilename = m[1]
	} else {
		// if we can't extract a filename value, fail the test
		t.Fatalf("could not extract filename from logs: %s", out)
	}

	if strings.Contains(loggedFilename, "\n") || strings.Contains(loggedFilename, "\r") {
		t.Fatalf("log filename contained raw newline: %q", loggedFilename)
	}
	if strings.Contains(loggedFilename, "..") {
		t.Fatalf("log filename contained path traversal: %q", loggedFilename)
	}
}
