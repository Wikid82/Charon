package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupUserCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.User{}, &models.Setting{})
	return db
}

func TestUserHandler_GetSetupStatus_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	// Drop table to cause error
	db.Migrator().DropTable(&models.User{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.GetSetupStatus(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to check setup status")
}

func TestUserHandler_Setup_CheckStatusError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	// Drop table to cause error
	db.Migrator().DropTable(&models.User{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.Setup(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to check setup status")
}

func TestUserHandler_Setup_AlreadyCompleted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	// Create a user to mark setup as complete
	user := &models.User{UUID: "uuid-a", Name: "Admin", Email: "admin@test.com", Role: "admin"}
	user.SetPassword("password123")
	db.Create(user)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.Setup(c)

	assert.Equal(t, 403, w.Code)
	assert.Contains(t, w.Body.String(), "Setup already completed")
}

func TestUserHandler_Setup_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/setup", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Setup(c)

	assert.Equal(t, 400, w.Code)
}

func TestUserHandler_RegenerateAPIKey_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No userID set in context

	h.RegenerateAPIKey(c)

	assert.Equal(t, 401, w.Code)
}

func TestUserHandler_RegenerateAPIKey_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	// Drop table to cause error
	db.Migrator().DropTable(&models.User{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uint(1))

	h.RegenerateAPIKey(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to update API key")
}

func TestUserHandler_GetProfile_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No userID set in context

	h.GetProfile(c)

	assert.Equal(t, 401, w.Code)
}

func TestUserHandler_GetProfile_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uint(9999)) // Non-existent user

	h.GetProfile(c)

	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "User not found")
}

func TestUserHandler_UpdateProfile_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No userID set in context

	h.UpdateProfile(c)

	assert.Equal(t, 401, w.Code)
}

func TestUserHandler_UpdateProfile_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uint(1))
	c.Request = httptest.NewRequest("PUT", "/profile", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateProfile(c)

	assert.Equal(t, 400, w.Code)
}

func TestUserHandler_UpdateProfile_UserNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	body, _ := json.Marshal(map[string]string{
		"name":  "Updated",
		"email": "updated@test.com",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uint(9999))
	c.Request = httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateProfile(c)

	assert.Equal(t, 404, w.Code)
}

func TestUserHandler_UpdateProfile_EmailConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	// Create two users
	user1 := &models.User{UUID: "uuid-1", Name: "User1", Email: "user1@test.com", Role: "admin", APIKey: "key1"}
	user1.SetPassword("password123")
	db.Create(user1)

	user2 := &models.User{UUID: "uuid-2", Name: "User2", Email: "user2@test.com", Role: "admin", APIKey: "key2"}
	user2.SetPassword("password123")
	db.Create(user2)

	// Try to change user2's email to user1's email
	body, _ := json.Marshal(map[string]string{
		"name":             "User2",
		"email":            "user1@test.com",
		"current_password": "password123",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", user2.ID)
	c.Request = httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateProfile(c)

	assert.Equal(t, 409, w.Code)
	assert.Contains(t, w.Body.String(), "Email already in use")
}

func TestUserHandler_UpdateProfile_EmailChangeNoPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	user := &models.User{UUID: "uuid-u", Name: "User", Email: "user@test.com", Role: "admin"}
	user.SetPassword("password123")
	db.Create(user)

	// Try to change email without password
	body, _ := json.Marshal(map[string]string{
		"name":  "User",
		"email": "newemail@test.com",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", user.ID)
	c.Request = httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateProfile(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "Current password is required")
}

func TestUserHandler_UpdateProfile_WrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupUserCoverageDB(t)
	h := NewUserHandler(db)

	user := &models.User{UUID: "uuid-u", Name: "User", Email: "user@test.com", Role: "admin"}
	user.SetPassword("password123")
	db.Create(user)

	// Try to change email with wrong password
	body, _ := json.Marshal(map[string]string{
		"name":             "User",
		"email":            "newemail@test.com",
		"current_password": "wrongpassword",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", user.ID)
	c.Request = httptest.NewRequest("PUT", "/profile", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateProfile(c)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid password")
}
