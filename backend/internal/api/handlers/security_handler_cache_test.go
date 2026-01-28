package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

type testCacheInvalidator struct {
	calls int
}

func (t *testCacheInvalidator) InvalidateCache() {
	t.calls++
}

func TestSecurityHandler_ToggleSecurityModule_InvalidatesCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	cache := &testCacheInvalidator{}
	handler := NewSecurityHandlerWithDeps(config.SecurityConfig{}, db, nil, cache)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/enable", handler.EnableWAF)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/enable", http.NoBody)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, cache.calls)

	var setting models.Setting
	require.NoError(t, db.Where("key = ?", "security.waf.enabled").First(&setting).Error)
	require.Equal(t, "true", setting.Value)
}
