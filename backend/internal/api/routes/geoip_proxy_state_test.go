package routes

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

func TestGeoIPProxyState_UnavailableWithoutService(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	state := geoIPProxyState(path, nil)

	gotPath, available := state()
	require.Equal(t, path, gotPath)
	require.False(t, available, "no service means no database for the proxy")
}

func TestGeoIPProxyState_UnavailableWhenServiceNotLoaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	state := geoIPProxyState(path, &services.GeoIPService{})

	gotPath, available := state()
	require.Equal(t, path, gotPath)
	require.False(t, available)
}
