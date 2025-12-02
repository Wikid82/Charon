package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/gin-gonic/gin"
)

func TestRequestIDAddsHeaderAndLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger.Init(true, buf)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		// Ensure logger exists in context and header is present
		if _, ok := c.Get("logger"); !ok {
			t.Fatalf("expected request-scoped logger in context")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get(RequestIDHeader) == "" {
		t.Fatalf("expected response to include X-Request-ID header")
	}
}
