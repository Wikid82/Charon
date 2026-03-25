package handlers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthHandler(t *testing.T) (*AuthHandler, *gorm.DB) {
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	_ = db.AutoMigrate(&models.User{}, &models.Setting{})

	cfg := config.Config{JWTSecret: "test-secret"}
	authService := services.NewAuthService(db, cfg)
	return NewAuthHandler(authService), db
}

func TestAuthHandler_Login(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	// Create user
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	r := gin.New()
	r.POST("/login", handler.Login)

	// Success
	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "token")
}

func TestSetSecureCookie_HTTPS_Strict(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "https://example.com/login", http.NoBody)
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.True(t, c.Secure)
	assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
}

func TestSetSecureCookie_HTTP_Lax(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://192.0.2.10/login", http.NoBody)
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.True(t, c.Secure)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
}

func TestSetSecureCookie_HTTP_Loopback_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://127.0.0.1:8080/login", http.NoBody)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_ForwardedHTTPS_LocalhostForcesInsecure(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://localhost:8080/login", http.NoBody)
	req.Host = "localhost:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_ForwardedHTTPS_LoopbackForcesInsecure(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://127.0.0.1:8080/login", http.NoBody)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_ForwardedHostLocalhostForcesInsecure(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://charon.local/login", http.NoBody)
	req.Host = "charon.internal:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "localhost:8080")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_OriginLoopbackForcesInsecure(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://service.internal/login", http.NoBody)
	req.Host = "service.internal:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_PrivateIP_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://192.168.1.50:8080/login", http.NoBody)
	req.Host = "192.168.1.50:8080"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_10Network_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://10.0.0.5:8080/login", http.NoBody)
	req.Host = "10.0.0.5:8080"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_172Network_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://172.16.0.1:8080/login", http.NoBody)
	req.Host = "172.16.0.1:8080"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTPS_PrivateIP_Secure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "https://192.168.1.50:8080/login", http.NoBody)
	req.Host = "192.168.1.50:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_IPv6ULA_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://[fd12::1]:8080/login", http.NoBody)
	req.Host = "[fd12::1]:8080"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_PublicIP_Secure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://203.0.113.5:8080/login", http.NoBody)
	req.Host = "203.0.113.5:8080"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestIsProduction(t *testing.T) {
	t.Setenv("CHARON_ENV", "production")
	assert.True(t, isProduction())

	t.Setenv("CHARON_ENV", "prod")
	assert.True(t, isProduction())

	t.Setenv("CHARON_ENV", "development")
	assert.False(t, isProduction())
}

func TestRequestScheme(t *testing.T) {

	t.Run("forwarded proto first value wins", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.Header.Set("X-Forwarded-Proto", "HTTPS, http")
		ctx.Request = req

		assert.Equal(t, "https", requestScheme(ctx))
	})

	t.Run("tls request", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "https://example.com", http.NoBody)
		req.TLS = &tls.ConnectionState{}
		ctx.Request = req

		assert.Equal(t, "https", requestScheme(ctx))
	})

	t.Run("url scheme fallback", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.URL.Scheme = "HTTP"
		ctx.Request = req

		assert.Equal(t, "http", requestScheme(ctx))
	})

	t.Run("default http fallback", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "/", http.NoBody)
		req.URL.Scheme = ""
		ctx.Request = req

		assert.Equal(t, "http", requestScheme(ctx))
	})
}

