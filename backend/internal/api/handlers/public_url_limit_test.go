package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// The URL check routes refuse a body over 64 KiB with 413 and read one at the limit.
func TestPublicURLChecksLimitTheRequestBody(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	handler := NewSettingsHandler(db)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, string(models.RoleAdmin))
		c.Next()
	})
	router.POST("/validate-url", LimitSmallJSONBody(), handler.ValidatePublicURL)
	router.POST("/test-url", LimitSmallJSONBody(), handler.TestPublicURL)

	for _, path := range []string{"/validate-url", "/test-url"} {
		t.Run(path, func(t *testing.T) {
			over := postPaddedURL(router, path, maxSmallJSONBodyBytes+1)
			require.Equal(t, http.StatusRequestEntityTooLarge, over.Code, over.Body.String())
			assert.Equal(t, codeRequestTooLarge, decodeBody(t, over)["error_code"])

			atLimit := postPaddedURL(router, path, maxSmallJSONBodyBytes)
			assert.NotEqual(t, http.StatusRequestEntityTooLarge, atLimit.Code, atLimit.Body.String())
		})
	}
}

// postPaddedURL posts a URL check body of exactly size bytes.
func postPaddedURL(router *gin.Engine, path string, size int) *httptest.ResponseRecorder {
	const head = `{"url":"https://example.com","pad":"`
	const tail = `"}`
	body := head + strings.Repeat("a", size-len(head)-len(tail)) + tail
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
