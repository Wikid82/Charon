// Package geoip holds the GeoIP database facts shared by the API and the proxy config generator. Apart from the logger,
// it has no dependencies on other Charon packages, so both can import it.
package geoip

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oschwald/geoip2-golang/v2"

	"github.com/Wikid82/charon/backend/internal/logger"
)

const (
	// EnvDBPath names the environment variable that sets the database path explicitly. When it is set and not
	// empty, it is used exactly and no file location is searched, with one exception: see FormerImageDefaultDBPath.
	EnvDBPath = "CHARON_GEOIP_DB_PATH"

	// FormerImageDefaultDBPath is the value earlier images set for EnvDBPath. Containers whose configuration was
	// copied from an earlier container may still carry it. When the file it names is not a regular file, the variable
	// is treated as unset.
	FormerImageDefaultDBPath = "/app/data/geoip/GeoLite2-Country.mmdb"

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

// DataDatabasePath returns the database path in the data folder.
func DataDatabasePath() string {
	return filepath.Join(DataDatabaseDir, DatabaseFileName)
}

// BundledDatabasePath returns the database path the image ships.
func BundledDatabasePath() string {
	return filepath.Join(BundledDatabaseDir, DatabaseFileName)
}

// resolution is the outcome of choosing a database path, with the notes the caller reports once.
type resolution struct {
	path string
	// ignoredFormerDefault is set when EnvDBPath named FormerImageDefaultDBPath, no file was there, and the variable
	// was treated as unset.
	ignoredFormerDefault bool
	// skippedUnreadable names the data-folder copy that was passed over because it could not be read while the
	// bundled copy could.
	skippedUnreadable string
}

var (
	reportIgnoredFormerDefault sync.Once
	reportSkippedUnreadable    sync.Once
)

// ResolveDatabasePath returns the database path to use. CHARON_GEOIP_DB_PATH wins when it is set, except that the
// former image default is treated as unset when no file is there. Otherwise, when both the data-folder copy and the
// bundled copy exist, the one with the newer MaxMind build time wins; a tie or a copy that cannot be read is resolved
// in favour of the readable one, and the data-folder copy otherwise. Without either, the bundled path is returned, and
// it reports as missing until a database is provided there.
func ResolveDatabasePath() string {
	r := resolveDatabasePath(os.Getenv(EnvDBPath), isRegularFile, readBuildEpoch)
	if r.ignoredFormerDefault {
		reportIgnoredFormerDefault.Do(func() {
			logger.Log().WithField("path", FormerImageDefaultDBPath).Info("CHARON_GEOIP_DB_PATH names the former image default, which is not a file; treating it as unset")
		})
	}
	if r.skippedUnreadable != "" {
		reportSkippedUnreadable.Do(func() {
			logger.Log().WithField("path", r.skippedUnreadable).Warn("GeoIP database in the data folder cannot be read; using the bundled copy instead")
		})
	}
	return r.path
}

// resolveDatabasePath implements ResolveDatabasePath with the environment value, the file check and the build-time
// reader injected.
func resolveDatabasePath(envPath string, isFile func(string) bool, buildEpoch func(string) (uint, error)) resolution {
	var r resolution
	if envPath == FormerImageDefaultDBPath && !isFile(envPath) {
		envPath = ""
		r.ignoredFormerDefault = true
	}
	if envPath != "" {
		r.path = envPath
		return r
	}

	dataPath := DataDatabasePath()
	bundledPath := BundledDatabasePath()
	if !isFile(dataPath) {
		r.path = bundledPath
		return r
	}
	if !isFile(bundledPath) {
		r.path = dataPath
		return r
	}

	dataEpoch, dataErr := buildEpoch(dataPath)
	bundledEpoch, bundledErr := buildEpoch(bundledPath)
	switch {
	case dataErr != nil && bundledErr == nil:
		r.path = bundledPath
		r.skippedUnreadable = dataPath
	case dataErr == nil && bundledErr != nil:
		r.path = dataPath
	case dataErr != nil && bundledErr != nil:
		// Neither copy can be read; the data-folder copy keeps its place so the failure is reported against it.
		r.path = dataPath
	case bundledEpoch > dataEpoch:
		r.path = bundledPath
	default:
		r.path = dataPath
	}
	return r
}

// readBuildEpoch returns the MaxMind build time, in Unix seconds, of the database at path. Opening the file reads its
// metadata; the database is closed again straight away.
func readBuildEpoch(path string) (uint, error) {
	db, err := geoip2.Open(filepath.Clean(path)) //nolint:gosec // G703: database path, read-only open
	if err != nil {
		return 0, fmt.Errorf("open geoip database %s: %w", path, err)
	}
	defer func() { _ = db.Close() }()
	return db.Metadata().BuildEpoch, nil
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
