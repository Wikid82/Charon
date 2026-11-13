package routes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportGeoIPBuildMarker_ReportsOnlyWhenMarkerPresent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")

	assert.False(t, reportGeoIPBuildMarker(dbPath, false), "no marker, nothing to report")

	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(dbPath), ".missing"), nil, 0o600))
	assert.True(t, reportGeoIPBuildMarker(dbPath, false))
	assert.True(t, reportGeoIPBuildMarker(dbPath, true), "marker is reported even when a database is provided later")
}

func TestSweepGeoIPDataFolder_ReportsWhetherTheFolderIsClean(t *testing.T) {
	dir := t.TempDir()
	assert.True(t, sweepGeoIPDataFolder(filepath.Join(dir, "geoip", "GeoLite2-Country.mmdb")), "a missing folder is clean")

	notAFolder := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(notAFolder, []byte("x"), 0o600))
	assert.False(t, sweepGeoIPDataFolder(filepath.Join(notAFolder, "GeoLite2-Country.mmdb")), "an unreadable folder is logged and startup goes on")
}
