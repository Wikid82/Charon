package crowdsec

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// Layout is where the live CrowdSec tree keeps its configuration.
type Layout int

const (
	// LayoutFlat is a tree with no configuration folder yet (fresh or never configured); both archive
	// layouts are acceptable.
	LayoutFlat Layout = iota
	// LayoutNested is a tree whose configuration lives in <DataDir>/config, the folder CrowdSec reads
	// (/etc/crowdsec is a link to it).
	LayoutNested
)

// ErrArchiveLayout is returned when an archive keeps its configuration where a nested installation
// does not read it.
var ErrArchiveLayout = errors.New("archive layout does not match the live configuration folder")

// DetectLayout reports the layout of the live tree below dataDir. It is nested when config/config.yaml
// exists or config/ holds at least one entry (a populated folder without config.yaml is still the folder
// CrowdSec reads). config/ is resolved by following links, because that is what /etc/crowdsec does.
// Edge cases:
//   - config/ with only hub/ or credentials: nested
//   - empty config/ directory, a regular file named config, or no config (also below a file): flat
//   - config/ unreadable, or a dangling config link: nested (fails safe: flat archives are refused and
//     nothing changes)
//   - config symlink to a populated directory: nested
//
// Callers hold the data lock; the result is never cached.
func DetectLayout(dataDir string) Layout {
	configDir := filepath.Join(filepath.Clean(dataDir), "config")
	if _, err := os.Stat(filepath.Join(configDir, "config.yaml")); err == nil {
		return LayoutNested
	}
	st, err := os.Stat(configDir)
	if err != nil {
		if _, lerr := os.Lstat(configDir); lerr == nil {
			logger.Log().Warn("crowdsec config folder is a dangling link; treating the layout as nested")
			return LayoutNested
		} else if errors.Is(lerr, os.ErrNotExist) || errors.Is(lerr, syscall.ENOTDIR) {
			return LayoutFlat
		}
		logger.Log().Warn("crowdsec config folder cannot be inspected; treating the layout as nested")
		return LayoutNested
	}
	if !st.IsDir() {
		return LayoutFlat
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		logger.Log().Warn("crowdsec config folder cannot be read; treating the layout as nested")
		return LayoutNested
	}
	if len(entries) == 0 {
		return LayoutFlat
	}
	return LayoutNested
}

// CheckImportLayout refuses an archive without config/config.yaml for a nested live tree.
func CheckImportLayout(live Layout, a ArchiveInfo) error {
	if live == LayoutNested && !a.HasNestedConfig {
		return ErrArchiveLayout
	}
	return nil
}
