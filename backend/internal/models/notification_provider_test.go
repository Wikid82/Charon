package models_test

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNotificationProvider_BeforeCreate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	provider := models.NotificationProvider{
		Name: "Test",
	}
	err = db.Create(&provider).Error
	require.NoError(t, err)

	assert.NotEmpty(t, provider.ID)
	// Check defaults if any (currently none enforced in BeforeCreate other than ID)
}
