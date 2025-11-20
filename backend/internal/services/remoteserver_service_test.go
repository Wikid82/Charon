package services

import (
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRemoteServerTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteServer{}))
	return db
}

func TestRemoteServerService_ValidateUniqueServer(t *testing.T) {
	db := setupRemoteServerTestDB(t)
	service := NewRemoteServerService(db)

	// Create existing server
	existing := &models.RemoteServer{
		Name: "Existing Server",
		Host: "192.168.1.100",
		Port: 8080,
	}
	require.NoError(t, db.Create(existing).Error)

	// Test 1: Duplicate Name
	err := service.ValidateUniqueServer("Existing Server", "192.168.1.101", 9090, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Test 2: Duplicate Host:Port
	err = service.ValidateUniqueServer("New Name", "192.168.1.100", 8080, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Test 3: New Server
	err = service.ValidateUniqueServer("New Server", "192.168.1.101", 8080, 0)
	assert.NoError(t, err)

	// Test 4: Update existing (exclude self)
	err = service.ValidateUniqueServer("Existing Server", "192.168.1.100", 8080, existing.ID)
	assert.NoError(t, err)
}
