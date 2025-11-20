package services

import (
	"testing"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	return db
}

func TestAuthService_Register(t *testing.T) {
	db := setupTestDB(t)
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
	db := setupTestDB(t)
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
