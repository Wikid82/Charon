package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Wikid82/charon/backend/internal/geoip"
)

// Sentinel errors of the GeoIP database store. Handlers map them to status codes and stable error codes; the wrapped
// detail is for the server log only.
var (
	// ErrGeoIPBusy is returned when another change to the database is already running.
	ErrGeoIPBusy = errors.New("geoip database change already in progress")
	// ErrGeoIPReloadFailed is returned when the database file could not be loaded.
	ErrGeoIPReloadFailed = errors.New("geoip database reload failed")
	// ErrGeoIPApplyFailed is returned when the proxy config could not be applied after the database changed.
	ErrGeoIPApplyFailed = errors.New("geoip proxy apply failed")
)

// Replace-blocked reasons reported by GeoIPStatus.ReplaceBlockedReason. An empty reason means replacing is possible.
const (
	ReplaceBlockedCustomPath   = "custom_path_set"
	ReplaceBlockedReadOnlyData = "data_folder_read_only"
)

// GeoIPStatus is the database card's view of the GeoIP database: where it comes from, what it is, and whether the
// operator can replace it. Its JSON names are the API contract; nullable fields are null when unknown.
type GeoIPStatus struct {
	Source               string  `json:"source"`
	DatabaseType         string  `json:"database_type"`
	BuildDate            *string `json:"build_date"`
	SizeBytes            *int64  `json:"size_bytes"`
	LoadedAt             *string `json:"loaded_at"`
	IncludedBuildDate    *string `json:"included_build_date"`
	DataCopyPresent      bool    `json:"data_copy_present"`
	CanReplace           bool    `json:"can_replace"`
	ReplaceBlockedReason string  `json:"replace_blocked_reason"`
	UnenforcedListsTotal int     `json:"unenforced_lists_total"`
}

// GeoIPDatabaseStoreConfig wires a GeoIPDatabaseStore. The paths are full file paths.
type GeoIPDatabaseStoreConfig struct {
	// Service is the lookup service whose database the store loads and reports.
	Service *GeoIPService
	// Apply applies the proxy config after the database changed. Nil when there is no proxy to apply to.
	Apply func(context.Context) error
	// DataPath is the data-folder copy that uploads will replace.
	DataPath string
	// BundledPath is the copy the image ships.
	BundledPath string
	// EnvInEffect reports whether CHARON_GEOIP_DB_PATH selects the database. Nil means geoip.EnvPathInEffect.
	EnvInEffect func() bool
}

// GeoIPDatabaseStore owns the changes to the GeoIP database and the status the card shows. Every mutating method runs
// under mu, acquired with TryLock: a request that finds another change running gets ErrGeoIPBusy at once instead of
// queueing behind it.
type GeoIPDatabaseStore struct {
	svc         *GeoIPService
	apply       func(context.Context) error
	dataPath    string
	bundledPath string
	envInEffect func() bool
	// writable reports whether the folder accepts writes. It is writableFolder; tests replace it.
	writable func(dir string) bool

	mu sync.Mutex

	// includedMu guards included, the cached build epoch of the bundled copy.
	includedMu sync.Mutex
	included   includedCopy
}

// includedCopy caches the build epoch of the bundled copy, keyed on the file's path, size and modification time.
type includedCopy struct {
	path    string
	size    int64
	modTime time.Time
	epoch   uint
	known   bool
	valid   bool
}

// NewGeoIPDatabaseStore returns a store over the service and the configured copies.
func NewGeoIPDatabaseStore(cfg GeoIPDatabaseStoreConfig) *GeoIPDatabaseStore {
	envInEffect := cfg.EnvInEffect
	if envInEffect == nil {
		envInEffect = geoip.EnvPathInEffect
	}
	return &GeoIPDatabaseStore{
		svc:         cfg.Service,
		apply:       cfg.Apply,
		dataPath:    cfg.DataPath,
		bundledPath: cfg.BundledPath,
		envInEffect: envInEffect,
		writable:    writableFolder,
	}
}

