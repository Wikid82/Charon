package routes

import (
	"os"

	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// initGeoIPAtStartup builds the GeoIP service at the database path resolve returns, loads it when the file exists, and
// reports the image build marker. The service always exists, so a database provided later is loaded by the reload
// endpoint and every consumer (including the proxy state) sees the same live service.
func initGeoIPAtStartup(resolve func() string) *services.GeoIPService {
	svc := services.NewResolvingGeoIPService(resolve)
	loadGeoIPAtStartup(svc, svc.GetDatabasePath())
	reportGeoIPBuildMarker(svc.GetDatabasePath(), svc.IsLoaded())
	return svc
}

// loadGeoIPAtStartup loads the GeoIP database at geoipPath into svc, falling back to the bundled copy when the
// data-folder copy fails to load. See loadGeoIPWithFallback.
func loadGeoIPAtStartup(svc *services.GeoIPService, geoipPath string) {
	loadGeoIPWithFallback(svc, geoipPath, geoip.DataDatabasePath(), geoip.BundledDatabasePath())
}

// loadGeoIPWithFallback loads the database at path into svc when the file exists. When path is the data-folder copy
// (dataPath) and it fails to load, the bundled copy is tried next and a Warn names the file that failed. A missing or
// unreadable file that is not the data-folder copy is logged and leaves svc unloaded, so country lists are refused
// until a database is provided and reloaded.
func loadGeoIPWithFallback(svc *services.GeoIPService, path, dataPath, bundledPath string) {
	if _, err := os.Stat(path); err != nil { //nolint:gosec // G703: resolved database path, read-only stat
		logger.Log().WithField("path", path).Info("GeoIP database not found - geo-blocking features will be unavailable")
		return
	}
	err := svc.LoadFrom(path)
	if err == nil {
		logger.Log().WithField("path", path).Info("GeoIP database loaded successfully")
		return
	}
	if path == dataPath && bundledPath != path {
		logger.Log().WithError(err).WithField("path", path).Warn("GeoIP database in the data folder failed to load; falling back to the bundled copy")
		path = bundledPath
		if err = svc.LoadFrom(path); err == nil {
			logger.Log().WithField("path", path).Info("GeoIP database loaded successfully")
			return
		}
	}
	logger.Log().WithError(err).WithField("path", path).Warn("Failed to load GeoIP database - geo-blocking features will be unavailable")
}