func TestHostHelpers(t *testing.T) {
	t.Run("normalizeHost", func(t *testing.T) {
		assert.Equal(t, "", normalizeHost("   "))
		assert.Equal(t, "example.com", normalizeHost("example.com:8080"))
		assert.Equal(t, "::1", normalizeHost("[::1]:2020"))
		assert.Equal(t, "localhost", normalizeHost("localhost"))
	})

	t.Run("originHost", func(t *testing.T) {
		assert.Equal(t, "", originHost(""))
		assert.Equal(t, "", originHost("::://bad-url"))
		assert.Equal(t, "localhost", originHost("http://localhost:8080/path"))
	})

	t.Run("isLocalOrPrivateHost", func(t *testing.T) {
		assert.True(t, isLocalOrPrivateHost("localhost"))
		assert.True(t, isLocalOrPrivateHost("127.0.0.1"))
		assert.True(t, isLocalOrPrivateHost("::1"))
		assert.True(t, isLocalOrPrivateHost("192.168.1.50"))
		assert.True(t, isLocalOrPrivateHost("10.0.0.1"))
		assert.True(t, isLocalOrPrivateHost("172.16.0.1"))
		assert.True(t, isLocalOrPrivateHost("fd12::1"))
		assert.False(t, isLocalOrPrivateHost("203.0.113.5"))
		assert.False(t, isLocalOrPrivateHost("example.com"))
	})
}

func TestIsLocalRequest(t *testing.T) {

	t.Run("forwarded host list includes localhost", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.Host = "example.com"
		req.Header.Set("X-Forwarded-Host", "example.com, localhost:8080")
		ctx.Request = req

		assert.True(t, isLocalRequest(ctx))
	})

	t.Run("origin loopback", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.Header.Set("Origin", "http://127.0.0.1:3000")
		ctx.Request = req

		assert.True(t, isLocalRequest(ctx))
	})

	t.Run("non local request", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.Host = "example.com"
		ctx.Request = req

		assert.False(t, isLocalRequest(ctx))
	})
}

func TestClearSecureCookie(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "http://example.com/logout", http.NoBody)

	clearSecureCookie(ctx, "auth_token")

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "auth_token", cookies[0].Name)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.True(t, cookies[0].Secure)
}

func TestAuthHandler_Login_Errors(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.POST("/login", handler.Login)

	// 1. Invalid JSON
	req := httptest.NewRequest("POST", "/login", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 2. Invalid Credentials
	body := map[string]string{
		"email":    "nonexistent@example.com",
		"password": "wrong",
	}
	jsonBody, _ := json.Marshal(body)
	req = httptest.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Register(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)

	r := gin.New()
	r.POST("/register", handler.Register)

	body := map[string]string{
		"email":    "new@example.com",
		"password": "password123",
		"name":     "New User",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "new@example.com")
}

func TestAuthHandler_Register_Duplicate(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)
	db.Create(&models.User{UUID: uuid.NewString(), Email: "dup@example.com", Name: "Dup"})

	r := gin.New()
	r.POST("/register", handler.Register)

	body := map[string]string{
		"email":    "dup@example.com",
		"password": "password123",
		"name":     "Dup User",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuthHandler_Logout(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)

	r := gin.New()
	r.POST("/logout", handler.Logout)

	req := httptest.NewRequest("POST", "/logout", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Logged out")
	// Check cookie
	cookie := w.Result().Cookies()[0]
	assert.Equal(t, "auth_token", cookie.Name)
	assert.Equal(t, -1, cookie.MaxAge)
}

func TestAuthHandler_Me(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	// Create user that matches the middleware ID
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "me@example.com",
		Name:  "Me User",
		Role:  models.RoleAdmin,
	}
	db.Create(user)

	r := gin.New()
	// Simulate middleware
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("role", user.Role)
		c.Next()
	})
	r.GET("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(user.ID), resp["user_id"])
	assert.Equal(t, "admin", resp["role"])
	assert.Equal(t, "Me User", resp["name"])
	assert.Equal(t, "me@example.com", resp["email"])
}

func TestAuthHandler_Me_NotFound(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(999)) // Non-existent ID
		c.Next()
	})
	r.GET("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthHandler_ChangePassword(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	// Create user
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "change@example.com",
		Name:  "Change User",
	}
	_ = user.SetPassword("oldpassword")
	db.Create(user)

	r := gin.New()
	// Simulate middleware
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/change-password", handler.ChangePassword)

	body := map[string]string{
		"old_password": "oldpassword",
		"new_password": "newpassword123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Password updated successfully")

	// Verify password changed
	var updatedUser models.User
	db.First(&updatedUser, user.ID)
	assert.True(t, updatedUser.CheckPassword("newpassword123"))
}

