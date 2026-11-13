package services

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeoIPService_Load_FailureKeepsLoadedDatabase(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing", "GeoLite2-Country.mmdb")
	svc := &GeoIPService{dbPath: missing, db: &fakeGeoIPReader{isoCode: "SE"}}

	require.Error(t, svc.Load(), "a file that cannot be opened is not loaded")
	require.True(t, svc.IsLoaded(), "the database loaded before the failed reload stays in use")

	country, err := svc.LookupCountry("8.8.8.8")
	require.NoError(t, err)
	require.Equal(t, "SE", country)
}
