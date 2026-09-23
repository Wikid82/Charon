package services

import (
	"fmt"
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

func validRedirectionHost() *models.RedirectionHost {
	return &models.RedirectionHost{
		UUID:        "rh-valid",
		Name:        "Old blog redirect",
		DomainNames: "old-blog.example.com",
		TargetURL:   "https://newblog.example.com",
		StatusCode:  301,
	}
}

func TestRedirectionHostService_Create_Valid(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	require.NoError(t, service.Create(host))
	assert.NotZero(t, host.ID)

	var count int64
	db.Model(&models.RedirectionHost{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestRedirectionHostService_Create_RequiresDomainNames(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	host.DomainNames = "   "
	err := service.Create(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain names is required")
}

func TestRedirectionHostService_Create_RequiresTargetURL(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	host.TargetURL = ""
	err := service.Create(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_url is required")
}

func TestRedirectionHostService_Create_RejectsInvalidTargetURL(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	tests := []struct {
		name      string
		targetURL string
	}{
		{"missing scheme", "example.com"},
		{"disallowed scheme", "ftp://example.com"},
		{"javascript scheme", "javascript:alert(1)"},
		{"empty host", "https:///path-only"},
		{"unparseable", "https://%zz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := validRedirectionHost()
			host.UUID = "rh-" + tt.name
			host.TargetURL = tt.targetURL
			err := service.Create(host)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "target_url must be a valid http(s) URL")
		})
	}
}

func TestRedirectionHostService_Create_RejectsInvalidStatusCode(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	for _, code := range []int{200, 300, 303, 304, 404, 500, 0} {
		host := validRedirectionHost()
		host.UUID = "rh-code"
		host.StatusCode = code
		err := service.Create(host)
		require.Error(t, err, "status code %d should be rejected", code)
		assert.Contains(t, err.Error(), "status_code must be one of 301, 302, 307, 308")
	}
}

func TestRedirectionHostService_Create_AcceptsAllValidStatusCodes(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	for i, code := range []int{301, 302, 307, 308} {
		host := validRedirectionHost()
		host.UUID = fmt.Sprintf("rh-code-%d", i)
		host.DomainNames = fmt.Sprintf("code-%d.example.com", i)
		host.StatusCode = code
		require.NoError(t, service.Create(host))
	}
}

func TestRedirectionHostService_Create_RejectsSelfRedirect(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	host.DomainNames = "loop.example.com"
	host.TargetURL = "https://loop.example.com/somewhere"
	err := service.Create(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect target cannot point back to one of this host's own domains")
}

func TestRedirectionHostService_Create_RejectsSelfRedirect_MultiDomain(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	host.DomainNames = "a.example.com, loop.example.com"
	host.TargetURL = "https://LOOP.example.com"
	err := service.Create(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect target cannot point back to one of this host's own domains")
}

func TestRedirectionHostService_Create_RejectsDuplicateDomain(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	first := validRedirectionHost()
	require.NoError(t, service.Create(first))

	second := validRedirectionHost()
	second.UUID = "rh-dup"
	err := service.Create(second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain already exists")
}

func TestRedirectionHostService_Create_RejectsProxyHostDomainConflict(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:        "ph-1",
		DomainNames: "claimed.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}).Error)

	host := validRedirectionHost()
	host.DomainNames = "claimed.example.com"
	err := service.Create(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain already in use by another host")
}

func TestRedirectionHostService_Create_RejectsUseDNSChallengeWithoutProvider(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	host.UseDNSChallenge = true
	err := service.Create(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dns provider is required when use_dns_challenge is enabled")
}

func TestRedirectionHostService_Update_Valid(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	require.NoError(t, service.Create(host))

	host.StatusCode = 308
	host.TargetURL = "https://updated.example.com"
	require.NoError(t, service.Update(host))

	fetched, err := service.GetByUUID(host.UUID)
	require.NoError(t, err)
	assert.Equal(t, 308, fetched.StatusCode)
	assert.Equal(t, "https://updated.example.com", fetched.TargetURL)
}

func TestRedirectionHostService_Update_AllowsKeepingOwnDomain(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	require.NoError(t, service.Create(host))

	// Re-saving the same domain_names for the same row must not trip the
	// same-table uniqueness check (excludeID must be honored).
	host.Name = "Renamed"
	require.NoError(t, service.Update(host))
}

func TestRedirectionHostService_Update_RejectsInvalidStatusCode(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	require.NoError(t, service.Create(host))

	host.StatusCode = 200
	err := service.Update(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status_code must be one of 301, 302, 307, 308")
}

func TestRedirectionHostService_Delete(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	require.NoError(t, service.Create(host))

	require.NoError(t, service.Delete(host.ID))

	var count int64
	db.Model(&models.RedirectionHost{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestRedirectionHostService_GetByUUID_NotFound(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	_, err := service.GetByUUID("does-not-exist")
	assert.Error(t, err)
}

func TestRedirectionHostService_GetByID(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host := validRedirectionHost()
	require.NoError(t, service.Create(host))

	fetched, err := service.GetByID(host.ID)
	require.NoError(t, err)
	assert.Equal(t, host.UUID, fetched.UUID)
}

func TestRedirectionHostService_List(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	first := validRedirectionHost()
	first.DomainNames = "first.example.com"
	require.NoError(t, service.Create(first))

	second := validRedirectionHost()
	second.UUID = "rh-second"
	second.DomainNames = "second.example.com"
	require.NoError(t, service.Create(second))

	hosts, err := service.List()
	require.NoError(t, err)
	assert.Len(t, hosts, 2)
}

func TestRedirectionHostService_DB(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)
	assert.Equal(t, db, service.DB())
}

// TestRedirectionHostService_Update_RejectsDuplicateDomain confirms Update's
// same-table uniqueness check (ValidateUniqueDomain) rejects a rename onto a
// domain already owned by a different RedirectionHost row — the excludeID
// argument must only exempt the row being updated, not every other row.
func TestRedirectionHostService_Update_RejectsDuplicateDomain(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	first := validRedirectionHost()
	first.DomainNames = "taken.example.com"
	require.NoError(t, service.Create(first))

	second := validRedirectionHost()
	second.UUID = "rh-second-update"
	second.DomainNames = "second.example.com"
	require.NoError(t, service.Create(second))

	second.DomainNames = "taken.example.com"
	err := service.Update(second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain already exists")
}

// TestRedirectionHostService_Update_RejectsProxyHostDomainConflict confirms
// Update's additive cross-table check rejects a rename onto a domain owned
// by a ProxyHost row.
func TestRedirectionHostService_Update_RejectsProxyHostDomainConflict(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:        "ph-update-conflict",
		DomainNames: "claimed-by-proxy.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}).Error)

	host := validRedirectionHost()
	require.NoError(t, service.Create(host))

	host.DomainNames = "claimed-by-proxy.example.com"
	err := service.Update(host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain already in use by another host")
}

// TestRedirectionHostService_GetByID_NotFound confirms GetByID surfaces the
// underlying gorm.ErrRecordNotFound rather than a zero-value host on a miss.
func TestRedirectionHostService_GetByID_NotFound(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	host, err := service.GetByID(999999)
	assert.Error(t, err)
	assert.Nil(t, host)
}

// TestRedirectionHostService_List_DBError confirms List propagates the
// underlying query error (e.g. dropped/missing table) instead of returning
// an empty slice with a nil error.
func TestRedirectionHostService_List_DBError(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	service := NewRedirectionHostService(db)

	require.NoError(t, db.Migrator().DropTable(&models.RedirectionHost{}))

	hosts, err := service.List()
	assert.Error(t, err)
	assert.Nil(t, hosts)
}
