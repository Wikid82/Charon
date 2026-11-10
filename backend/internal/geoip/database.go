// Package geoip holds the GeoIP database facts shared by the API and the proxy config generator. It has no
// dependencies on other Charon packages, so both can import it.
package geoip

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// EnvDBPath names the environment variable that sets the database path explicitly. When it is set and not
	// empty, it is used exactly and no file location is searched.
	EnvDBPath = "CHARON_GEOIP_DB_PATH"

	// DatabaseFileName is the MaxMind database file name used in every location.
	DatabaseFileName = "GeoLite2-Country.mmdb"

	// DataDatabaseDir is the GeoIP folder inside the data volume. An operator-provided or persisted database goes here.
	DataDatabaseDir = "/app/data/geoip"

	// BundledDatabaseDir holds the database the image ships. It sits outside the data volume, so a volume mounted
	// over /app/data does not hide it.
	BundledDatabaseDir = "/app/geoip"

	// MissingMarkerName is the file the image build writes beside the database when it could not fetch it.
	MissingMarkerName = ".missing"
)

// ResolveDatabasePath returns the database path to use. CHARON_GEOIP_DB_PATH wins when it is set. Otherwise a
// database in the data folder is preferred, so an operator-provided file takes effect over the bundled copy. Without
// either, the bundled path is returned, and it reports as missing until a database is provided there.
func ResolveDatabasePath() string {
	return resolveDatabasePath(os.Getenv(EnvDBPath), isRegularFile)
}

// resolveDatabasePath implements ResolveDatabasePath with the environment value and file check injected.
func resolveDatabasePath(envPath string, exists func(path string) bool) string {
	if envPath != "" {
		return envPath
	}
	dataPath := filepath.Join(DataDatabaseDir, DatabaseFileName)
	if exists(dataPath) {
		return dataPath
	}
	return filepath.Join(BundledDatabaseDir, DatabaseFileName)
}

// isRegularFile reports whether path names an existing regular file. The path is cleaned first; it is either a fixed
// database location or an operator-configured one, and only a read-only stat is made.
func isRegularFile(path string) bool {
	info, err := os.Stat(filepath.Clean(path)) //nolint:gosec // G703: database path, read-only stat
	return err == nil && info.Mode().IsRegular()
}

// DatabaseAvailable reports whether the GeoIP database at path can serve lookups. It is available when the GeoIP
// service has loaded it (loaded), the path names a regular file, and the file name ends in .mmdb.
func DatabaseAvailable(path string, loaded bool) bool {
	if !loaded || path == "" || !strings.HasSuffix(filepath.Base(path), ".mmdb") {
		return false
	}
	return isRegularFile(path)
}

// MissingMarkerPath returns the build-time marker path for the database at dbPath. The marker sits in the same
// directory as the database.
func MissingMarkerPath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), MissingMarkerName)
}
