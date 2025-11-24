package services

import (
	"fmt"
	"net"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProxyHostTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}))
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

	tests := []struct {
		name        string
		domainNames string
		excludeID   uint
		wantErr     bool
	}{
		{
			name:        "New unique domain",
			domainNames: "new.example.com",
			excludeID:   0,
			wantErr:     false,
		},
		{
			name:        "Duplicate domain",
			domainNames: "example.com",
			excludeID:   0,
			wantErr:     true,
		},
		{
			name:        "Same domain but excluded ID (update self)",
			domainNames: "example.com",
			excludeID:   existing.ID,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateUniqueDomain(tt.domainNames, tt.excludeID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProxyHostService_CRUD(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	// Create
	host := &models.ProxyHost{
		UUID:        "uuid-1",
		DomainNames: "test.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	err := service.Create(host)
	assert.NoError(t, err)
	assert.NotZero(t, host.ID)

	// Create Duplicate
	dup := &models.ProxyHost{
		UUID:        "uuid-2",
		DomainNames: "test.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8081,
	}
	err = service.Create(dup)
	assert.Error(t, err)

	// GetByID
	fetched, err := service.GetByID(host.ID)
	assert.NoError(t, err)
	assert.Equal(t, host.DomainNames, fetched.DomainNames)

	// GetByUUID
	fetchedUUID, err := service.GetByUUID(host.UUID)
	assert.NoError(t, err)
	assert.Equal(t, host.ID, fetchedUUID.ID)

	// Update
	host.ForwardPort = 9090
	err = service.Update(host)
	assert.NoError(t, err)

	fetched, err = service.GetByID(host.ID)
	assert.NoError(t, err)
	assert.Equal(t, 9090, fetched.ForwardPort)

	// Update Duplicate
	host2 := &models.ProxyHost{
		UUID:        "uuid-3",
		DomainNames: "other.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	service.Create(host2)

	host.DomainNames = "other.example.com" // Conflict with host2
	err = service.Update(host)
	assert.Error(t, err)

	// List
	hosts, err := service.List()
	assert.NoError(t, err)
	assert.Len(t, hosts, 2)

	// Delete
	err = service.Delete(host.ID)
	assert.NoError(t, err)

	_, err = service.GetByID(host.ID)
	assert.Error(t, err)
}

func TestProxyHostService_TestConnection(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	// 1. Invalid Input
	err := service.TestConnection("", 80)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid host or port")

	err = service.TestConnection("example.com", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid host or port")

	// 2. Connection Failure (Unreachable)
	err = service.TestConnection("localhost", 54321)
	assert.Error(t, err)

	// 3. Connection Success
	// Start a local listener
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)

	err = service.TestConnection(addr.IP.String(), addr.Port)
	assert.NoError(t, err)
}
