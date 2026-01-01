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

	req := httptest.NewRequest(http.MethodGet, "/panic", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	if !strings.Contains(out, "PANIC: test panic") {
		t.Fatalf("log did not include panic message: %s", out)
	}
	// Stack traces are no longer logged to prevent CWE-312 (clear-text logging of sensitive data)
	// We now log a debug message indicating stack trace is available but not logged
	if !strings.Contains(out, "Stack trace available") {
		t.Fatalf("verbose log did not include stack trace availability message: %s", out)
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

	req := httptest.NewRequest(http.MethodGet, "/panic", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	if !strings.Contains(out, "PANIC: brief panic") {
		t.Fatalf("log did not include panic message: %s", out)
	}
	// Stack traces should not appear in non-verbose mode
	if strings.Contains(out, "Stacktrace:") {
		t.Fatalf("non-verbose log unexpectedly included stacktrace: %s", out)
	}
}

// TestRecoveryDoesNotLogSensitiveHeaders verifies that sensitive headers
// are no longer logged at all (not even redacted) to prevent CWE-312.
func TestRecoveryDoesNotLogSensitiveHeaders(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/panic", http.NoBody)
	// Add sensitive header that should not appear in logs at all
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	// Verify sensitive token is not logged
	if strings.Contains(out, "secret-token") {
		t.Fatalf("log contained sensitive token: %s", out)
	}
	// Headers are no longer logged at all to prevent potential information leakage
	if strings.Contains(out, "headers") {
		t.Fatalf("log should not include headers field: %s", out)
	}
	// Verify sanitized panic message is logged
	if !strings.Contains(out, "PANIC: sensitive panic") {
		t.Fatalf("log did not include sanitized panic message: %s", out)
	}
}

// TestRecoveryTruncatesLongPanicMessage verifies that panic messages longer
// than 200 characters are truncated with "..." suffix.
func TestRecoveryTruncatesLongPanicMessage(t *testing.T) {
	old := log.Writer()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	defer log.SetOutput(old)

	logger.Init(false, buf)

	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery(false))

	// Create a panic message longer than 200 characters
	longMessage := strings.Repeat("x", 250)
	router.GET("/panic", func(c *gin.Context) {
		panic(longMessage)
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	// Should contain truncated message (200 chars + "...")
	expectedTruncated := strings.Repeat("x", 200) + "..."
	if !strings.Contains(out, expectedTruncated) {
		t.Fatalf("log should contain truncated panic message with '...': %s", out)
	}
	// Should NOT contain the full 250 char message
	if strings.Contains(out, longMessage) {
		t.Fatalf("log should not contain full long panic message: %s", out)
	}
}

// TestRecoveryNoPanicNormalFlow verifies that middleware passes through
// normally when no panic occurs.
func TestRecoveryNoPanicNormalFlow(t *testing.T) {
	old := log.Writer()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	defer log.SetOutput(old)

	logger.Init(false, buf)

	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery(true))
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	out := buf.String()
	// Should NOT contain PANIC in logs
	if strings.Contains(out, "PANIC") {
		t.Fatalf("log should not contain PANIC for normal flow: %s", out)
	}
}

// TestRecoveryPanicWithNilValue tests recovery from panic with a nil-like value.
// Note: panic(nil) behavior changed in Go 1.21+ and triggers linter warnings,
// so we use an explicit error value instead.
func TestRecoveryPanicWithNilValue(t *testing.T) {
	old := log.Writer()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	defer log.SetOutput(old)

	logger.Init(false, buf)

	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery(false))
	router.GET("/panic-nil", func(c *gin.Context) {
		panic("intentional test panic with nil-like value")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-nil", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify the panic was recovered and returned 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	out := buf.String()
	if !strings.Contains(out, "PANIC") {
		t.Error("expected PANIC in log output")
	}
}
