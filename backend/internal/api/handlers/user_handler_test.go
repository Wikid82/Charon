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
	"github.com/Wikid82/charon/backend/internal/services"
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
	_ = db.AutoMigrate(&models.User{}, &models.Setting{})
	return NewUserHandler(db), db
}

func TestUserHandler_GetSetupStatus(t *testing.T) {
	handler, db := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/setup", handler.GetSetupStatus)

	// No users -> setup required
	req, _ := http.NewRequest("GET", "/setup", http.NoBody)
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

	req, _ := http.NewRequest("POST", "/api-key", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
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

	req, _ := http.NewRequest("GET", "/profile", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.User
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
	assert.Equal(t, user.Email, resp.Email)
	// APIKey is not exposed in JSON (json:"-" tag), so it should be empty in response
	assert.Empty(t, resp.APIKey, "APIKey should not be exposed in profile response")
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
	req, _ := http.NewRequest("GET", "/profile-no-auth", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	req, _ = http.NewRequest("POST", "/api-key-no-auth", http.NoBody)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test Not Found (GetProfile)
	req, _ = http.NewRequest("GET", "/profile-not-found", http.NoBody)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test DB Error (RegenerateAPIKey) - Hard to mock DB error on update with sqlite memory,
	// but we can try to update a non-existent user which GORM Update might not treat as error unless we check RowsAffected.
	// The handler code: if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Update("api_key", apiKey).Error; err != nil
	// Update on non-existent record usually returns nil error in GORM unless configured otherwise.
	// However, let's see if we can force an error by closing DB? No, shared DB.
	// We can drop the table?
	_ = db.Migrator().DropTable(&models.User{})
	req, _ = http.NewRequest("POST", "/api-key-not-found", http.NoBody)
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
	_ = user.SetPassword("password123")
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
	req, _ := http.NewRequest("PUT", "/profile-no-auth", http.NoBody)
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
	_ = db.AutoMigrate(&models.User{}, &models.Setting{}, &models.ProxyHost{})
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

	req := httptest.NewRequest("GET", "/users", http.NoBody)
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

	req := httptest.NewRequest("GET", "/users", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var users []map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &users)
	require.NoError(t, err, "Failed to unmarshal response")
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

	body := map[string]any{
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

	body := map[string]any{
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

	body := map[string]any{
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

	body := map[string]any{
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

	req := httptest.NewRequest("GET", "/users/1", http.NoBody)
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

	req := httptest.NewRequest("GET", "/users/invalid", http.NoBody)
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

	req := httptest.NewRequest("GET", "/users/999", http.NoBody)
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

	req := httptest.NewRequest("GET", "/users/1", http.NoBody)
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

	body := map[string]any{"name": "Updated"}
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

	body := map[string]any{"name": "Updated"}
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

	body := map[string]any{"name": "Updated"}
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

	body := map[string]any{
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

func TestUserHandler_UpdateUser_PasswordReset(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	user := &models.User{UUID: uuid.NewString(), Email: "reset@example.com", Name: "Reset User", Role: "user"}
	require.NoError(t, user.SetPassword("oldpassword123"))
	lockUntil := time.Now().Add(10 * time.Minute)
	user.FailedLoginAttempts = 4
	user.LockedUntil = &lockUntil
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id", handler.UpdateUser)

	body := map[string]any{
		"password": "newpassword123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updated models.User
	db.First(&updated, user.ID)
	assert.True(t, updated.CheckPassword("newpassword123"))
	assert.False(t, updated.CheckPassword("oldpassword123"))
	assert.Equal(t, 0, updated.FailedLoginAttempts)
	assert.Nil(t, updated.LockedUntil)
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

	req := httptest.NewRequest("DELETE", "/users/1", http.NoBody)
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

	req := httptest.NewRequest("DELETE", "/users/invalid", http.NoBody)
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

	req := httptest.NewRequest("DELETE", "/users/999", http.NoBody)
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

	req := httptest.NewRequest("DELETE", "/users/1", http.NoBody)
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

	req := httptest.NewRequest("DELETE", "/users/1", http.NoBody)
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

	body := map[string]any{"permission_mode": "allow_all"}
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

	body := map[string]any{"permission_mode": "allow_all"}
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

	body := map[string]any{"permission_mode": "allow_all"}
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

	body := map[string]any{
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

	req := httptest.NewRequest("GET", "/invite/validate", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_ValidateInvite_InvalidToken(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invite/validate", handler.ValidateInvite)

	req := httptest.NewRequest("GET", "/invite/validate?token=invalidtoken", http.NoBody)
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

	req := httptest.NewRequest("GET", "/invite/validate?token=expiredtoken123", http.NoBody)
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

	req := httptest.NewRequest("GET", "/invite/validate?token=acceptedtoken123", http.NoBody)
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

	req := httptest.NewRequest("GET", "/invite/validate?token=validtoken123", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
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

	body := map[string]any{
		"email": "newinvite@example.com",
		"role":  "user",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
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

	body := map[string]any{
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

func TestUserHandler_InviteUser_WithSMTPConfigured(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create admin user
	admin := &models.User{
		UUID:   uuid.NewString(),
		APIKey: uuid.NewString(),
		Email:  "admin-smtp@example.com",
		Role:   "admin",
	}
	db.Create(admin)

	// Configure SMTP settings to trigger email code path and getAppName call
	smtpSettings := []models.Setting{
		{Key: "smtp_host", Value: "smtp.example.com", Type: "string", Category: "smtp"},
		{Key: "smtp_port", Value: "587", Type: "integer", Category: "smtp"},
		{Key: "smtp_username", Value: "user@example.com", Type: "string", Category: "smtp"},
		{Key: "smtp_password", Value: "password", Type: "string", Category: "smtp"},
		{Key: "smtp_from_address", Value: "noreply@example.com", Type: "string", Category: "smtp"},
		{Key: "app_name", Value: "TestApp", Type: "string", Category: "app"},
	}
	for _, setting := range smtpSettings {
		db.Create(&setting)
	}

	// Reinitialize mail service to pick up new settings
	handler.MailService = services.NewMailService(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", admin.ID)
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)

	body := map[string]any{
		"email": "smtp-test@example.com",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify user was created
	var user models.User
	db.Where("email = ?", "smtp-test@example.com").First(&user)
	assert.Equal(t, "pending", user.InviteStatus)
	assert.False(t, user.Enabled)

	// Note: email_sent will be false because we can't actually send email in tests,
	// but the code path through IsConfigured() and getAppName() is still executed
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
	assert.NotEmpty(t, resp["invite_token"])
}

func TestUserHandler_InviteUser_WithSMTPConfigured_DefaultAppName(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create admin user
	admin := &models.User{
		UUID:   uuid.NewString(),
		APIKey: uuid.NewString(),
		Email:  "admin-smtp-default@example.com",
		Role:   "admin",
	}
	db.Create(admin)

	// Configure SMTP settings WITHOUT app_name to trigger default "Charon" path
	smtpSettings := []models.Setting{
		{Key: "smtp_host", Value: "smtp.example.com", Type: "string", Category: "smtp"},
		{Key: "smtp_port", Value: "587", Type: "integer", Category: "smtp"},
		{Key: "smtp_username", Value: "user@example.com", Type: "string", Category: "smtp"},
		{Key: "smtp_password", Value: "password", Type: "string", Category: "smtp"},
		{Key: "smtp_from_address", Value: "noreply@example.com", Type: "string", Category: "smtp"},
		// Intentionally NOT setting app_name to test default path
	}
	for _, setting := range smtpSettings {
		db.Create(&setting)
	}

	// Reinitialize mail service to pick up new settings
	handler.MailService = services.NewMailService(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", admin.ID)
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)

	body := map[string]any{
		"email": "smtp-test-default@example.com",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify user was created
	var user models.User
	db.Where("email = ?", "smtp-test-default@example.com").First(&user)
	assert.Equal(t, "pending", user.InviteStatus)
	assert.False(t, user.Enabled)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
	assert.NotEmpty(t, resp["invite_token"])
}

// Note: TestGetBaseURL and TestGetAppName have been removed as these internal helper
// functions have been refactored into the utils package. URL functionality is tested
// via integration tests and the utils package should have its own unit tests.

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

// ============= Priority 1: Zero Coverage Functions =============

// PreviewInviteURL Tests
func TestUserHandler_PreviewInviteURL_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/users/preview-invite-url", handler.PreviewInviteURL)

	body := map[string]string{"email": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/preview-invite-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Admin access required")
}

func TestUserHandler_PreviewInviteURL_InvalidJSON(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/preview-invite-url", handler.PreviewInviteURL)

	req := httptest.NewRequest("POST", "/users/preview-invite-url", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandler_PreviewInviteURL_Success_Unconfigured(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/preview-invite-url", handler.PreviewInviteURL)

	body := map[string]string{"email": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/preview-invite-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.Equal(t, false, resp["is_configured"].(bool))
	assert.Equal(t, true, resp["warning"].(bool))
	assert.Contains(t, resp["warning_message"].(string), "not configured")
	// When unconfigured, base_url and preview_url must be empty (CodeQL go/email-injection remediation)
	assert.Equal(t, "", resp["base_url"].(string), "base_url must be empty when public_url is not configured")
	assert.Equal(t, "", resp["preview_url"].(string), "preview_url must be empty when public_url is not configured")
	assert.Equal(t, "test@example.com", resp["email"].(string))
}

func TestUserHandler_PreviewInviteURL_Success_Configured(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create public_url setting
	publicURLSetting := &models.Setting{
		Key:      "app.public_url",
		Value:    "https://charon.example.com",
		Type:     "string",
		Category: "app",
	}
	db.Create(publicURLSetting)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/preview-invite-url", handler.PreviewInviteURL)

	body := map[string]string{"email": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/preview-invite-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.Equal(t, true, resp["is_configured"].(bool))
	assert.Equal(t, false, resp["warning"].(bool))
	assert.Contains(t, resp["preview_url"].(string), "https://charon.example.com")
	assert.Contains(t, resp["preview_url"].(string), "SAMPLE_TOKEN_PREVIEW")
	assert.Equal(t, "https://charon.example.com", resp["base_url"].(string))
	assert.Equal(t, "test@example.com", resp["email"].(string))
}

// getAppName Tests
func TestGetAppName_Default(t *testing.T) {
	_, db := setupUserHandlerWithProxyHosts(t)

	appName := getAppName(db)

	assert.Equal(t, "Charon", appName)
}

func TestGetAppName_FromSettings(t *testing.T) {
	_, db := setupUserHandlerWithProxyHosts(t)

	// Create app_name setting
	appNameSetting := &models.Setting{
		Key:      "app_name",
		Value:    "MyCustomApp",
		Type:     "string",
		Category: "app",
	}
	db.Create(appNameSetting)

	appName := getAppName(db)

	assert.Equal(t, "MyCustomApp", appName)
}

func TestGetAppName_EmptyValue(t *testing.T) {
	_, db := setupUserHandlerWithProxyHosts(t)

	// Create app_name setting with empty value
	appNameSetting := &models.Setting{
		Key:      "app_name",
		Value:    "",
		Type:     "string",
		Category: "app",
	}
	db.Create(appNameSetting)

	appName := getAppName(db)

	// Should return default when value is empty
	assert.Equal(t, "Charon", appName)
}

// ============= Priority 2: Error Paths =============

func TestUserHandler_UpdateUser_EmailConflict(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create two users
	user1 := &models.User{
		UUID:   uuid.NewString(),
		APIKey: uuid.NewString(),
		Email:  "user1@example.com",
		Name:   "User 1",
	}
	user2 := &models.User{
		UUID:   uuid.NewString(),
		APIKey: uuid.NewString(),
		Email:  "user2@example.com",
		Name:   "User 2",
	}
	db.Create(user1)
	db.Create(user2)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.PUT("/users/:id", handler.UpdateUser)

	// Try to update user1's email to user2's email
	body := map[string]string{
		"email": "user2@example.com",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/users/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "Email already in use")
}

// ============= Priority 3: Edge Cases and Defaults =============

func TestUserHandler_CreateUser_EmailNormalization(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	// Create user with mixed-case email
	body := map[string]any{
		"email":    "User@Example.COM",
		"name":     "Test User",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify email is stored lowercase
	var user models.User
	db.Where("email = ?", "user@example.com").First(&user)
	assert.Equal(t, "user@example.com", user.Email)
}

func TestUserHandler_InviteUser_EmailNormalization(t *testing.T) {
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

	// Invite user with mixed-case email
	body := map[string]any{
		"email": "Invite@Example.COM",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify email is stored lowercase
	var user models.User
	db.Where("email = ?", "invite@example.com").First(&user)
	assert.Equal(t, "invite@example.com", user.Email)
}

func TestUserHandler_CreateUser_DefaultPermissionMode(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	// Create user without specifying permission_mode
	body := map[string]any{
		"email":    "defaultperms@example.com",
		"name":     "Default Perms User",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify permission_mode defaults to "allow_all"
	var user models.User
	db.Where("email = ?", "defaultperms@example.com").First(&user)
	assert.Equal(t, models.PermissionModeAllowAll, user.PermissionMode)
}

func TestUserHandler_InviteUser_DefaultPermissionMode(t *testing.T) {
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

	// Invite user without specifying permission_mode
	body := map[string]any{
		"email": "defaultinvite@example.com",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify permission_mode defaults to "allow_all"
	var user models.User
	db.Where("email = ?", "defaultinvite@example.com").First(&user)
	assert.Equal(t, models.PermissionModeAllowAll, user.PermissionMode)
}

func TestUserHandler_CreateUser_DefaultRole(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	// Create user without specifying role
	body := map[string]any{
		"email":    "defaultrole@example.com",
		"name":     "Default Role User",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify role defaults to "user"
	var user models.User
	db.Where("email = ?", "defaultrole@example.com").First(&user)
	assert.Equal(t, "user", user.Role)
}

func TestUserHandler_InviteUser_DefaultRole(t *testing.T) {
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

	// Invite user without specifying role
	body := map[string]any{
		"email": "defaultroleinvite@example.com",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify role defaults to "user"
	var user models.User
	db.Where("email = ?", "defaultroleinvite@example.com").First(&user)
	assert.Equal(t, "user", user.Role)
}

// TestUserHandler_PreviewInviteURL_Unconfigured_DoesNotUseRequestHost verifies that
// when app.public_url is not configured, the preview does NOT use request Host header.
// This prevents host header injection attacks (CodeQL go/email-injection remediation).
func TestUserHandler_PreviewInviteURL_Unconfigured_DoesNotUseRequestHost(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/preview-invite-url", handler.PreviewInviteURL)

	body := map[string]string{"email": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users/preview-invite-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// Set malicious Host and X-Forwarded-Proto headers
	req.Host = "evil.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	// Response must NOT contain the malicious host
	responseJSON := w.Body.String()
	assert.NotContains(t, responseJSON, "evil.example.com", "Malicious Host header must not appear in response")
	// Verify base_url and preview_url are empty
	assert.Equal(t, "", resp["base_url"].(string))
	assert.Equal(t, "", resp["preview_url"].(string))
}

// ============= Priority 4: Integration Edge Cases =============

func TestUserHandler_CreateUser_EmptyPermittedHosts(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	// Create user with deny_all mode but empty permitted_hosts
	body := map[string]any{
		"email":           "emptyhosts@example.com",
		"name":            "Empty Hosts User",
		"password":        "password123",
		"permission_mode": "deny_all",
		"permitted_hosts": []uint{},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify user was created with deny_all mode and no permitted hosts
	var user models.User
	db.Preload("PermittedHosts").Where("email = ?", "emptyhosts@example.com").First(&user)
	assert.Equal(t, models.PermissionModeDenyAll, user.PermissionMode)
	assert.Len(t, user.PermittedHosts, 0)
}

func TestUserHandler_CreateUser_NonExistentPermittedHosts(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	// Create user with non-existent host IDs
	body := map[string]any{
		"email":           "nonexistenthosts@example.com",
		"name":            "Non-Existent Hosts User",
		"password":        "password123",
		"permission_mode": "deny_all",
		"permitted_hosts": []uint{999, 1000},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify user was created but no hosts were associated (non-existent IDs are ignored)
	var user models.User
	db.Preload("PermittedHosts").Where("email = ?", "nonexistenthosts@example.com").First(&user)
	assert.Len(t, user.PermittedHosts, 0)
}

// ============= ResendInvite Tests =============

func TestResendInvite_NonAdmin(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/users/:id/resend-invite", handler.ResendInvite)

	req := httptest.NewRequest("POST", "/users/1/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Admin access required")
}

func TestResendInvite_InvalidID(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/:id/resend-invite", handler.ResendInvite)

	req := httptest.NewRequest("POST", "/users/invalid/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid user ID")
}

func TestResendInvite_UserNotFound(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/:id/resend-invite", handler.ResendInvite)

	req := httptest.NewRequest("POST", "/users/999/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "User not found")
}

func TestResendInvite_UserNotPending(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create user with accepted invite (not pending)
	user := &models.User{
		UUID:         uuid.NewString(),
		APIKey:       uuid.NewString(),
		Email:        "accepted-user@example.com",
		Name:         "Accepted User",
		InviteStatus: "accepted",
		Enabled:      true,
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/:id/resend-invite", handler.ResendInvite)

	req := httptest.NewRequest("POST", "/users/"+strconv.FormatUint(uint64(user.ID), 10)+"/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "does not have a pending invite")
}

func TestResendInvite_Success(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create user with pending invite
	expires := time.Now().Add(24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		APIKey:        uuid.NewString(),
		Email:         "pending-user@example.com",
		Name:          "Pending User",
		InviteStatus:  "pending",
		InviteToken:   "oldtoken123",
		InviteExpires: &expires,
		Enabled:       false,
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/:id/resend-invite", handler.ResendInvite)

	req := httptest.NewRequest("POST", "/users/"+strconv.FormatUint(uint64(user.ID), 10)+"/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
	assert.NotEmpty(t, resp["invite_token"])
	assert.NotEqual(t, "oldtoken123", resp["invite_token"])
	assert.Equal(t, "pending-user@example.com", resp["email"])
	assert.Equal(t, false, resp["email_sent"].(bool)) // No SMTP configured

	// Verify token was updated in DB
	var updatedUser models.User
	db.First(&updatedUser, user.ID)
	assert.NotEqual(t, "oldtoken123", updatedUser.InviteToken)
	assert.Equal(t, resp["invite_token"], updatedUser.InviteToken)
}

func TestResendInvite_WithExpiredInvite(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)

	// Create user with expired pending invite
	expired := time.Now().Add(-24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		APIKey:        uuid.NewString(),
		Email:         "expired-pending@example.com",
		Name:          "Expired Pending User",
		InviteStatus:  "pending",
		InviteToken:   "expiredtoken",
		InviteExpires: &expired,
		Enabled:       false,
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/users/:id/resend-invite", handler.ResendInvite)

	req := httptest.NewRequest("POST", "/users/"+strconv.FormatUint(uint64(user.ID), 10)+"/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should succeed - resend should work even if previous invite expired
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
	assert.NotEmpty(t, resp["invite_token"])
	assert.NotEqual(t, "expiredtoken", resp["invite_token"])

	// Verify new expiration is in the future
	var updatedUser models.User
	db.First(&updatedUser, user.ID)
	assert.True(t, updatedUser.InviteExpires.After(time.Now()))
}
