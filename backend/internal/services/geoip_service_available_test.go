package services

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestGeoIPServiceAvailable(t *testing.T) {
	t.Run("nil service has no database", func(t *testing.T) {
		var svc *GeoIPService
		assert.False(t, svc.Available())
	})

	t.Run("unloaded service is not available", func(t *testing.T) {
		svc := NewUnloadedGeoIPService(filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb"))
		assert.False(t, svc.Available())
	})

	t.Run("loaded service whose file is missing is not available", func(t *testing.T) {
		svc := &GeoIPService{dbPath: "/fixture/country.mmdb", db: &fakeGeoIPReader{isoCode: "US"}}
		assert.False(t, svc.Available())
	})

	t.Run("loaded service on a file without the mmdb suffix is not available", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "country.bin")
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
		svc := &GeoIPService{dbPath: path, db: &fakeGeoIPReader{isoCode: "US"}}
		assert.False(t, svc.Available())
	})

	t.Run("loaded service on an mmdb file is available", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
		svc := &GeoIPService{dbPath: path, db: &fakeGeoIPReader{isoCode: "US"}}
		assert.True(t, svc.Available())
	})
}

func TestEvaluateWithoutGeoIPServiceLeavesCountryListUnenforced(t *testing.T) {
	svc := NewAccessListService(nil)
	svc.SetGeoIPService(nil)

	d, err := svc.Evaluate(&models.AccessList{Type: "geo_blacklist", CountryCodes: "RU", Enabled: true}, netip.MustParseAddr(testVisitor), nil)
	require.NoError(t, err)
	assert.True(t, d.Allowed)
	assert.Equal(t, ReasonGeoIPUnavailable, d.Reason)
	assert.False(t, d.GeoIPAvailable)
}
