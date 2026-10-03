package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

// The shared chain owns request logging and panic recovery on the router.
func TestNewRouter_BaseChainLogsOnceAndRecoversPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	buf := &bytes.Buffer{}
	logger.Init(false, buf)
	secret, err := selfhop.NewSecret()
	require.NoError(t, err)

	router := NewRouter("", "", nil)
	router.Use(middleware.BaseChain(secret, security.TrustedProxyMatcher{}, false)...)
	router.GET("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/boom", func(_ *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", http.NoBody))
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 1, strings.Count(buf.String(), "handled request"))

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", http.NoBody))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, buf.String(), "PANIC")
}
