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
