package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/gin-gonic/gin"
)

func TestRecoveryLogsStacktraceVerbose(t *testing.T) {
	old := log.Writer()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	defer log.SetOutput(old)
	// Ensure structured logger writes to the same buffer and enable debug
	logger.Init(true, buf)

	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery(true))
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	if !strings.Contains(out, "PANIC: test panic") {
		t.Fatalf("log did not include panic message: %s", out)
	}
	if !strings.Contains(out, "Stacktrace:") {
		t.Fatalf("verbose log did not include stack trace: %s", out)
	}
	if !strings.Contains(out, "request_id") {
		t.Fatalf("verbose log did not include request_id: %s", out)
	}
}

func TestRecoveryLogsBriefWhenNotVerbose(t *testing.T) {
	old := log.Writer()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	defer log.SetOutput(old)

	// Ensure structured logger writes to the same buffer and keep debug off
	logger.Init(false, buf)
	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery(false))
	router.GET("/panic", func(c *gin.Context) {
		panic("brief panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	if !strings.Contains(out, "PANIC: brief panic") {
		t.Fatalf("log did not include panic message: %s", out)
	}
	if strings.Contains(out, "Stacktrace:") {
		t.Fatalf("non-verbose log unexpectedly included stacktrace: %s", out)
	}
}

func TestRecoverySanitizesHeadersAndPath(t *testing.T) {
	old := log.Writer()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	defer log.SetOutput(old)

	// Ensure structured logger writes to the same buffer and enable debug
	logger.Init(true, buf)

	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery(true))
	router.GET("/panic", func(c *gin.Context) {
		panic("sensitive panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	// Add sensitive header that should be redacted
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	if strings.Contains(out, "secret-token") {
		t.Fatalf("log contained sensitive token: %s", out)
	}
	if !strings.Contains(out, "<redacted>") {
		t.Fatalf("log did not include redaction marker: %s", out)
	}
}