func TestAuthHandler_ChangePassword_WrongOld(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)
	user := &models.User{UUID: uuid.NewString(), Email: "wrong@example.com"}
	_ = user.SetPassword("correct")
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/change-password", handler.ChangePassword)

	body := map[string]string{
		"old_password": "wrong",
		"new_password": "newpassword",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_ChangePassword_Errors(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.POST("/change-password", handler.ChangePassword)

	// 1. BindJSON error (checked before auth)
	req, _ := http.NewRequest("POST", "/change-password", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 2. Unauthorized (valid JSON but no user in context)
	body := map[string]string{
		"old_password": "oldpassword",
		"new_password": "newpassword123",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ = http.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// setupAuthHandlerWithDB creates an AuthHandler with DB access for forward auth tests
func setupAuthHandlerWithDB(t *testing.T) (*AuthHandler, *gorm.DB) {
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	_ = db.AutoMigrate(&models.User{}, &models.Setting{}, &models.ProxyHost{})

	cfg := config.Config{JWTSecret: "test-secret"}
	authService := services.NewAuthService(db, cfg)
	return NewAuthHandlerWithDB(authService, db), db
}

func TestNewAuthHandlerWithDB(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
	assert.NotNil(t, db)
}

func TestAuthHandler_Verify_NoCookie(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/login", w.Header().Get("X-Auth-Redirect"))
}

func TestAuthHandler_Verify_InvalidToken(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Verify_ValidToken(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create user
	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "test@example.com",
		Name:    "Test User",
		Role:    models.RoleUser,
		Enabled: true,
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	// Generate token
	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test@example.com", w.Header().Get("X-Forwarded-User"))
	assert.Equal(t, "user", w.Header().Get("X-Forwarded-Groups"))
}

func TestAuthHandler_Verify_BearerToken(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "bearer@example.com",
		Name:    "Bearer User",
		Role:    models.RoleAdmin,
		Enabled: true,
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "bearer@example.com", w.Header().Get("X-Forwarded-User"))
}

func TestAuthHandler_Verify_DisabledUser(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "disabled@example.com",
		Name:  "Disabled User",
		Role:  models.RoleUser,
	}
	_ = user.SetPassword("password123")
	db.Create(user)
	// Explicitly disable after creation to bypass GORM's default:true behavior
	db.Model(user).Update("enabled", false)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Verify_ForwardAuthDenied(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy host with forward auth enabled
	proxyHost := &models.ProxyHost{
		UUID:               uuid.NewString(),
		Name:               "Protected App",
		DomainNames:        "app.example.com",
		ForwardAuthEnabled: true,
		Enabled:            true,
	}
	db.Create(proxyHost)

	// Create user with deny_all permission
	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "denied@example.com",
		Name:           "Denied User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	req.Header.Set("X-Forwarded-Host", "app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthHandler_VerifyStatus_NotAuthenticated(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_VerifyStatus_InvalidToken(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_VerifyStatus_Authenticated(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "status@example.com",
		Name:    "Status User",
		Role:    models.RoleUser,
		Enabled: true,
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["authenticated"])
	userObj := resp["user"].(map[string]any)
	assert.Equal(t, "status@example.com", userObj["email"])
}

func TestAuthHandler_VerifyStatus_DisabledUser(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "disabled2@example.com",
		Name:  "Disabled User 2",
		Role:  models.RoleUser,
	}
	_ = user.SetPassword("password123")
	db.Create(user)
	// Explicitly disable after creation to bypass GORM's default:true behavior
	db.Model(user).Update("enabled", false)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_GetAccessibleHosts_Unauthorized(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_GetAccessibleHosts_AllowAll(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy hosts
	host1 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	host2 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 2", DomainNames: "host2.example.com", Enabled: true}
	db.Create(host1)
	db.Create(host2)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "allowall@example.com",
		Name:           "Allow All User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeAllowAll,
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]any)
	assert.Len(t, hosts, 2)
}

func TestAuthHandler_GetAccessibleHosts_DenyAll(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy hosts
	host1 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	db.Create(host1)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "denyall@example.com",
		Name:           "Deny All User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]any)
	assert.Len(t, hosts, 0)
}

func TestAuthHandler_GetAccessibleHosts_PermittedHosts(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy hosts
	host1 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	host2 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 2", DomainNames: "host2.example.com", Enabled: true}
	db.Create(host1)
	db.Create(host2)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "permitted@example.com",
		Name:           "Permitted User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
		PermittedHosts: []models.ProxyHost{*host1}, // Only host1
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]any)
	assert.Len(t, hosts, 1)
}

func TestAuthHandler_GetAccessibleHosts_UserNotFound(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(99999))
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthHandler_CheckHostAccess_Unauthorized(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_CheckHostAccess_InvalidHostID(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{UUID: uuid.NewString(), Email: "check@example.com", Enabled: true}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/invalid/access", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_CheckHostAccess_Allowed(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	host := &models.ProxyHost{UUID: uuid.NewString(), Name: "Test Host", DomainNames: "test.example.com", Enabled: true}
	db.Create(host)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "checkallowed@example.com",
		Enabled:        true,
		PermissionMode: models.PermissionModeAllowAll,
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["can_access"])
}

func TestAuthHandler_CheckHostAccess_Denied(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	host := &models.ProxyHost{UUID: uuid.NewString(), Name: "Protected Host", DomainNames: "protected.example.com", Enabled: true}
	db.Create(host)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "checkdenied@example.com",
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["can_access"])
}

func TestAuthHandler_Logout_InvalidatesBearerSession(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "logout-session@example.com",
		Name:    "Logout Session",
		Role:    models.RoleAdmin,
		Enabled: true,
	}
	_ = user.SetPassword("password123")
	require.NoError(t, db.Create(user).Error)

	r := gin.New()
	r.POST("/auth/login", handler.Login)
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware(handler.authService))
	protected.POST("/auth/logout", handler.Logout)
	protected.GET("/auth/me", handler.Me)

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "logout-session@example.com",
		"password": "password123",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	require.Equal(t, http.StatusOK, loginRes.Code)

	var loginPayload map[string]string
	require.NoError(t, json.Unmarshal(loginRes.Body.Bytes(), &loginPayload))
	token := loginPayload["token"]
	require.NotEmpty(t, token)

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", http.NoBody)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meRes := httptest.NewRecorder()
	r.ServeHTTP(meRes, meReq)
	require.Equal(t, http.StatusOK, meRes.Code)

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutRes := httptest.NewRecorder()
	r.ServeHTTP(logoutRes, logoutReq)
	require.Equal(t, http.StatusOK, logoutRes.Code)

	meAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/auth/me", http.NoBody)
	meAfterLogoutReq.Header.Set("Authorization", "Bearer "+token)
	meAfterLogoutRes := httptest.NewRecorder()
	r.ServeHTTP(meAfterLogoutRes, meAfterLogoutReq)
	require.Equal(t, http.StatusUnauthorized, meAfterLogoutRes.Code)
}

