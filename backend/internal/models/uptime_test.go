package models_test

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUptimeMonitor_BeforeCreate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}))

	monitor := models.UptimeMonitor{
		Name: "Test",
	}
	err = db.Create(&monitor).Error
	require.NoError(t, err)

	assert.NotEmpty(t, monitor.ID)
	assert.Equal(t, "pending", monitor.Status)
}
