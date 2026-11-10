// Package geoip holds the GeoIP database facts shared by the API and the proxy config generator. It has no
// dependencies on other Charon packages, so both can import it.
package geoip

import (
	"os"
	"path/filepath"
	"strings"
)

// MissingMarkerName is the file the image build writes beside the database when it could not fetch it.
const MissingMarkerName = ".missing"

// DatabaseAvailable reports whether the GeoIP database at path can serve lookups. It is available when the GeoIP
// service has loaded it (loaded), the path names a regular file, and the file name ends in .mmdb.
func DatabaseAvailable(path string, loaded bool) bool {
	if !loaded || path == "" || !strings.HasSuffix(filepath.Base(path), ".mmdb") {
		return false
	}
	info, err := os.Stat(filepath.Clean(path)) //nolint:gosec // G703: operator-configured path, read-only stat
	return err == nil && info.Mode().IsRegular()
}

// MissingMarkerPath returns the build-time marker path for the database at dbPath. The marker sits in the same
// directory as the database.
func MissingMarkerPath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), MissingMarkerName)
}
