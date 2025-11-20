package services

import (
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProxyHostTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}))
	return db
}

func TestProxyHostService_ValidateUniqueDomain(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	// Create existing host
	existing := &models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	require.NoError(t, db.Create(existing).Error)

	// Test 1: Duplicate domain
	err := service.ValidateUniqueDomain("example.com", 0)
	assert.Error(t, err)
	assert.Equal(t, "domain already exists", err.Error())

	// Test 2: New domain
	err = service.ValidateUniqueDomain("new.com", 0)
	assert.NoError(t, err)

	// Test 3: Update existing (exclude self)
	err = service.ValidateUniqueDomain("example.com", existing.ID)
	assert.NoError(t, err)
}
