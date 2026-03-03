package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionalAuth_NilServicePassThrough(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth(nil))
	r.GET("/", func(c *gin.Context) {
		_, hasUserID := c.Get("userID")
		_, hasRole := c.Get("role")
		assert.False(t, hasUserID)
		assert.False(t, hasRole)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
}

func TestOptionalAuth_EmergencyBypassPassThrough(t *testing.T) {
	t.Parallel()

	authService := setupAuthService(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("emergency_bypass", true)
		c.Next()
	})
	r.Use(OptionalAuth(authService))
	r.GET("/", func(c *gin.Context) {
		_, hasUserID := c.Get("userID")
		_, hasRole := c.Get("role")
		assert.False(t, hasUserID)
		assert.False(t, hasRole)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
}

func TestOptionalAuth_RoleAlreadyInContextSkipsAuth(t *testing.T) {
	t.Parallel()

	authService := setupAuthService(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(42))
		c.Next()
	})
	r.Use(OptionalAuth(authService))
	r.GET("/", func(c *gin.Context) {
		role, _ := c.Get("role")
		userID, _ := c.Get("userID")
		assert.Equal(t, "admin", role)
		assert.Equal(t, uint(42), userID)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
}

func TestOptionalAuth_NoTokenPassThrough(t *testing.T) {
	t.Parallel()

	authService := setupAuthService(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth(authService))
	r.GET("/", func(c *gin.Context) {
		_, hasUserID := c.Get("userID")
		_, hasRole := c.Get("role")
		assert.False(t, hasUserID)
		assert.False(t, hasRole)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
}

func TestOptionalAuth_InvalidTokenPassThrough(t *testing.T) {
	t.Parallel()

	authService := setupAuthService(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth(authService))
	r.GET("/", func(c *gin.Context) {
		_, hasUserID := c.Get("userID")
		_, hasRole := c.Get("role")
		assert.False(t, hasUserID)
		assert.False(t, hasRole)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-token")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
}

func TestOptionalAuth_ValidTokenSetsContext(t *testing.T) {
	t.Parallel()

	authService, db := setupAuthServiceWithDB(t)
	user := &models.User{Email: "optional-auth@example.com", Name: "Optional Auth", Role: models.RoleAdmin, Enabled: true}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.Create(user).Error)

	token, err := authService.GenerateToken(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth(authService))
	r.GET("/", func(c *gin.Context) {
		role, roleExists := c.Get("role")
		userID, userExists := c.Get("userID")
		require.True(t, roleExists)
		require.True(t, userExists)
		assert.Equal(t, "admin", role)
		assert.Equal(t, user.ID, userID)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
}
