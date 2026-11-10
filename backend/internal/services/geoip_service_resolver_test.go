package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil"
)

func TestNewResolvingGeoIPService_ReportsResolvedPathWithoutLoading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")

	svc := NewResolvingGeoIPService(func() string { return path })

	assert.Equal(t, path, svc.GetDatabasePath())
	assert.False(t, svc.IsLoaded())
}

func TestGeoIPService_LoadFromOpensExactPathWithoutResolving(t *testing.T) {
	resolved := filepath.Join(t.TempDir(), "absent.mmdb")
	exact := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	testutil.WriteGeoIPCountryDB(t, exact, 1)

	svc := NewResolvingGeoIPService(func() string { return resolved })
	require.NoError(t, svc.LoadFrom(exact))

	assert.True(t, svc.IsLoaded())
	assert.Equal(t, exact, svc.GetDatabasePath())
}

func TestGeoIPService_LoadReresolvesPathAndKeepsPreviousOnFailure(t *testing.T) {
	first := filepath.Join(t.TempDir(), "absent.mmdb")
	second := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(second, []byte("not a database"), 0o600))

	current := first
	svc := NewResolvingGeoIPService(func() string { return current })
	current = second

	require.Error(t, svc.Load(), "the re-resolved file is not a database")
	assert.Equal(t, first, svc.GetDatabasePath(), "a failed load keeps the path of the database in use")
	assert.False(t, svc.IsLoaded())
}
