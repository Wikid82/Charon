package routes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

func TestLoadGeoIPAtStartup_MissingFileLeavesServiceUnloaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	svc := services.NewUnloadedGeoIPService(path)

	loadGeoIPAtStartup(svc, path)

	require.False(t, svc.IsLoaded())
}

func TestLoadGeoIPAtStartup_UnreadableFileLeavesServiceUnloaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(path, []byte("not a database"), 0o600))
	svc := services.NewUnloadedGeoIPService(path)

	loadGeoIPAtStartup(svc, path)

	require.False(t, svc.IsLoaded())
}

func TestLoadGeoIPWithFallback_LoadsReadableDataCopy(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "data", "GeoLite2-Country.mmdb")
	bundledPath := filepath.Join(dir, "bundled", "GeoLite2-Country.mmdb")
	require.NoError(t, os.MkdirAll(filepath.Dir(dataPath), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Dir(bundledPath), 0o700))
	testutil.WriteGeoIPCountryDB(t, dataPath, 1)
	testutil.WriteGeoIPCountryDB(t, bundledPath, 2)
	svc := services.NewUnloadedGeoIPService(dataPath)

	loadGeoIPWithFallback(svc, dataPath, dataPath, bundledPath)

	require.True(t, svc.IsLoaded())
	require.Equal(t, dataPath, svc.GetDatabasePath())
}

func TestLoadGeoIPWithFallback_UnreadableDataCopyUsesBundledCopy(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "data", "GeoLite2-Country.mmdb")
	bundledPath := filepath.Join(dir, "bundled", "GeoLite2-Country.mmdb")
	require.NoError(t, os.MkdirAll(filepath.Dir(bundledPath), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Dir(dataPath), 0o700))
	require.NoError(t, os.WriteFile(dataPath, []byte("not a database"), 0o600))
	testutil.WriteGeoIPCountryDB(t, bundledPath, 2)
	svc := services.NewUnloadedGeoIPService(dataPath)

	loadGeoIPWithFallback(svc, dataPath, dataPath, bundledPath)

	require.True(t, svc.IsLoaded())
	require.Equal(t, bundledPath, svc.GetDatabasePath())
}

func TestLoadGeoIPWithFallback_UnreadableDataAndBundledLeavesUnloaded(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "data", "GeoLite2-Country.mmdb")
	bundledPath := filepath.Join(dir, "bundled", "GeoLite2-Country.mmdb")
	require.NoError(t, os.MkdirAll(filepath.Dir(dataPath), 0o700))
	require.NoError(t, os.WriteFile(dataPath, []byte("not a database"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Dir(bundledPath), 0o700))
	require.NoError(t, os.WriteFile(bundledPath, []byte("not a database"), 0o600))
	svc := services.NewUnloadedGeoIPService(dataPath)

	loadGeoIPWithFallback(svc, dataPath, dataPath, bundledPath)

	require.False(t, svc.IsLoaded())
}

func TestLoadGeoIPWithFallback_ExplicitPathDoesNotFallBack(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "custom", "GeoLite2-Country.mmdb")
	dataPath := filepath.Join(dir, "data", "GeoLite2-Country.mmdb")
	bundledPath := filepath.Join(dir, "bundled", "GeoLite2-Country.mmdb")
	require.NoError(t, os.MkdirAll(filepath.Dir(explicit), 0o700))
	require.NoError(t, os.WriteFile(explicit, []byte("not a database"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Dir(bundledPath), 0o700))
	testutil.WriteGeoIPCountryDB(t, bundledPath, 2)
	svc := services.NewUnloadedGeoIPService(explicit)

	loadGeoIPWithFallback(svc, explicit, dataPath, bundledPath)

	require.False(t, svc.IsLoaded(), "an explicitly configured path is used exactly")
	require.Equal(t, explicit, svc.GetDatabasePath())
}

func TestInitGeoIPAtStartup_UsesEnvironmentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	t.Setenv(geoip.EnvDBPath, path)

	svc := initGeoIPAtStartup(geoip.ResolveDatabasePath)

	require.Equal(t, path, svc.GetDatabasePath())
	require.False(t, svc.IsLoaded(), "no file at the configured path")
}

func TestInitGeoIPAtStartup_DefaultsToImageDatabaseLocation(t *testing.T) {
	t.Setenv(geoip.EnvDBPath, "")

	svc := initGeoIPAtStartup(geoip.ResolveDatabasePath)

	require.Equal(t, geoip.DatabaseFileName, filepath.Base(svc.GetDatabasePath()))
}