func TestAuthHandler_Me_RequiresUserContext(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)

	r := gin.New()
	r.GET("/me", handler.Me)

	req := httptest.NewRequest(http.MethodGet, "/me", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestAuthHandler_HelperFunctions(t *testing.T) {
	t.Parallel()

	t.Run("requestScheme prefers forwarded proto", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.Header.Set("X-Forwarded-Proto", "HTTPS, http")
		ctx.Request = req
		assert.Equal(t, "https", requestScheme(ctx))
	})

	t.Run("requestScheme uses tls when forwarded proto missing", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.TLS = &tls.ConnectionState{}
		ctx.Request = req
		assert.Equal(t, "https", requestScheme(ctx))
	})

	t.Run("requestScheme uses request url scheme when available", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.URL.Scheme = "HTTP"
		ctx.Request = req
		assert.Equal(t, "http", requestScheme(ctx))
	})

	t.Run("requestScheme defaults to http when request url is nil", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.URL = nil
		ctx.Request = req
		assert.Equal(t, "http", requestScheme(ctx))
	})

	t.Run("normalizeHost strips brackets and port", func(t *testing.T) {
		assert.Equal(t, "::1", normalizeHost("[::1]:443"))
		assert.Equal(t, "example.com", normalizeHost("example.com:8080"))
	})

	t.Run("originHost returns empty for invalid url", func(t *testing.T) {
		assert.Equal(t, "", originHost("://bad"))
		assert.Equal(t, "example.com", originHost("https://example.com/path"))
	})

	t.Run("isLocalOrPrivateHost and isLocalRequest", func(t *testing.T) {
		assert.True(t, isLocalOrPrivateHost("localhost"))
		assert.True(t, isLocalOrPrivateHost("127.0.0.1"))
		assert.False(t, isLocalOrPrivateHost("example.com"))

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://service.internal", http.NoBody)
		req.Host = "service.internal:8080"
		req.Header.Set("X-Forwarded-Host", "example.com, localhost:8080")
		ctx.Request = req
		assert.True(t, isLocalRequest(ctx))
	})
}

