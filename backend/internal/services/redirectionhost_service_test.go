package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupRedirectionHostTestDB migrates both RedirectionHost and ProxyHost so
// the minimal RedirectionHostService's same-table check and its additive
// cross-table check both have real tables to query.
func setupRedirectionHostTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RedirectionHost{}, &models.ProxyHost{}, &models.Location{}))
	return db
}

func TestRedirectionHostService_ValidateUniqueDomain(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	existing := &models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "old-blog.example.com",
		TargetURL:   "https://newblog.example.com",
		StatusCode:  301,
	}
	require.NoError(t, db.Create(existing).Error)

	tests := []struct {
		name        string
		domainNames string
		excludeID   uint
		wantErr     bool
	}{
		{name: "new unique domain", domainNames: "new.example.com", wantErr: false},
		{name: "duplicate domain", domainNames: "old-blog.example.com", wantErr: true},
		{name: "same domain excluded (update self)", domainNames: "old-blog.example.com", excludeID: existing.ID, wantErr: false},
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

func TestRedirectionHostService_ValidateUniqueDomain_DBError(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = service.ValidateUniqueDomain("example.com", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checking domain uniqueness")
}

func TestRedirectionHostService_CheckCrossTableDomainConflict_RejectsProxyHostDomain(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:        "ph-1",
		DomainNames: "claimed.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}).Error)

	err := service.CheckCrossTableDomainConflict("claimed.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain already in use by another host")
}

func TestRedirectionHostService_CheckCrossTableDomainConflict_RejectsOtherRedirectionHostDomain(t *testing.T) {
	// CheckCrossTableDomainConflict only checks ProxyHost; the same-table
	// rejection for another RedirectionHost's domain goes through
	// ValidateUniqueDomain above. This test documents that combination as
	// Commit 4's Create/Update are expected to call both checks together.
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "claimed.example.com",
		TargetURL:   "https://elsewhere.example.com",
		StatusCode:  301,
	}).Error)

	// Same-table check (own table) catches it.
	err := service.ValidateUniqueDomain("claimed.example.com", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain already exists")

	// Cross-table check against ProxyHost does not (no ProxyHost row exists).
	err = service.CheckCrossTableDomainConflict("claimed.example.com")
	assert.NoError(t, err)
}

func TestRedirectionHostService_CheckCrossTableDomainConflict_NoConflict(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:        "ph-1",
		DomainNames: "unrelated.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}).Error)

	err := service.CheckCrossTableDomainConflict("free.example.com")
	assert.NoError(t, err)
}
