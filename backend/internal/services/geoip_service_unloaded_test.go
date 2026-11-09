package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewUnloadedGeoIPService_LoadsOnceFileAppears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	svc := NewUnloadedGeoIPService(path)
	require.False(t, svc.IsLoaded())
	require.Equal(t, path, svc.GetDatabasePath())

	require.Error(t, svc.Load(), "a missing database file cannot be loaded")
	require.False(t, svc.IsLoaded())

	require.NoError(t, os.WriteFile(path, []byte("not a database"), 0o600))
	require.Error(t, svc.Load(), "a file the reader cannot open is not loaded")
	require.False(t, svc.IsLoaded())
}
