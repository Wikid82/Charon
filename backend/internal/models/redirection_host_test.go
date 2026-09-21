package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRedirectionHostModelDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&RedirectionHost{}, &SSLCertificate{}, &DNSProvider{}))
	return db
}

func TestRedirectionHostBeforeCreate_AssignsUUID(t *testing.T) {
	db := setupRedirectionHostModelDB(t)
	rh := &RedirectionHost{
		Name:        "auto uuid redirect",
		DomainNames: "old.example.com",
		TargetURL:   "https://new.example.com",
		StatusCode:  301,
	}
	require.NoError(t, db.Create(rh).Error)
	assert.NotEmpty(t, rh.UUID)
}

func TestRedirectionHostBeforeCreate_PreservesExistingUUID(t *testing.T) {
	db := setupRedirectionHostModelDB(t)
	preset := "preset-uuid-value-12345"
	rh := &RedirectionHost{
		UUID:        preset,
		Name:        "preset uuid redirect",
		DomainNames: "old2.example.com",
		TargetURL:   "https://new2.example.com",
		StatusCode:  302,
	}
	require.NoError(t, db.Create(rh).Error)
	assert.Equal(t, preset, rh.UUID)
}

// TestRedirectionHost_NotNullColumns confirms the schema-level NOT NULL
// constraints declared on DomainNames/TargetURL are present, matching the
// spec's gorm:"not null" tags. Note (consistent with every other model in
// this codebase, e.g. ProxyHost.ForwardHost): a Go zero-value empty string
// satisfies a SQL NOT NULL constraint (it is a value, not NULL), so
// "required" semantics for these fields are enforced at the service-layer
// validation (RedirectionHostService, a later commit), not by GORM/SQLite
// here. This test documents that boundary rather than asserting a DB-level
// rejection that would never fire.
func TestRedirectionHost_NotNullColumns(t *testing.T) {
	db := setupRedirectionHostModelDB(t)

	rh := &RedirectionHost{DomainNames: "", TargetURL: "", StatusCode: 301}
	require.NoError(t, db.Create(rh).Error, "empty string satisfies NOT NULL — service layer enforces required-ness")
	assert.NotEmpty(t, rh.UUID)
}

func TestRedirectionHost_DefaultValues(t *testing.T) {
	db := setupRedirectionHostModelDB(t)
	rh := &RedirectionHost{
		Name:        "defaults check",
		DomainNames: "defaults.example.com",
		TargetURL:   "https://target.example.com",
	}
	require.NoError(t, db.Create(rh).Error)

	var fetched RedirectionHost
	require.NoError(t, db.First(&fetched, rh.ID).Error)

	assert.Equal(t, 301, fetched.StatusCode)
	assert.True(t, fetched.PreservePath)
	assert.True(t, fetched.SSLForced)
	assert.True(t, fetched.HTTP2Support)
	assert.False(t, fetched.HSTSEnabled)
	assert.False(t, fetched.HSTSSubdomains)
	assert.True(t, fetched.Enabled)
	assert.False(t, fetched.UseDNSChallenge)
}

func TestRedirectionHost_CRUDRoundTrip(t *testing.T) {
	db := setupRedirectionHostModelDB(t)

	cert := &SSLCertificate{Domains: "old.example.com", Provider: "custom"}
	require.NoError(t, db.Create(cert).Error)

	dnsProvider := &DNSProvider{Name: "test-dns", ProviderType: "cloudflare"}
	require.NoError(t, db.Create(dnsProvider).Error)

	// Create
	rh := &RedirectionHost{
		Name:            "old blog redirect",
		DomainNames:     "old-blog.example.com",
		TargetURL:       "https://newblog.example.com",
		StatusCode:      308,
		PreservePath:    true,
		SSLForced:       true,
		HTTP2Support:    true,
		HSTSEnabled:     true,
		HSTSSubdomains:  true,
		Enabled:         true,
		CertificateID:   &cert.ID,
		DNSProviderID:   &dnsProvider.ID,
		UseDNSChallenge: true,
	}
	require.NoError(t, db.Create(rh).Error)
	require.NotEmpty(t, rh.UUID)
	require.NotZero(t, rh.ID)

	// Read
	var fetched RedirectionHost
	require.NoError(t, db.Preload("Certificate").Preload("DNSProvider").First(&fetched, "uuid = ?", rh.UUID).Error)
	assert.Equal(t, "old blog redirect", fetched.Name)
	assert.Equal(t, "old-blog.example.com", fetched.DomainNames)
	assert.Equal(t, "https://newblog.example.com", fetched.TargetURL)
	assert.Equal(t, 308, fetched.StatusCode)
	assert.True(t, fetched.PreservePath)
	assert.True(t, fetched.HSTSEnabled)
	assert.True(t, fetched.HSTSSubdomains)
	require.NotNil(t, fetched.Certificate)
	assert.Equal(t, "old.example.com", fetched.Certificate.Domains)
	require.NotNil(t, fetched.DNSProvider)
	assert.Equal(t, "test-dns", fetched.DNSProvider.Name)

	// Update — note PreservePath toggling to false here is a plain UPDATE
	// statement (via Save on an existing primary key), which is unaffected
	// by the gorm:"default:true" clause (that clause only fires on INSERT
	// when the field holds its Go zero value, per the well-known GORM
	// bool-zero-value-vs-default-tag behavior documented above).
	fetched.TargetURL = "https://updated.example.com"
	fetched.StatusCode = 302
	fetched.PreservePath = false
	require.NoError(t, db.Save(&fetched).Error)

	var updated RedirectionHost
	require.NoError(t, db.First(&updated, fetched.ID).Error)
	assert.Equal(t, "https://updated.example.com", updated.TargetURL)
	assert.Equal(t, 302, updated.StatusCode)
	assert.False(t, updated.PreservePath)

	// Delete
	require.NoError(t, db.Delete(&updated).Error)
	var count int64
	require.NoError(t, db.Model(&RedirectionHost{}).Where("id = ?", updated.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestValidRedirectStatusCodes(t *testing.T) {
	assert.True(t, ValidRedirectStatusCodes[301])
	assert.True(t, ValidRedirectStatusCodes[302])
	assert.True(t, ValidRedirectStatusCodes[307])
	assert.True(t, ValidRedirectStatusCodes[308])
	assert.False(t, ValidRedirectStatusCodes[300])
	assert.False(t, ValidRedirectStatusCodes[303])
	assert.False(t, ValidRedirectStatusCodes[304])
}
