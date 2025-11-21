package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	return db
}

func TestAuthService_Register(t *testing.T) {
	db := setupAuthTestDB(t)
	cfg := config.Config{JWTSecret: "test-secret"}
	service := NewAuthService(db, cfg)

	// Test 1: First user should be admin
	admin, err := service.Register("admin@example.com", "password123", "Admin User")
	require.NoError(t, err)
	assert.Equal(t, "admin", admin.Role)
	assert.NotEmpty(t, admin.PasswordHash)
	assert.NotEqual(t, "password123", admin.PasswordHash)

	// Test 2: Second user should be regular user
	user, err := service.Register("user@example.com", "password123", "Regular User")
	require.NoError(t, err)
	assert.Equal(t, "user", user.Role)
}

func TestAuthService_Login(t *testing.T) {
	db := setupAuthTestDB(t)
	cfg := config.Config{JWTSecret: "test-secret"}
	service := NewAuthService(db, cfg)

	// Setup user
	_, err := service.Register("test@example.com", "password123", "Test User")
	require.NoError(t, err)

	// Test 1: Successful login
	token, err := service.Login("test@example.com", "password123")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Test 2: Invalid password
	token, err = service.Login("test@example.com", "wrongpassword")
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, "invalid credentials", err.Error())

	// Test 3: Account locking
	// Fail 4 more times (total 5)
	for i := 0; i < 4; i++ {
		_, err = service.Login("test@example.com", "wrongpassword")
		assert.Error(t, err)
	}

	// Check if locked
	var user models.User
	db.Where("email = ?", "test@example.com").First(&user)
	assert.Equal(t, 5, user.FailedLoginAttempts)
	assert.NotNil(t, user.LockedUntil)
	assert.True(t, user.LockedUntil.After(time.Now()))

	// Try login with correct password while locked
	token, err = service.Login("test@example.com", "password123")
	assert.Error(t, err)
	assert.Equal(t, "account locked", err.Error())
}

func TestAuthService_ChangePassword(t *testing.T) {
	db := setupAuthTestDB(t)
	cfg := config.Config{JWTSecret: "test-secret"}
	service := NewAuthService(db, cfg)

	user, err := service.Register("test@example.com", "password123", "Test User")
	require.NoError(t, err)

	// Success
	err = service.ChangePassword(user.ID, "password123", "newpassword")
	assert.NoError(t, err)

	// Verify login with new password
	_, err = service.Login("test@example.com", "newpassword")
	assert.NoError(t, err)

	// Fail with old password
	_, err = service.Login("test@example.com", "password123")
	assert.Error(t, err)

	// Fail with wrong current password
	err = service.ChangePassword(user.ID, "wrong", "another")
	assert.Error(t, err)
	assert.Equal(t, "invalid current password", err.Error())

	// Fail with non-existent user
	err = service.ChangePassword(999, "password", "new")
	assert.Error(t, err)
}

func TestAuthService_ValidateToken(t *testing.T) {
	db := setupAuthTestDB(t)
	cfg := config.Config{JWTSecret: "test-secret"}
	service := NewAuthService(db, cfg)

	user, err := service.Register("test@example.com", "password123", "Test User")
	require.NoError(t, err)

	token, err := service.Login("test@example.com", "password123")
	require.NoError(t, err)

	// Valid token
	claims, err := service.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)

	// Invalid token
	_, err = service.ValidateToken("invalid.token.string")
	assert.Error(t, err)
}
