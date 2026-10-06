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
// checkCrossTableDomainConflict has both tables available to query against.
func setupDomainUniquenessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.RedirectionHost{}))
	return db
}

func TestCrossTableDomainConflict_FindsConflict(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "old-blog.example.com",
		TargetURL:   "https://newblog.example.com",
		StatusCode:  301,
	}).Error)

	err := checkCrossTableDomainConflict(db, "old-blog.example.com", &models.RedirectionHost{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain already in use by another host")
}

func TestCrossTableDomainConflict_CaseInsensitiveAndMultiDomain(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "Old-Blog.example.com,other.example.com",
		TargetURL:   "https://newblog.example.com",
		StatusCode:  301,
	}).Error)

	// candidate list contains a domain that overlaps case-insensitively
	err := checkCrossTableDomainConflict(db, "unrelated.example.com,OLD-BLOG.EXAMPLE.COM", &models.RedirectionHost{})
	assert.Error(t, err)
}

func TestCrossTableDomainConflict_NoConflict(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)

	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID:        "rh-1",
		DomainNames: "old-blog.example.com",
		TargetURL:   "https://newblog.example.com",
		StatusCode:  301,
	}).Error)

	err := checkCrossTableDomainConflict(db, "brand-new.example.com", &models.RedirectionHost{})
	assert.NoError(t, err)
}

func TestCrossTableDomainConflict_EmptyDomainNames(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)

	err := checkCrossTableDomainConflict(db, "", &models.RedirectionHost{})
	assert.NoError(t, err)

	err = checkCrossTableDomainConflict(db, "  , ,", &models.RedirectionHost{})
	assert.NoError(t, err)
}

func TestCrossTableDomainConflict_OtherTableMissing_NoError(t *testing.T) {
	// Only ProxyHost is migrated — RedirectionHost table does not exist.
	// This mirrors legacy test DB setups elsewhere in this package that
	// predate the RedirectionHost model; checkCrossTableDomainConflict must not turn
	// that into a hard error.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}))

	err = checkCrossTableDomainConflict(db, "example.com", &models.RedirectionHost{})
	assert.NoError(t, err)
}

func TestCrossTableDomainConflict_DBError(t *testing.T) {
	db := setupDomainUniquenessTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	// SSLCertificate's table exists (so the HasTable guard passes) but has no
	// domain_names column, so the underlying Select query genuinely fails —
	// this exercises checkCrossTableDomainConflict's real DB-error branch without
	// relying on a closed connection, which HasTable would otherwise mask.
	err := checkCrossTableDomainConflict(db, "example.com", &models.SSLCertificate{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checking cross-table domain conflict")
}
func TestCanonicalDomainNames(t *testing.T) {
	assert.Equal(t, "a.com,b.com", CanonicalDomainNames("  A.com , ,B.COM,a.com "))
	assert.Equal(t, "", CanonicalDomainNames(" , "))
}

func TestDomainsShareAny(t *testing.T) {
	assert.True(t, DomainsShareAny("a.com,b.com", " B.com"))
	assert.False(t, DomainsShareAny("a.com", "c.com"))
	assert.False(t, DomainsShareAny("", "c.com"))
}

func TestProxyHostService_SameTableDomainConflict_PerDomain(t *testing.T) {
	db := setupProxyHostTestDB(t)
	existing := &models.ProxyHost{UUID: "u1", DomainNames: "a.com,b.com", ForwardHost: "127.0.0.1", ForwardPort: 80}
	require.NoError(t, db.Create(existing).Error)

	tests := []struct {
		name      string
		domains   string
		excludeID uint
		wantErr   bool
	}{
		{"subset of existing", "b.com", 0, true},
		{"superset of existing", "x.com,a.com", 0, true},
		{"case variant", "B.COM", 0, true},
		{"whitespace variant", "  b.com  ", 0, true},
		{"whitespace in list", "x.com , A.com", 0, true},
		{"disjoint", "c.com,d.com", 0, false},
		{"update excluding self", "a.com", existing.ID, false},
		{"empty list", " , ", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSameTableDomainConflict(db, tt.domains, &models.ProxyHost{}, tt.excludeID)
			if tt.wantErr {
				assert.EqualError(t, err, "domain already exists")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProxyHostService_CreateUpdate_RejectOverlapAndNormalize(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)

	first := &models.ProxyHost{UUID: "u1", DomainNames: " A.com , B.com ", ForwardHost: "127.0.0.1", ForwardPort: 80}
	require.NoError(t, svc.Create(first))

	var stored models.ProxyHost
	require.NoError(t, db.First(&stored, first.ID).Error)
	assert.Equal(t, "a.com,b.com", stored.DomainNames)

	dup := &models.ProxyHost{UUID: "u2", DomainNames: "B.com", ForwardHost: "127.0.0.1", ForwardPort: 80}
	assert.EqualError(t, svc.Create(dup), "domain already exists")

	second := &models.ProxyHost{UUID: "u3", DomainNames: "c.com", ForwardHost: "127.0.0.1", ForwardPort: 80}
	require.NoError(t, svc.Create(second))

	second.DomainNames = "C.com, a.com"
	assert.EqualError(t, svc.Update(second), "domain already exists")

	second.DomainNames = "C.com,d.com"
	require.NoError(t, svc.Update(second))
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, second.ID).Error)
	assert.Equal(t, "c.com,d.com", updated.DomainNames)
}

func TestRedirectionHostService_SameTableDomainConflict_PerDomain(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	existing := &models.RedirectionHost{UUID: "r1", DomainNames: "a.com,b.com", TargetURL: "https://t.example", StatusCode: 301}
	require.NoError(t, db.Create(existing).Error)

	assert.EqualError(t, checkSameTableDomainConflict(db, "B.com", &models.RedirectionHost{}, 0), "domain already exists")
	assert.NoError(t, checkSameTableDomainConflict(db, "a.com", &models.RedirectionHost{}, existing.ID))
	assert.NoError(t, checkSameTableDomainConflict(db, "z.com", &models.RedirectionHost{}, 0))
}
