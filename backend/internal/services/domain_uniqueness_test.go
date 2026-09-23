package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupDomainUniquenessTestDB migrates both ProxyHost and RedirectionHost so
// CheckDomainConflict has both tables available to query against.
func setupDomainUniquenessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.RedirectionHost{}))
	return db
}

func TestCheckDomainConflict_FindsConflict(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "old-blog.example.com",
		TargetURL:   "https://newblog.example.com",
		StatusCode:  301,
	}).Error)

	err := CheckDomainConflict(db, "old-blog.example.com", &models.RedirectionHost{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain already in use by another host")
}

func TestCheckDomainConflict_CaseInsensitiveAndMultiDomain(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "Old-Blog.example.com,other.example.com",
		TargetURL:   "https://newblog.example.com",
		StatusCode:  301,
	}).Error)

	// candidate list contains a domain that overlaps case-insensitively
	err := CheckDomainConflict(db, "unrelated.example.com,OLD-BLOG.EXAMPLE.COM", &models.RedirectionHost{})
	assert.Error(t, err)
}

func TestCheckDomainConflict_NoConflict(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "old-blog.example.com",
		TargetURL:   "https://newblog.example.com",
		StatusCode:  301,
	}).Error)

	err := CheckDomainConflict(db, "brand-new.example.com", &models.RedirectionHost{})
	assert.NoError(t, err)
}

func TestCheckDomainConflict_EmptyDomainNames(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)

	err := CheckDomainConflict(db, "", &models.RedirectionHost{})
	assert.NoError(t, err)

	err = CheckDomainConflict(db, "  , ,", &models.RedirectionHost{})
	assert.NoError(t, err)
}

func TestCheckDomainConflict_OtherTableMissing_NoError(t *testing.T) {
	// Only ProxyHost is migrated — RedirectionHost table does not exist.
	// This mirrors legacy test DB setups elsewhere in this package that
	// predate the RedirectionHost model; CheckDomainConflict must not turn
	// that into a hard error.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}))

	err = CheckDomainConflict(db, "example.com", &models.RedirectionHost{})
	assert.NoError(t, err)
}

func TestCheckDomainConflict_DBError(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	// SSLCertificate's table exists (so the HasTable guard passes) but has no
	// domain_names column, so the underlying Select query genuinely fails —
	// this exercises CheckDomainConflict's real DB-error branch without
	// relying on a closed connection, which HasTable would otherwise mask.
	err := CheckDomainConflict(db, "example.com", &models.SSLCertificate{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checking cross-table domain conflict")
}