func TestAuthHandler_Refresh(t *testing.T) {
	t.Parallel()

	handler, db := setupAuthHandler(t)

	user := &models.User{UUID: uuid.NewString(), Email: "refresh@example.com", Name: "Refresh User", Role: models.RoleUser, Enabled: true}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.Create(user).Error)

	r := gin.New()
	r.POST("/refresh", func(c *gin.Context) {
		c.Set("userID", user.ID)
		handler.Refresh(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "token")
	cookies := res.Result().Cookies()
	assert.NotEmpty(t, cookies)
}

func TestAuthHandler_Refresh_Unauthorized(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.POST("/refresh", handler.Refresh)

	req := httptest.NewRequest(http.MethodPost, "/refresh", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestAuthHandler_Register_BadRequest(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.POST("/register", handler.Register)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestAuthHandler_Logout_InvalidateSessionsFailure(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(999999))
		c.Next()
	})
	r.POST("/logout", handler.Logout)

	req := httptest.NewRequest(http.MethodPost, "/logout", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.Contains(t, res.Body.String(), "Failed to invalidate session")
}

func TestAuthHandler_Verify_UsesOriginalHostFallback(t *testing.T) {
	t.Parallel()

	handler, db := setupAuthHandlerWithDB(t)

	proxyHost := &models.ProxyHost{
		UUID:               uuid.NewString(),
		Name:               "Original Host App",
		DomainNames:        "original-host.example.com",
		ForwardAuthEnabled: true,
		Enabled:            true,
	}
	require.NoError(t, db.Create(proxyHost).Error)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "originalhost@example.com",
		Name:           "Original Host User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeAllowAll,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.Create(user).Error)

	token, err := handler.authService.GenerateToken(user)
	require.NoError(t, err)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest(http.MethodGet, "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	req.Header.Set("X-Original-Host", "original-host.example.com")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "originalhost@example.com", res.Header().Get("X-Forwarded-User"))
}

func TestAuthHandler_GetAccessibleHosts_DatabaseUnavailable(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest(http.MethodGet, "/hosts", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.Contains(t, res.Body.String(), "Database not available")
}

func TestAuthHandler_CheckHostAccess_DatabaseUnavailable(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest(http.MethodGet, "/hosts/1/access", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.Contains(t, res.Body.String(), "Database not available")
}

func TestAuthHandler_CheckHostAccess_UserNotFound(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(999999))
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest(http.MethodGet, "/hosts/1/access", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusNotFound, res.Code)
	assert.Contains(t, res.Body.String(), "User not found")
}
