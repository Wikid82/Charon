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
	// Check default template is minimal if Config is empty
	assert.Equal(t, "minimal", provider.Template)

	// If Config is present, Template default should be 'custom'
	provider2 := models.NotificationProvider{
		Name:   "Test2",
		Config: `{"custom":"ok"}`,
	}
	err = db.Create(&provider2).Error
	require.NoError(t, err)
	assert.Equal(t, "custom", provider2.Template)
}
