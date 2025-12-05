package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUserHandler(t *testing.T) (*UserHandler, *gorm.DB) {
	// Use unique DB for each test to avoid pollution
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&models.User{}, &models.Setting{})
	return NewUserHandler(db), db
}

func TestUserHandler_GetSetupStatus(t *testing.T) {
	handler, db := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/setup", handler.GetSetupStatus)

	// No users -> setup required
	req, _ := http.NewRequest("GET", "/setup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"setupRequired\":true")

	// Create user -> setup not required
	db.Create(&models.User{Email: "test@example.com"})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"setupRequired\":false")
}

func TestUserHandler_Setup(t *testing.T) {
	handler, _ := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/setup", handler.Setup)

	// 1. Invalid JSON (Before setup is done)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/setup", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 2. Valid Setup
	body := map[string]string{
		"name":     "Admin",
		"email":    "admin@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ = http.NewRequest("POST", "/setup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "Setup completed successfully")

	// 3. Try again -> should fail (already setup)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/setup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_Setup_DBError(t *testing.T) {
	// Can't easily mock DB error with sqlite memory unless we close it or something.
	// But we can try to insert duplicate email if we had a unique constraint and pre-seeded data,
	// but Setup checks if ANY user exists first.
	// So if we have a user, it returns Forbidden.
	// If we don't, it tries to create.
	// If we want Create to fail, maybe invalid data that passes binding but fails DB constraint?
	// User model has validation?
	// Let's try empty password if allowed by binding but rejected by DB?
	// Or very long string?
}

