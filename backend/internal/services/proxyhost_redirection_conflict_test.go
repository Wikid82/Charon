package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupProxyHostRedirectionConflictTestDB migrates ProxyHost, Location, and
// RedirectionHost so the additive CheckDomainConflict call in
// ProxyHostService.Create/Update has a real RedirectionHost table to query
// against. This is deliberately a separate helper from
// setupProxyHostTestDB (proxyhost_service_test.go), which is left untouched
// so every pre-existing ProxyHostService test keeps passing unmodified.
func setupProxyHostRedirectionConflictTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.RedirectionHost{}))
	return db
}

func TestProxyHostService_Create_RejectsDomainUsedByRedirectionHost(t *testing.T) {
	db := setupProxyHostRedirectionConflictTestDB(t)
	service := NewProxyHostService(db)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "claimed.example.com",
		TargetURL:   "https://elsewhere.example.com",
		StatusCode:  301,
	}).Error)

	host := &models.ProxyHost{
		UUID:        "ph-1",
		DomainNames: "claimed.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	err := service.Create(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain already in use by another host")
}

func TestProxyHostService_Update_RejectsDomainUsedByRedirectionHost(t *testing.T) {
	db := setupProxyHostRedirectionConflictTestDB(t)
	service := NewProxyHostService(db)

	host := &models.ProxyHost{
		UUID:        "ph-1",
		DomainNames: "mine.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	require.NoError(t, service.Create(host))

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "claimed.example.com",
		TargetURL:   "https://elsewhere.example.com",
		StatusCode:  301,
	}).Error)

	host.DomainNames = "claimed.example.com"
	err := service.Update(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain already in use by another host")
}

func TestProxyHostService_Create_NoRedirectionHostConflict_Succeeds(t *testing.T) {
	db := setupProxyHostRedirectionConflictTestDB(t)
	service := NewProxyHostService(db)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "unrelated.example.com",
		TargetURL:   "https://elsewhere.example.com",
		StatusCode:  301,
	}).Error)

	host := &models.ProxyHost{
		UUID:        "ph-1",
		DomainNames: "free.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	err := service.Create(host)
	assert.NoError(t, err)
}
