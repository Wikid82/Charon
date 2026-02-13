package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthService(t *testing.T) *services.AuthService {
	authService, _ := setupAuthServiceWithDB(t)
	return authService
}

func setupAuthServiceWithDB(t *testing.T) (*services.AuthService, *gorm.DB) {
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	_ = db.AutoMigrate(&models.User{})
	cfg := config.Config{JWTSecret: "test-secret"}
	return services.NewAuthService(db, cfg), db
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// We pass nil for authService because we expect it to fail before using it
	r.Use(AuthMiddleware(nil))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization header required")
}

func TestAuthMiddleware_EmergencyBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("emergency_bypass", true)
		c.Next()
	})
	r.Use(AuthMiddleware(nil))
	r.GET("/test", func(c *gin.Context) {
		role, _ := c.Get("role")
		userID, _ := c.Get("userID")
		assert.Equal(t, "admin", role)
		assert.Equal(t, uint(0), userID)
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.Use(RequireRole("admin"))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.Use(RequireRole("admin"))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthMiddleware_Cookie(t *testing.T) {
	authService := setupAuthService(t)
	user, err := authService.Register("test@example.com", "password", "Test User")
	require.NoError(t, err)
	token, err := authService.GenerateToken(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		assert.Equal(t, user.ID, userID)
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	authService := setupAuthService(t)
	user, err := authService.Register("test@example.com", "password", "Test User")
	require.NoError(t, err)
	token, err := authService.GenerateToken(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		assert.Equal(t, user.ID, userID)
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_PrefersCookieOverAuthorizationHeader(t *testing.T) {
	authService := setupAuthService(t)
	cookieUser, _ := authService.Register("cookie-header@example.com", "password", "Cookie Header User")
	cookieToken, _ := authService.GenerateToken(cookieUser)
	headerUser, _ := authService.Register("header@example.com", "password", "Header User")
	headerToken, _ := authService.GenerateToken(headerUser)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		assert.Equal(t, cookieUser.ID, userID)
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+headerToken)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: cookieToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_UsesCookieWhenAuthorizationHeaderIsInvalid(t *testing.T) {
	authService := setupAuthService(t)
	user, err := authService.Register("cookie-valid@example.com", "password", "Cookie Valid User")
	require.NoError(t, err)
	token, err := authService.GenerateToken(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		assert.Equal(t, user.ID, userID)
		c.Status(http.StatusOK)
	})

	req, err := http.NewRequest("GET", "/test", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer invalid-token")
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_UsesLastNonEmptyCookieWhenDuplicateCookiesExist(t *testing.T) {
	authService := setupAuthService(t)
	user, err := authService.Register("dupecookie@example.com", "password", "Dup Cookie User")
	require.NoError(t, err)
	token, err := authService.GenerateToken(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		assert.Equal(t, user.ID, userID)
		c.Status(http.StatusOK)
	})

	req, err := http.NewRequest("GET", "/test", http.NoBody)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: ""})
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	authService := setupAuthService(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid token")
}

func TestRequireRole_MissingRoleInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// No role set in context
	r.Use(RequireRole("admin"))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_QueryParamFallback(t *testing.T) {
	authService := setupAuthService(t)
	user, err := authService.Register("test@example.com", "password", "Test User")
	require.NoError(t, err)
	token, err := authService.GenerateToken(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		assert.Equal(t, user.ID, userID)
		c.Status(http.StatusOK)
	})

	// Test that query param auth still works (deprecated fallback)
	req, err := http.NewRequest("GET", "/test?token="+token, http.NoBody)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_PrefersCookieOverQueryParam(t *testing.T) {
	authService := setupAuthService(t)

	// Create two different users
	cookieUser, err := authService.Register("cookie@example.com", "password", "Cookie User")
	require.NoError(t, err)
	cookieToken, err := authService.GenerateToken(cookieUser)
	require.NoError(t, err)

	queryUser, err := authService.Register("query@example.com", "password", "Query User")
	require.NoError(t, err)
	queryToken, err := authService.GenerateToken(queryUser)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		// Should use the cookie user, not the query param user
		assert.Equal(t, cookieUser.ID, userID)
		c.Status(http.StatusOK)
	})

	// Both cookie and query param provided - cookie should win
	req, err := http.NewRequest("GET", "/test?token="+queryToken, http.NoBody)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: cookieToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_RejectsDisabledUserToken(t *testing.T) {
	authService, db := setupAuthServiceWithDB(t)
	user, err := authService.Register("disabled@example.com", "password", "Disabled User")
	require.NoError(t, err)

	token, err := authService.GenerateToken(user)
	require.NoError(t, err)

	require.NoError(t, db.Model(&models.User{}).Where("id = ?", user.ID).Update("enabled", false).Error)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, err := http.NewRequest("GET", "/test", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_RejectsDeletedUserToken(t *testing.T) {
	authService, db := setupAuthServiceWithDB(t)
	user, err := authService.Register("deleted@example.com", "password", "Deleted User")
	require.NoError(t, err)

	token, err := authService.GenerateToken(user)
	require.NoError(t, err)

	require.NoError(t, db.Delete(&models.User{}, user.ID).Error)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authService))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, err := http.NewRequest("GET", "/test", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
