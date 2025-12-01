package middleware

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/gin-gonic/gin"
)

func TestRequestLoggerSanitizesPath(t *testing.T) {
    old := logger.Log()
    buf := &bytes.Buffer{}
    logger.Init(true, buf)

    longPath := "/" + strings.Repeat("a", 300)

    router := gin.New()
    router.Use(RequestID())
    router.Use(RequestLogger())
    router.GET(longPath, func(c *gin.Context) { c.Status(http.StatusOK) })

    req := httptest.NewRequest(http.MethodGet, longPath, nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    out := buf.String()
    if strings.Contains(out, strings.Repeat("a", 300)) {
        t.Fatalf("logged unsanitized long path")
    }
    i := strings.Index(out, "path=")
    if i == -1 {
        t.Fatalf("could not find path in logs: %s", out)
    }
    sub := out[i:]
    j := strings.Index(sub, " request_id=")
    if j == -1 {
        t.Fatalf("could not isolate path field from logs: %s", out)
    }
    pathField := sub[len("path=") : j]
    if strings.Contains(pathField, "\n") || strings.Contains(pathField, "\r") {
        t.Fatalf("path field contains control characters after sanitization: %s", pathField)
    }
    _ = old // silence unused var
}

func TestRequestLoggerIncludesRequestID(t *testing.T) {
    buf := &bytes.Buffer{}
    logger.Init(true, buf)

    router := gin.New()
    router.Use(RequestID())
    router.Use(RequestLogger())
    router.GET("/ok", func(c *gin.Context) { c.String(200, "ok") })

    req := httptest.NewRequest(http.MethodGet, "/ok", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("unexpected status code: %d", w.Code)
    }
    out := buf.String()
    if !strings.Contains(out, "request_id") {
        t.Fatalf("expected log output to include request_id: %s", out)
    }
    if !strings.Contains(out, "handled request") {
        t.Fatalf("expected log output to indicate handled request: %s", out)
    }
}
