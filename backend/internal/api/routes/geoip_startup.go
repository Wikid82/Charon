package routes

import (
	"os"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// initGeoIPAtStartup builds the GeoIP service at the database path resolve returns, loads it when the file exists, and
// reports the image build marker. The service always exists, so a database provided later is loaded by the reload
// endpoint and every consumer (including the proxy state) sees the same live service.
func initGeoIPAtStartup(resolve func() string) *services.GeoIPService {
	svc := services.NewResolvingGeoIPService(resolve)
	path := svc.GetDatabasePath()
	loadGeoIPAtStartup(svc, path)
	reportGeoIPBuildMarker(path, svc.IsLoaded())
	return svc
}

// loadGeoIPAtStartup loads the GeoIP database at geoipPath into svc when the file exists. A missing or unreadable file
// is logged and leaves svc unloaded, so country lists are refused until a database is provided and reloaded.
func loadGeoIPAtStartup(svc *services.GeoIPService, geoipPath string) {
	if _, err := os.Stat(geoipPath); err != nil { //nolint:gosec // G703: resolved database path, read-only stat
		logger.Log().WithField("path", geoipPath).Info("GeoIP database not found - geo-blocking features will be unavailable")
		return
	}
	if err := svc.Load(); err != nil {
		logger.Log().WithError(err).WithField("path", geoipPath).Warn("Failed to load GeoIP database - geo-blocking features will be unavailable")
		return
	}
	logger.Log().WithField("path", geoipPath).Info("GeoIP database loaded successfully")
}
