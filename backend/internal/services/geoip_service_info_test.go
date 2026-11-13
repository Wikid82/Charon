package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil"
)

// fixedClock returns a clock that always reports at.
func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func TestGeoIPServiceInfo_BeforeLoadReportsPathOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	svc := NewUnloadedGeoIPService(path)

	info := svc.Info()

	assert.Equal(t, path, info.Path)
	assert.False(t, info.Loaded)
	assert.True(t, info.LoadedAt.IsZero())
	assert.Empty(t, info.DatabaseType)
	assert.Zero(t, info.BuildEpoch)
}

func TestGeoIPServiceInfo_LoadRecordsMetadataAndClock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	testutil.WriteGeoIPCountryDB(t, path, 1_700_000_000, testutil.WithGeoIPDatabaseType("GeoLite2-Country"))
	at := time.Date(2026, 11, 10, 5, 0, 1, 0, time.UTC)

	svc := NewUnloadedGeoIPService(path)
	svc.now = fixedClock(at)
	require.NoError(t, svc.Load())

	info := svc.Info()
	assert.Equal(t, path, info.Path)
	assert.True(t, info.Loaded)
	assert.Equal(t, at, info.LoadedAt)
	assert.Equal(t, "GeoLite2-Country", info.DatabaseType)
	assert.Equal(t, uint(1_700_000_000), info.BuildEpoch)
}

func TestGeoIPServiceInfo_FailedLoadKeepsDatabaseInUse(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.mmdb")
	testutil.WriteGeoIPCountryDB(t, good, 1_700_000_000, testutil.WithGeoIPDatabaseType("GeoLite2-Country"))
	loadedAt := time.Date(2026, 11, 10, 5, 0, 0, 0, time.UTC)

	svc := NewUnloadedGeoIPService(good)
	svc.now = fixedClock(loadedAt)
	require.NoError(t, svc.LoadFrom(good))
	before := svc.Info()

	badPath := filepath.Join(dir, "bad.mmdb")
	require.NoError(t, os.WriteFile(badPath, []byte("not a database"), 0o600))
	svc.now = fixedClock(loadedAt.Add(time.Hour))
	require.Error(t, svc.LoadFrom(badPath))

	assert.Equal(t, before, svc.Info(), "a failed load must not replace the reported metadata")
}

func TestGeoIPServiceInfo_CloseClearsMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	testutil.WriteGeoIPCountryDB(t, path, 1_700_000_000)

	svc := NewUnloadedGeoIPService(path)
	require.NoError(t, svc.Load())
	require.NoError(t, svc.Close())

	info := svc.Info()
	assert.False(t, info.Loaded)
	assert.True(t, info.LoadedAt.IsZero())
	assert.Empty(t, info.DatabaseType)
	assert.Zero(t, info.BuildEpoch)
}

func TestGeoIPServiceInfo_ZeroValueUsesWallClock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	testutil.WriteGeoIPCountryDB(t, path, 1)

	svc := &GeoIPService{dbPath: path}
	before := time.Now()
	require.NoError(t, svc.Load())

	assert.False(t, svc.Info().LoadedAt.Before(before))
}
