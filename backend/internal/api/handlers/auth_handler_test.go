package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	db.AutoMigrate(&models.User{}, &models.Setting{})

	cfg := config.Config{JWTSecret: "test-secret"}
	authService := services.NewAuthService(db, cfg)
	return NewAuthHandler(authService), db
}

func TestAuthHandler_Login(t *testing.T) {
	handler, db := setupAuthHandler(t)

	// Create user
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	user.SetPassword("password123")
	db.Create(user)

	gin.SetMode(gin.TestMode)
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

func TestAuthHandler_Login_Errors(t *testing.T) {
	handler, _ := setupAuthHandler(t)
	gin.SetMode(gin.TestMode)
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
	handler, _ := setupAuthHandler(t)

	gin.SetMode(gin.TestMode)
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
	handler, db := setupAuthHandler(t)
	db.Create(&models.User{UUID: uuid.NewString(), Email: "dup@example.com", Name: "Dup"})

	gin.SetMode(gin.TestMode)
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
	handler, _ := setupAuthHandler(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/logout", handler.Logout)

	req := httptest.NewRequest("POST", "/logout", nil)
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
	handler, db := setupAuthHandler(t)

	// Create user that matches the middleware ID
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "me@example.com",
		Name:  "Me User",
		Role:  "admin",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Simulate middleware
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("role", user.Role)
		c.Next()
	})
	r.GET("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(user.ID), resp["user_id"])
	assert.Equal(t, "admin", resp["role"])
	assert.Equal(t, "Me User", resp["name"])
	assert.Equal(t, "me@example.com", resp["email"])
}

func TestAuthHandler_Me_NotFound(t *testing.T) {
	handler, _ := setupAuthHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(999)) // Non-existent ID
		c.Next()
	})
	r.GET("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthHandler_ChangePassword(t *testing.T) {
	handler, db := setupAuthHandler(t)

	// Create user
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "change@example.com",
		Name:  "Change User",
	}
	user.SetPassword("oldpassword")
	db.Create(user)

	gin.SetMode(gin.TestMode)
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
	handler, db := setupAuthHandler(t)
	user := &models.User{UUID: uuid.NewString(), Email: "wrong@example.com"}
	user.SetPassword("correct")
	db.Create(user)

	gin.SetMode(gin.TestMode)
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
	handler, _ := setupAuthHandler(t)
	gin.SetMode(gin.TestMode)
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
	db.AutoMigrate(&models.User{}, &models.Setting{}, &models.ProxyHost{})

	cfg := config.Config{JWTSecret: "test-secret"}
	authService := services.NewAuthService(db, cfg)
	return NewAuthHandlerWithDB(authService, db), db
}

func TestNewAuthHandlerWithDB(t *testing.T) {
	handler, db := setupAuthHandlerWithDB(t)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
	assert.NotNil(t, db)
}

func TestAuthHandler_Verify_NoCookie(t *testing.T) {
	handler, _ := setupAuthHandlerWithDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/login", w.Header().Get("X-Auth-Redirect"))
}

func TestAuthHandler_Verify_InvalidToken(t *testing.T) {
	handler, _ := setupAuthHandlerWithDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Verify_ValidToken(t *testing.T) {
	handler, db := setupAuthHandlerWithDB(t)

	// Create user
	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "test@example.com",
		Name:    "Test User",
		Role:    "user",
		Enabled: true,
	}
	user.SetPassword("password123")
	db.Create(user)

	// Generate token
	token, _ := handler.authService.GenerateToken(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test@example.com", w.Header().Get("X-Forwarded-User"))
	assert.Equal(t, "user", w.Header().Get("X-Forwarded-Groups"))
}

func TestAuthHandler_Verify_BearerToken(t *testing.T) {
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "bearer@example.com",
		Name:    "Bearer User",
		Role:    "admin",
		Enabled: true,
	}
	user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "bearer@example.com", w.Header().Get("X-Forwarded-User"))
}

func TestAuthHandler_Verify_DisabledUser(t *testing.T) {
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "disabled@example.com",
		Name:  "Disabled User",
		Role:  "user",
	}
	user.SetPassword("password123")
	db.Create(user)
	// Explicitly disable after creation to bypass GORM's default:true behavior
	db.Model(user).Update("enabled", false)

	token, _ := handler.authService.GenerateToken(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Verify_ForwardAuthDenied(t *testing.T) {
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
		Role:           "user",
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
	}
	user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	req.Header.Set("X-Forwarded-Host", "app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthHandler_VerifyStatus_NotAuthenticated(t *testing.T) {
	handler, _ := setupAuthHandlerWithDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_VerifyStatus_InvalidToken(t *testing.T) {
	handler, _ := setupAuthHandlerWithDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_VerifyStatus_Authenticated(t *testing.T) {
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "status@example.com",
		Name:    "Status User",
		Role:    "user",
		Enabled: true,
	}
	user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["authenticated"])
	userObj := resp["user"].(map[string]interface{})
	assert.Equal(t, "status@example.com", userObj["email"])
}

func TestAuthHandler_VerifyStatus_DisabledUser(t *testing.T) {
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "disabled2@example.com",
		Name:  "Disabled User 2",
		Role:  "user",
	}
	user.SetPassword("password123")
	db.Create(user)
	// Explicitly disable after creation to bypass GORM's default:true behavior
	db.Model(user).Update("enabled", false)

	token, _ := handler.authService.GenerateToken(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_GetAccessibleHosts_Unauthorized(t *testing.T) {
	handler, _ := setupAuthHandlerWithDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_GetAccessibleHosts_AllowAll(t *testing.T) {
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
		Role:           "user",
		Enabled:        true,
		PermissionMode: models.PermissionModeAllowAll,
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]interface{})
	assert.Len(t, hosts, 2)
}

func TestAuthHandler_GetAccessibleHosts_DenyAll(t *testing.T) {
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy hosts
	host1 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	db.Create(host1)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "denyall@example.com",
		Name:           "Deny All User",
		Role:           "user",
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]interface{})
	assert.Len(t, hosts, 0)
}

func TestAuthHandler_GetAccessibleHosts_PermittedHosts(t *testing.T) {
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
		Role:           "user",
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
		PermittedHosts: []models.ProxyHost{*host1}, // Only host1
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]interface{})
	assert.Len(t, hosts, 1)
}

func TestAuthHandler_GetAccessibleHosts_UserNotFound(t *testing.T) {
	handler, _ := setupAuthHandlerWithDB(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(99999))
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthHandler_CheckHostAccess_Unauthorized(t *testing.T) {
	handler, _ := setupAuthHandlerWithDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_CheckHostAccess_InvalidHostID(t *testing.T) {
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{UUID: uuid.NewString(), Email: "check@example.com", Enabled: true}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/invalid/access", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_CheckHostAccess_Allowed(t *testing.T) {
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

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["can_access"])
}

func TestAuthHandler_CheckHostAccess_Denied(t *testing.T) {
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

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["can_access"])
}
