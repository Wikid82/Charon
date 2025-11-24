package services

import (
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRemoteServerTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteServer{}))
	// Clear table
	db.Exec("DELETE FROM remote_servers")
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

func TestRemoteServerService_CRUD(t *testing.T) {
	db := setupRemoteServerTestDB(t)
	service := NewRemoteServerService(db)

	// Create
	rs := &models.RemoteServer{
		UUID:     uuid.NewString(),
		Name:     "Test Server",
		Host:     "192.168.1.100",
		Port:     22,
		Provider: "manual",
	}
	err := service.Create(rs)
	require.NoError(t, err)
	assert.NotZero(t, rs.ID)
	assert.NotEmpty(t, rs.UUID)

	// GetByID
	fetched, err := service.GetByID(rs.ID)
	require.NoError(t, err)
	assert.Equal(t, rs.Name, fetched.Name)

	// GetByUUID
	fetchedUUID, err := service.GetByUUID(rs.UUID)
	require.NoError(t, err)
	assert.Equal(t, rs.ID, fetchedUUID.ID)

	// Update
	rs.Name = "Updated Server"
	err = service.Update(rs)
	require.NoError(t, err)

	fetchedUpdated, err := service.GetByID(rs.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Server", fetchedUpdated.Name)

	// List
	list, err := service.List(false)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete
	err = service.Delete(rs.ID)
	require.NoError(t, err)

	// Verify Delete
	_, err = service.GetByID(rs.ID)
	assert.Error(t, err)
}
