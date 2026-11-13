package routes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

func TestGeoIPProxyState_UnavailableWithoutService(t *testing.T) {
	state := geoIPProxyState(nil)

	gotPath, available := state()
	require.Empty(t, gotPath)
	require.False(t, available, "no service means no database for the proxy")
}

func TestGeoIPProxyState_UnavailableWhenServiceNotLoaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	state := geoIPProxyState(services.NewUnloadedGeoIPService(path))

	gotPath, available := state()
	require.Equal(t, path, gotPath)
	require.False(t, available)
}

func TestGeoIPProxyState_FollowsPathResolvedOnLoad(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "absent.mmdb")
	svc := services.NewResolvingGeoIPService(func() string { return current })
	state := geoIPProxyState(svc)

	dropped := filepath.Join(dir, "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(dropped, []byte("not a database"), 0o600))
	current = dropped
	require.Error(t, svc.Load())

	gotPath, available := state()
	require.Equal(t, filepath.Join(dir, "absent.mmdb"), gotPath, "a failed reload keeps the path in use")
	require.False(t, available)
}
