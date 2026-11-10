package routes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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
