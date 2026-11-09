package routes

import (
	"os"

	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// reportGeoIPBuildMarker logs one Info line when the image build could not bundle the GeoIP database. It reports
// whether the marker was present. loaded is whether the GeoIP service loaded a database at startup.
func reportGeoIPBuildMarker(dbPath string, loaded bool) bool {
	if _, err := os.Stat(geoip.MissingMarkerPath(dbPath)); err != nil { //nolint:gosec // G703: marker path derived from CHARON_GEOIP_DB_PATH, read-only stat
		return false
	}
	logger.Log().WithField("path", dbPath).WithField("database_loaded", loaded).Info(
		"Image was built without the GeoIP database; country access lists stay refused until one is provided at CHARON_GEOIP_DB_PATH",
	)
	return true
}
