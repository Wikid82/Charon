package models

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthUserTestDB(t *testing.T) *gorm.DB {
	dsn := filepath.Join(t.TempDir(), "test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuthUser{}))
	return db
}

func TestAuthUser_BeforeCreate(t *testing.T) {
	db := setupAuthUserTestDB(t)

	t.Run("generates UUID when empty", func(t *testing.T) {
		user := &AuthUser{
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, db.Create(user).Error)
		assert.NotEmpty(t, user.UUID)
		assert.Len(t, user.UUID, 36) // UUID format
	})

	t.Run("keeps existing UUID", func(t *testing.T) {
		customUUID := "custom-uuid-value"
		user := &AuthUser{
			UUID:         customUUID,
			Username:     "testuser2",
			Email:        "test2@example.com",
			PasswordHash: "hash",
		}
		require.NoError(t, db.Create(user).Error)
		assert.Equal(t, customUUID, user.UUID)
	})
}

func TestAuthUser_SetPassword(t *testing.T) {
	t.Run("hashes password", func(t *testing.T) {
		user := &AuthUser{}
		err := user.SetPassword("mypassword123")
		require.NoError(t, err)
		assert.NotEmpty(t, user.PasswordHash)
		assert.NotEqual(t, "mypassword123", user.PasswordHash)
		// bcrypt hashes start with $2a$ or $2b$
		assert.Contains(t, user.PasswordHash, "$2a$")
	})

	t.Run("empty password", func(t *testing.T) {
		user := &AuthUser{}
		err := user.SetPassword("")
		require.NoError(t, err)
		assert.NotEmpty(t, user.PasswordHash)
	})
}

func TestAuthUser_CheckPassword(t *testing.T) {
	user := &AuthUser{}
	require.NoError(t, user.SetPassword("correctpassword"))

	t.Run("correct password returns true", func(t *testing.T) {
		assert.True(t, user.CheckPassword("correctpassword"))
	})

	t.Run("wrong password returns false", func(t *testing.T) {
		assert.False(t, user.CheckPassword("wrongpassword"))
	})

	t.Run("empty password returns false", func(t *testing.T) {
		assert.False(t, user.CheckPassword(""))
	})
}

func TestAuthUser_HasRole(t *testing.T) {
	t.Run("empty roles returns false", func(t *testing.T) {
		user := &AuthUser{Roles: ""}
		assert.False(t, user.HasRole("admin"))
	})

	t.Run("single role match", func(t *testing.T) {
		user := &AuthUser{Roles: "admin"}
		assert.True(t, user.HasRole("admin"))
		assert.False(t, user.HasRole("user"))
	})

	t.Run("multiple roles", func(t *testing.T) {
		user := &AuthUser{Roles: "admin,user,editor"}
		assert.True(t, user.HasRole("admin"))
		assert.True(t, user.HasRole("user"))
		assert.True(t, user.HasRole("editor"))
		assert.False(t, user.HasRole("guest"))
	})

	t.Run("roles with spaces", func(t *testing.T) {
		user := &AuthUser{Roles: "admin, user, editor"}
		assert.True(t, user.HasRole("admin"))
		assert.True(t, user.HasRole("user"))
		assert.True(t, user.HasRole("editor"))
	})
}

func TestSplitRoles(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", []string{}},
		{"single role", "admin", []string{"admin"}},
		{"multiple roles", "admin,user", []string{"admin", "user"}},
		{"with spaces", "admin, user, editor", []string{"admin", "user", "editor"}},
		{"trailing comma", "admin,user,", []string{"admin", "user"}},
		{"leading comma", ",admin,user", []string{"admin", "user"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitRoles(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
