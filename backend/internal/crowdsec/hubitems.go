package crowdsec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	// MaxHubItems bounds the items one manifest, scan or restore request carries.
	MaxHubItems = 1000
	// maxHubScanEntries bounds how many directory entries one scan visits.
	maxHubScanEntries = 10 * MaxHubItems
)

// stagedHubTypes keep their installed links one directory deeper, below a processing stage such as
// s01-parse.
var stagedHubTypes = map[string]bool{"parsers": true, "postoverflows": true}

// InstalledHubItems lists the hub items installed below dataDir, sorted by type then name. An item is
// installed when a symlink under <type>/ (or config/<type>/) points into the hub tree at
// hub/<type>/[stage/]<owner>/<name>.yaml or .yml. Only the link text is read: no cscli, no network, and the
// target is not required to exist. Links that do not fit that shape (relative, pointing elsewhere, a
// name that fails ValidHubItemName) are skipped, real files are not hub items, and at most MaxHubItems
// are returned.
func InstalledHubItems(dataDir string) ([]HubItem, error) {
	items, _, err := scanHubItems(dataDir)
	return items, err
}

// scanHubItems is InstalledHubItems plus the number of links that were skipped.
func scanHubItems(dataDir string) (items []HubItem, skipped int, err error) {
	seen := map[HubItem]struct{}{}
	visited := 0
	for _, base := range []string{"config", ""} {
		for _, typ := range HubItemTypes {
			root := filepath.Join(dataDir, base, typ)
			walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
				if werr != nil {
					return handleHubScanError(root, p, d, werr)
				}
				visited++
				if visited > maxHubScanEntries {
					return fs.SkipAll
				}
				if d.Type()&fs.ModeSymlink == 0 {
					return nil
				}
				rel, relErr := filepath.Rel(root, p)
				if relErr != nil {
					return fmt.Errorf("hub item relative path: %w", relErr)
				}
				item, ok := hubItemFromLink(typ, rel, p)
				if !ok {
					skipped++
					return nil
				}
				seen[item] = struct{}{}
				return nil
			})
			if walkErr != nil {
				return nil, skipped, fmt.Errorf("scan %s: %w", typ, walkErr)
			}
		}
	}
	items = make([]HubItem, 0, len(seen))
	for item := range seen {
		items = append(items, item)
	}
	slices.SortFunc(items, compareHubItems)
	if len(items) > MaxHubItems {
		logger.Log().Warnf("hub item scan found %d items; keeping the first %d", len(items), MaxHubItems)
		items = items[:MaxHubItems]
	}
	if skipped > 0 {
		logger.Log().Debugf("hub item scan skipped %d links that are not installed hub items", skipped)
	}
	return items, skipped, nil
}

// handleHubScanError ignores a type directory that does not exist and an unreadable subdirectory;
// anything else (including an unreadable type directory) fails the scan.
func handleHubScanError(root, p string, d fs.DirEntry, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case d != nil && d.IsDir() && p != root && errors.Is(err, fs.ErrPermission):
		logger.Log().WithError(err).Warnf("skipping unreadable directory %s during hub item scan", util.SanitizeForLog(p))
		return fs.SkipDir
	default:
		return err
	}
}

// hubItemFromLink derives the item behind the symlink at p (rel below its type directory). It reports
// false for anything that is not a link into the hub tree of the same type.
func hubItemFromLink(typ, rel, p string) (HubItem, bool) {
	wantDepth := 1
	if stagedHubTypes[typ] {
		wantDepth = 2
	}
	if strings.Count(filepath.ToSlash(rel), "/")+1 != wantDepth {
		return HubItem{}, false
	}
	target, err := os.Readlink(p)
	if err != nil || !filepath.IsAbs(target) {
		return HubItem{}, false
	}
	item, ok := hubItemFromTarget(typ, filepath.ToSlash(filepath.Clean(target)))
	return item, ok && item.Valid()
}

// hubItemExtensions are the file suffixes hub items ship with. The hub uses both spellings, so every
// hub item check goes through hubItemStem.
var hubItemExtensions = []string{".yaml", ".yml"}

// hubItemStem returns the item name of a hub item file (the file name without its .yaml or .yml
// suffix) and reports whether file is a hub item file at all. The match is case sensitive, like the hub.
func hubItemStem(file string) (string, bool) {
	for _, ext := range hubItemExtensions {
		if stem, ok := strings.CutSuffix(file, ext); ok {
			return stem, true
		}
	}
	return "", false
}

// hubItemFromTarget parses a cleaned absolute link target ending in
// hub/<typ>/[stage/]<owner>/<name>.yaml (or .yml).
func hubItemFromTarget(typ, target string) (HubItem, bool) {
	parts := strings.Split(strings.TrimPrefix(target, "/"), "/")
	tail := 2 // owner/name.yaml
	if stagedHubTypes[typ] {
		tail = 3 // stage/owner/name.yaml
	}
	hubAt := len(parts) - tail - 2
	if hubAt < 0 || parts[hubAt] != "hub" || parts[hubAt+1] != typ {
		return HubItem{}, false
	}
	owner, file := parts[len(parts)-2], parts[len(parts)-1]
	name, found := hubItemStem(file)
	if !found {
		return HubItem{}, false
	}
	return HubItem{Type: typ, Name: owner + "/" + name}, true
}

// CountDownloadedHubFiles counts the item files (*.yaml and *.yml) below the hub trees of dataDir, whether or
// not the item is enabled yet. It is cheap (no cscli) and bounded like the installed-item scan.
func CountDownloadedHubFiles(dataDir string) int {
	n, visited := 0, 0
	for _, root := range hubRoots {
		for _, typ := range HubItemTypes {
			_ = filepath.WalkDir(filepath.Join(dataDir, root, typ), func(_ string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				visited++
				if visited > maxHubScanEntries {
					return fs.SkipAll
				}
				if _, isItem := hubItemStem(d.Name()); isItem && !d.IsDir() && d.Type().IsRegular() {
					n++
				}
				return nil
			})
		}
	}
	return n
}
