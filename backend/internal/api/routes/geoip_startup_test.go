package routes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/services"
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
