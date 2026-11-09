package routes

import (
	"os"

	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// reportGeoIPBuildMarker logs one Info line when the image build could not bundle the GeoIP database. It reports
// whether the marker was present. loaded is whether the GeoIP service loaded a database at startup.
func reportGeoIPBuildMarker(dbPath string, loaded bool) bool {
	if _, err := os.Stat(geoip.MissingMarkerPath(dbPath)); err != nil { //nolint:gosec // G703: marker path derived from CHARON_GEOIP_DB_PATH, read-only stat
		return false
	}
	logger.Log().WithField("path", dbPath).WithField("database_loaded", loaded).Info(geoIPBuildMarkerMessage(loaded))
	return true
}

// geoIPBuildMarkerMessage describes the GeoIP state for the build-marker Info line. Country lists are refused only
// while no database is loaded.
func geoIPBuildMarkerMessage(loaded bool) string {
	if loaded {
		return "Image was built without the GeoIP database; the database loaded from CHARON_GEOIP_DB_PATH is in use"
	}
	return "Image was built without the GeoIP database; country access lists stay refused until one is provided at CHARON_GEOIP_DB_PATH"
}

// geoIPProxyState returns the source of the GeoIP database state the proxy config reads at each apply. The database is
// usable when the GeoIP service has loaded it and the file is a database the proxy module can open.
func geoIPProxyState(path string, svc *services.GeoIPService) func() (string, bool) {
	return func() (string, bool) {
		return path, geoip.DatabaseAvailable(path, svc != nil && svc.IsLoaded())
	}
}