func TestUserHandler_RegenerateAPIKey(t *testing.T) {
	handler, db := setupUserHandler(t)

	user := &models.User{Email: "api@example.com"}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/api-key", handler.RegenerateAPIKey)

	req, _ := http.NewRequest("POST", "/api-key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp["api_key"])

	// Verify DB
	var updatedUser models.User
	db.First(&updatedUser, user.ID)
	assert.Equal(t, resp["api_key"], updatedUser.APIKey)
}

func TestUserHandler_GetProfile(t *testing.T) {
	handler, db := setupUserHandler(t)

	user := &models.User{
		Email:  "profile@example.com",
		Name:   "Profile User",
		APIKey: "existing-key",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/profile", handler.GetProfile)

	req, _ := http.NewRequest("GET", "/profile", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.User
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, user.Email, resp.Email)
	assert.Equal(t, user.APIKey, resp.APIKey)
}

func TestUserHandler_RegisterRoutes(t *testing.T) {
	handler, _ := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	handler.RegisterRoutes(api)

	routes := r.Routes()
	expectedRoutes := map[string]string{
		"/api/setup":              "GET,POST",
		"/api/profile":            "GET",
		"/api/regenerate-api-key": "POST",
	}

	for path := range expectedRoutes {
		found := false
		for _, route := range routes {
			if route.Path == path {
				found = true
				break
			}
		}
		assert.True(t, found, "Route %s not found", path)
	}
}

func TestUserHandler_Errors(t *testing.T) {
	handler, db := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Middleware to simulate missing userID
	r.GET("/profile-no-auth", func(c *gin.Context) {
		// No userID set
		handler.GetProfile(c)
	})
	r.POST("/api-key-no-auth", func(c *gin.Context) {
		// No userID set
		handler.RegenerateAPIKey(c)
	})

	// Middleware to simulate non-existent user
	r.GET("/profile-not-found", func(c *gin.Context) {
		c.Set("userID", uint(99999))
		handler.GetProfile(c)
	})
	r.POST("/api-key-not-found", func(c *gin.Context) {
		c.Set("userID", uint(99999))
		handler.RegenerateAPIKey(c)
	})

	// Test Unauthorized
	req, _ := http.NewRequest("GET", "/profile-no-auth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	req, _ = http.NewRequest("POST", "/api-key-no-auth", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test Not Found (GetProfile)
	req, _ = http.NewRequest("GET", "/profile-not-found", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test DB Error (RegenerateAPIKey) - Hard to mock DB error on update with sqlite memory,
	// but we can try to update a non-existent user which GORM Update might not treat as error unless we check RowsAffected.
	// The handler code: if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Update("api_key", apiKey).Error; err != nil
	// Update on non-existent record usually returns nil error in GORM unless configured otherwise.
	// However, let's see if we can force an error by closing DB? No, shared DB.
	// We can drop the table?
	db.Migrator().DropTable(&models.User{})
	req, _ = http.NewRequest("POST", "/api-key-not-found", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// If table missing, Update should fail
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUserHandler_UpdateProfile(t *testing.T) {
	handler, db := setupUserHandler(t)

	// Create user
	user := &models.User{
		UUID:   uuid.NewString(),
		Email:  "test@example.com",
		Name:   "Test User",
		APIKey: uuid.NewString(),
	}
	user.SetPassword("password123")
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.PUT("/profile", handler.UpdateProfile)

	// 1. Success - Name only
	t.Run("Success Name Only", func(t *testing.T) {
		body := map[string]string{
			"name":  "Updated Name",
			"email": "test@example.com",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var updatedUser models.User
		db.First(&updatedUser, user.ID)
		assert.Equal(t, "Updated Name", updatedUser.Name)
	})

	// 2. Success - Email change with password
	t.Run("Success Email Change", func(t *testing.T) {
		body := map[string]string{
			"name":             "Updated Name",
			"email":            "newemail@example.com",
			"current_password": "password123",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var updatedUser models.User
		db.First(&updatedUser, user.ID)
		assert.Equal(t, "newemail@example.com", updatedUser.Email)
	})

	// 3. Fail - Email change without password
	t.Run("Fail Email Change No Password", func(t *testing.T) {
		// Reset email
		db.Model(user).Update("email", "test@example.com")

		body := map[string]string{
			"name":  "Updated Name",
			"email": "another@example.com",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// 4. Fail - Email change wrong password
	t.Run("Fail Email Change Wrong Password", func(t *testing.T) {
		body := map[string]string{
			"name":             "Updated Name",
			"email":            "another@example.com",
			"current_password": "wrongpassword",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// 5. Fail - Email already in use
	t.Run("Fail Email In Use", func(t *testing.T) {
		// Create another user
		otherUser := &models.User{
			UUID:   uuid.NewString(),
			Email:  "other@example.com",
			Name:   "Other User",
			APIKey: uuid.NewString(),
		}
		db.Create(otherUser)

		body := map[string]string{
			"name":  "Updated Name",
			"email": "other@example.com",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestUserHandler_UpdateProfile_Errors(t *testing.T) {
	handler, _ := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// 1. Unauthorized (no userID)
	r.PUT("/profile-no-auth", handler.UpdateProfile)
	req, _ := http.NewRequest("PUT", "/profile-no-auth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Middleware for subsequent tests
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(999)) // Non-existent ID
		c.Next()
	})
	r.PUT("/profile", handler.UpdateProfile)

	// 2. BindJSON error
	req, _ = http.NewRequest("PUT", "/profile", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 3. User not found
	body := map[string]string{"name": "New Name", "email": "new@example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ = http.NewRequest("PUT", "/profile", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============= User Management Tests (Admin functions) =============

func setupUserHandlerWithProxyHosts(t *testing.T) (*UserHandler, *gorm.DB) {
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&models.User{}, &models.Setting{}, &models.ProxyHost{})
	return NewUserHandler(db), db
}

func TestUserHandler_ListUsers_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.GET("/users", handler.ListUsers)

	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_ListUsers_Admin(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create users with unique API keys
	user1 := &models.User{UUID: uuid.NewString(), Email: "user1@example.com", Name: "User 1", APIKey: uuid.NewString()}
	user2 := &models.User{UUID: uuid.NewString(), Email: "user2@example.com", Name: "User 2", APIKey: uuid.NewString()}
	db.Create(user1)
	db.Create(user2)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.GET("/users", handler.ListUsers)

	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var users []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &users)
	assert.Len(t, users, 2)
}

func TestUserHandler_CreateUser_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	body := map[string]interface{}{
		"email":    "new@example.com",
		"name":     "New User",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_CreateUser_Admin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	body := map[string]interface{}{
		"email":    "newuser@example.com",
		"name":     "New User",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestUserHandler_CreateUser_InvalidJSON(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	req := httptest.NewRequest("POST", "/users", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_CreateUser_DuplicateEmail(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	existing := &models.User{UUID: uuid.NewString(), Email: "existing@example.com", Name: "Existing"}
	db.Create(existing)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	body := map[string]interface{}{
		"email":    "existing@example.com",
		"name":     "New User",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUserHandler_CreateUser_WithPermittedHosts(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	host := &models.ProxyHost{Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	db.Create(host)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	body := map[string]interface{}{
		"email":           "withhosts@example.com",
		"name":            "User With Hosts",
		"password":        "password123",
		"permission_mode": "deny_all",
		"permitted_hosts": []uint{host.ID},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestUserHandler_GetUser_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.GET("/users/:id", handler.GetUser)

	req := httptest.NewRequest("GET", "/users/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_GetUser_InvalidID(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.GET("/users/:id", handler.GetUser)

	req := httptest.NewRequest("GET", "/users/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_GetUser_NotFound(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.GET("/users/:id", handler.GetUser)

	req := httptest.NewRequest("GET", "/users/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_GetUser_Success(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	user := &models.User{UUID: uuid.NewString(), Email: "getuser@example.com", Name: "Get User"}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.GET("/users/:id", handler.GetUser)

	req := httptest.NewRequest("GET", "/users/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandler_UpdateUser_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.PUT("/users/:id", handler.UpdateUser)

	body := map[string]interface{}{"name": "Updated"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_UpdateUser_InvalidID(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id", handler.UpdateUser)

	body := map[string]interface{}{"name": "Updated"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/invalid", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_UpdateUser_InvalidJSON(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create user first
	user := &models.User{UUID: uuid.NewString(), Email: "toupdate@example.com", Name: "To Update", APIKey: uuid.NewString()}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id", handler.UpdateUser)

	req := httptest.NewRequest("PUT", "/users/1", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_UpdateUser_NotFound(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id", handler.UpdateUser)

	body := map[string]interface{}{"name": "Updated"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_UpdateUser_Success(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	user := &models.User{UUID: uuid.NewString(), Email: "update@example.com", Name: "Original", Role: "user"}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id", handler.UpdateUser)

	body := map[string]interface{}{
		"name":    "Updated Name",
		"enabled": true,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandler_DeleteUser_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.DELETE("/users/:id", handler.DeleteUser)

	req := httptest.NewRequest("DELETE", "/users/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_DeleteUser_InvalidID(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.DELETE("/users/:id", handler.DeleteUser)

	req := httptest.NewRequest("DELETE", "/users/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_DeleteUser_NotFound(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1)) // Current user ID (different from target)
		c.Next()
	})
	r.DELETE("/users/:id", handler.DeleteUser)

	req := httptest.NewRequest("DELETE", "/users/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_DeleteUser_Success(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	user := &models.User{UUID: uuid.NewString(), Email: "delete@example.com", Name: "Delete Me"}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(999)) // Different user
		c.Next()
	})
	r.DELETE("/users/:id", handler.DeleteUser)

	req := httptest.NewRequest("DELETE", "/users/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandler_DeleteUser_CannotDeleteSelf(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	user := &models.User{UUID: uuid.NewString(), Email: "self@example.com", Name: "Self"}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", user.ID) // Same user
		c.Next()
	})
	r.DELETE("/users/:id", handler.DeleteUser)

	req := httptest.NewRequest("DELETE", "/users/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_UpdateUserPermissions_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.PUT("/users/:id/permissions", handler.UpdateUserPermissions)

	body := map[string]interface{}{"permission_mode": "allow_all"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/1/permissions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_UpdateUserPermissions_InvalidID(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id/permissions", handler.UpdateUserPermissions)

	body := map[string]interface{}{"permission_mode": "allow_all"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/invalid/permissions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_UpdateUserPermissions_InvalidJSON(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create a user first
	user := &models.User{
		UUID:    uuid.NewString(),
		APIKey:  uuid.NewString(),
		Email:   "perms-invalid@example.com",
		Name:    "Perms Invalid Test",
		Role:    "user",
		Enabled: true,
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id/permissions", handler.UpdateUserPermissions)

	req := httptest.NewRequest("PUT", "/users/"+strconv.FormatUint(uint64(user.ID), 10)+"/permissions", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_UpdateUserPermissions_NotFound(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id/permissions", handler.UpdateUserPermissions)

	body := map[string]interface{}{"permission_mode": "allow_all"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/999/permissions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_UpdateUserPermissions_Success(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	host := &models.ProxyHost{Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	db.Create(host)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "perms@example.com",
		Name:           "Perms User",
		PermissionMode: models.PermissionModeAllowAll,
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id/permissions", handler.UpdateUserPermissions)

	body := map[string]interface{}{
		"permission_mode": "deny_all",
		"permitted_hosts": []uint{host.ID},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/1/permissions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandler_ValidateInvite_MissingToken(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invite/validate", handler.ValidateInvite)

	req := httptest.NewRequest("GET", "/invite/validate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_ValidateInvite_InvalidToken(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invite/validate", handler.ValidateInvite)

	req := httptest.NewRequest("GET", "/invite/validate?token=invalidtoken", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_ValidateInvite_ExpiredToken(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	expiredTime := time.Now().Add(-24 * time.Hour) // Expired yesterday
	user := &models.User{
		UUID:          uuid.NewString(),
		Email:         "expired@example.com",
		Name:          "Expired Invite",
		InviteToken:   "expiredtoken123",
		InviteExpires: &expiredTime,
		InviteStatus:  "pending",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invite/validate", handler.ValidateInvite)

	req := httptest.NewRequest("GET", "/invite/validate?token=expiredtoken123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGone, w.Code)
}

func TestUserHandler_ValidateInvite_AlreadyAccepted(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	expiresAt := time.Now().Add(24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		Email:         "accepted@example.com",
		Name:          "Accepted Invite",
		InviteToken:   "acceptedtoken123",
		InviteExpires: &expiresAt,
		InviteStatus:  "accepted",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invite/validate", handler.ValidateInvite)

	req := httptest.NewRequest("GET", "/invite/validate?token=acceptedtoken123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUserHandler_ValidateInvite_Success(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	expiresAt := time.Now().Add(24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		Email:         "valid@example.com",
		Name:          "Valid Invite",
		InviteToken:   "validtoken123",
		InviteExpires: &expiresAt,
		InviteStatus:  "pending",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invite/validate", handler.ValidateInvite)

	req := httptest.NewRequest("GET", "/invite/validate?token=validtoken123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "valid@example.com", resp["email"])
}

func TestUserHandler_AcceptInvite_InvalidJSON(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/invite/accept", handler.AcceptInvite)

	req := httptest.NewRequest("POST", "/invite/accept", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_AcceptInvite_InvalidToken(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/invite/accept", handler.AcceptInvite)

	body := map[string]string{
		"token":    "invalidtoken",
		"name":     "Test User",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/invite/accept", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_AcceptInvite_Success(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	expiresAt := time.Now().Add(24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		Email:         "accept@example.com",
		Name:          "Accept User",
		InviteToken:   "accepttoken123",
		InviteExpires: &expiresAt,
		InviteStatus:  "pending",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/invite/accept", handler.AcceptInvite)

	body := map[string]string{
		"token":    "accepttoken123",
		"password": "newpassword123",
		"name":     "Accepted User",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/invite/accept", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify user was updated
	var updated models.User
	db.First(&updated, user.ID)
	assert.Equal(t, "accepted", updated.InviteStatus)
	assert.True(t, updated.Enabled)
}

func TestGenerateSecureToken(t *testing.T) {
	token, err := generateSecureToken(32)
	assert.NoError(t, err)
	assert.Len(t, token, 64) // 32 bytes = 64 hex chars
	assert.Regexp(t, "^[a-f0-9]+$", token)

	// Ensure uniqueness
	token2, err := generateSecureToken(32)
	assert.NoError(t, err)
	assert.NotEqual(t, token, token2)
}

func TestUserHandler_InviteUser_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)

	body := map[string]string{"email": "invitee@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_InviteUser_InvalidJSON(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)

	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_InviteUser_DuplicateEmail(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create existing user
	existingUser := &models.User{
		UUID:   uuid.NewString(),
		APIKey: uuid.NewString(),
		Email:  "existing@example.com",
	}
	db.Create(existingUser)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)

	body := map[string]string{"email": "existing@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUserHandler_InviteUser_Success(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create admin user
	admin := &models.User{
		UUID:   uuid.NewString(),
		APIKey: uuid.NewString(),
		Email:  "admin@example.com",
		Role:   "admin",
	}
	db.Create(admin)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", admin.ID)
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)

	body := map[string]interface{}{
		"email": "newinvite@example.com",
		"role":  "user",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp["invite_token"])
	// email_sent is false because no SMTP is configured
	assert.Equal(t, false, resp["email_sent"].(bool))

	// Verify user was created
	var user models.User
	db.Where("email = ?", "newinvite@example.com").First(&user)
	assert.Equal(t, "pending", user.InviteStatus)
	assert.False(t, user.Enabled)
}

func TestUserHandler_InviteUser_WithPermittedHosts(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create admin user
	admin := &models.User{
		UUID:   uuid.NewString(),
		APIKey: uuid.NewString(),
		Email:  "admin-perm@example.com",
		Role:   "admin",
	}
	db.Create(admin)

	// Create proxy host
	host := &models.ProxyHost{
		UUID:        uuid.NewString(),
		Name:        "Test Host",
		DomainNames: "test.example.com",
	}
	db.Create(host)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", admin.ID)
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)

	body := map[string]interface{}{
		"email":           "invitee-perms@example.com",
		"permission_mode": "deny_all",
		"permitted_hosts": []uint{host.ID},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify user has permitted hosts
	var user models.User
	db.Preload("PermittedHosts").Where("email = ?", "invitee-perms@example.com").First(&user)
	assert.Len(t, user.PermittedHosts, 1)
	assert.Equal(t, models.PermissionModeDenyAll, user.PermissionMode)
}

func TestGetBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test with X-Forwarded-Proto header
	r := gin.New()
	r.GET("/test", func(c *gin.Context) {
		url := getBaseURL(c)
		c.String(200, url)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "https://example.com", w.Body.String())
}

func TestGetAppName(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:appname?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&models.Setting{})

	// Test default
	name := getAppName(db)
	assert.Equal(t, "Charon", name)

	// Test with custom setting
	db.Create(&models.Setting{Key: "app_name", Value: "CustomApp"})
	name = getAppName(db)
	assert.Equal(t, "CustomApp", name)
}

func TestUserHandler_AcceptInvite_ExpiredToken(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create user with expired invite
	expired := time.Now().Add(-24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		APIKey:        uuid.NewString(),
		Email:         "expired-invite@example.com",
		InviteToken:   "expiredtoken123",
		InviteExpires: &expired,
		InviteStatus:  "pending",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/invite/accept", handler.AcceptInvite)

	body := map[string]string{
		"token":    "expiredtoken123",
		"name":     "Expired User",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/invite/accept", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGone, w.Code)
}

func TestUserHandler_AcceptInvite_AlreadyAccepted(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	expires := time.Now().Add(24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		APIKey:        uuid.NewString(),
		Email:         "accepted-already@example.com",
		InviteToken:   "acceptedtoken123",
		InviteExpires: &expires,
		InviteStatus:  "accepted",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/invite/accept", handler.AcceptInvite)

	body := map[string]string{
		"token":    "acceptedtoken123",
		"name":     "Already Accepted",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/invite/accept", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}
