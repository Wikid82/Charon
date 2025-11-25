package models

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthProviderTestDB(t *testing.T) *gorm.DB {
	dsn := filepath.Join(t.TempDir(), "test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuthProvider{}))
	return db
}

func TestAuthProvider_BeforeCreate(t *testing.T) {
	db := setupAuthProviderTestDB(t)

	t.Run("generates UUID when empty", func(t *testing.T) {
		provider := &AuthProvider{
			Name: "test-provider",
			Type: "oidc",
		}
		require.NoError(t, db.Create(provider).Error)
		assert.NotEmpty(t, provider.UUID)
		assert.Len(t, provider.UUID, 36)
	})

	t.Run("keeps existing UUID", func(t *testing.T) {
		customUUID := "custom-provider-uuid"
		provider := &AuthProvider{
			UUID: customUUID,
			Name: "test-provider-2",
			Type: "google",
		}
		require.NoError(t, db.Create(provider).Error)
		assert.Equal(t, customUUID, provider.UUID)
	})
}
