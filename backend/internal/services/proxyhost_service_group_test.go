package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProxyHostGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyGroup{},
		&models.ProxyHost{},
		&models.Location{},
		&models.SSLCertificate{},
		&models.AccessList{},
		&models.SecurityHeaderProfile{},
		&models.DNSProvider{},
	))
	return db
}

func TestProxyHostService_List_PreloadsGroup(t *testing.T) {
	db := setupProxyHostGroupTestDB(t)
	svc := NewProxyHostService(db)

	group := &models.ProxyGroup{Name: "Web Services", Color: "#00ff00"}
	require.NoError(t, db.Create(group).Error)

	host := &models.ProxyHost{
		UUID:          "host-list-uuid",
		DomainNames:   "example.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   80,
		ProxyGroupID:  &group.ID,
	}
	require.NoError(t, db.Create(host).Error)

	hosts, err := svc.List()
	require.NoError(t, err)
	require.Len(t, hosts, 1)
	require.NotNil(t, hosts[0].ProxyGroup)
	assert.Equal(t, "Web Services", hosts[0].ProxyGroup.Name)
}

func TestProxyHostService_GetByUUID_PreloadsGroup(t *testing.T) {
	db := setupProxyHostGroupTestDB(t)
	svc := NewProxyHostService(db)

	group := &models.ProxyGroup{Name: "API Services", Color: "#0000ff"}
	require.NoError(t, db.Create(group).Error)

	host := &models.ProxyHost{
		UUID:          "host-getbyuuid-uuid",
		DomainNames:   "api.example.com",
		ForwardScheme: "https",
		ForwardHost:   "localhost",
		ForwardPort:   443,
		ProxyGroupID:  &group.ID,
	}
	require.NoError(t, db.Create(host).Error)

	got, err := svc.GetByUUID("host-getbyuuid-uuid")
	require.NoError(t, err)
	require.NotNil(t, got.ProxyGroup)
	assert.Equal(t, "API Services", got.ProxyGroup.Name)
}