// Status describes the database in use and whether it can be replaced. UnenforcedListsTotal is left zero here; the
// caller that owns the access lists fills it in.
func (s *GeoIPDatabaseStore) Status() GeoIPStatus {
	info := s.svc.Info()
	envInEffect := s.envInEffect()

	st := GeoIPStatus{
		Source:            string(geoip.SourceNone),
		IncludedBuildDate: s.includedBuildDate(),
		DataCopyPresent:   isRegularFile(s.dataPath),
	}
	if info.Loaded {
		st.Source = string(geoip.SourceOf(info.Path, envInEffect))
		st.DatabaseType = info.DatabaseType
		st.BuildDate = formatEpochUTC(info.BuildEpoch)
		st.LoadedAt = formatTimeUTC(info.LoadedAt)
		st.SizeBytes = fileSize(info.Path)
	}
	st.CanReplace, st.ReplaceBlockedReason = s.replaceability(envInEffect)
	return st
}

// replaceability reports whether an upload or a removal can change the data folder, and the reason when it cannot.
// A custom path wins over folder permissions because the environment variable decides the database regardless.
func (s *GeoIPDatabaseStore) replaceability(envInEffect bool) (canReplace bool, blockedReason string) {
	switch {
	case envInEffect:
		return false, ReplaceBlockedCustomPath
	case !s.writable(filepath.Dir(s.dataPath)):
		return false, ReplaceBlockedReadOnlyData
	default:
		return true, ""
	}
}

// Reload loads the database file again from its location and applies the proxy config. The load runs even when the
// apply will follow, and the apply runs even when the load failed, so the proxy converges on the database state it can
// see. The returned error wraps ErrGeoIPReloadFailed, ErrGeoIPApplyFailed, or both; ErrGeoIPBusy means nothing ran.
func (s *GeoIPDatabaseStore) Reload(ctx context.Context) error {
	if !s.mu.TryLock() {
		return ErrGeoIPBusy
	}
	defer s.mu.Unlock()

	loadErr := s.svc.Load()
	var applyErr error
	if s.apply != nil {
		applyErr = s.apply(ctx)
	}
	return errors.Join(
		wrapFailure(ErrGeoIPReloadFailed, loadErr),
		wrapFailure(ErrGeoIPApplyFailed, applyErr),
	)
}

// wrapFailure wraps err with sentinel, or returns nil when err is nil.
func wrapFailure(sentinel, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", sentinel, err)
}

// includedBuildDate returns the bundled copy's build date, or nil when that copy is absent or unreadable.
func (s *GeoIPDatabaseStore) includedBuildDate() *string {
	epoch, known := s.includedBuildEpoch()
	if !known {
		return nil
	}
	return formatEpochUTC(epoch)
}

// includedBuildEpoch reads the bundled copy's build epoch. The read is cached on (path, size, modification time), so
// polling the status does not reopen the file each time.
func (s *GeoIPDatabaseStore) includedBuildEpoch() (uint, bool) {
	info, err := os.Stat(filepath.Clean(s.bundledPath))
	if err != nil || !info.Mode().IsRegular() {
		return 0, false
	}

	s.includedMu.Lock()
	defer s.includedMu.Unlock()
	if s.included.valid && s.included.path == s.bundledPath &&
		s.included.size == info.Size() && s.included.modTime.Equal(info.ModTime()) {
		return s.included.epoch, s.included.known
	}

	db, err := geoip.Inspect(s.bundledPath)
	s.included = includedCopy{
		path:    s.bundledPath,
		size:    info.Size(),
		modTime: info.ModTime(),
		epoch:   db.BuildEpoch,
		known:   err == nil,
		valid:   true,
	}
	return s.included.epoch, s.included.known
}

// writableFolder reports whether writes to the GeoIP folder dir are possible. The folder is created on the first write,
// so when it does not exist yet its parent is probed instead. A probe that fails for any reason counts as not writable.
func writableFolder(dir string) bool {
	probe := filepath.Clean(dir)
	if _, err := os.Stat(probe); err != nil {
		probe = filepath.Dir(probe)
	}
	return unix.Access(probe, unix.W_OK) == nil
}

// isRegularFile reports whether path names an existing regular file.
func isRegularFile(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	return err == nil && info.Mode().IsRegular()
}

// fileSize returns the size of the file at path, or nil when it cannot be read.
func fileSize(path string) *int64 {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	size := info.Size()
	return &size
}

// formatEpochUTC renders a MaxMind build epoch as an RFC 3339 UTC time, or nil for an unknown (zero) epoch.
func formatEpochUTC(epoch uint) *string {
	if epoch == 0 {
		return nil
	}
	return formatTimeUTC(time.Unix(int64(epoch), 0))
}

// formatTimeUTC renders t as an RFC 3339 UTC time, or nil for the zero time.
func formatTimeUTC(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}
