package crowdsec

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// A hub preset apply does not copy the installed hub tree into its snapshot: the tree is a regenerable cache
// that can reach tens of megabytes. The snapshot instead carries a small sidecar directory next to the copied
// configuration, holding a manifest of the hub tree (paths and sizes, no content) and a copy of the one hub file
// that an install may overwrite in place: the top-level item's own file. Restoring such a snapshot keeps the hub
// tree where it is, puts the configuration back, deletes every hub entry the manifest does not list and puts the
// saved item file back.
//
// Consequence: a rollback restores configuration and removes hub files that were added during the apply; it
// cannot return other hub files that an install overwrote (for example a newer dependency) to their earlier
// content. Those are logged.

const (
	// snapshotSidecarDir is the top-level directory of a light snapshot that holds the manifest and the item file.
	snapshotSidecarDir = ".charon-apply"
	hubManifestName    = "hub-manifest.json"
	// sidecarItemDir holds the saved item file below the same relative path it has in the data directory.
	sidecarItemDir = "item"
)

// hubRoots are the hub trees below the data directory (see IsHubPath).
var hubRoots = []string{"hub", filepath.Join("config", "hub")}

// hubManifestEntry is one entry of the hub tree at snapshot time.
type hubManifestEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Dir  bool   `json:"dir,omitempty"`
}

// isLightSnapshot reports whether the snapshot was taken without the hub tree.
func isLightSnapshot(snapshotPath string) bool {
	info, err := os.Lstat(filepath.Join(snapshotPath, snapshotSidecarDir))
	return err == nil && info.IsDir()
}

// keepForLightRestore decides which entries a light restore leaves alone: engine-owned state and the hub tree.
func keepForLightRestore(rel string) bool {
	return IsEngineOwnedPath(rel) || IsHubPath(rel)
}

// snapshotWithoutHub is Snapshot for a hub apply: the hub tree is left out and described by a manifest, and
// itemFile (the top-level item's own hub file, as cscli reports it; may be empty) is saved next to it.
func snapshotWithoutHub(dataDir, itemFile string) (string, error) {
	return snapshotTree(dataDir, keepForLightRestore, func(path string) error {
		return writeSnapshotSidecar(dataDir, path, itemFile)
	})
}

// writeSnapshotSidecar records the hub manifest and the item file in the snapshot directory path.
func writeSnapshotSidecar(dataDir, path, itemFile string) error {
	sidecar := filepath.Join(path, snapshotSidecarDir)
	if err := os.Mkdir(sidecar, 0o700); err != nil {
		return fmt.Errorf("mkdir snapshot sidecar: %w", err)
	}
	manifest, err := listHubTrees(dataDir)
	if err != nil {
		return err
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode hub manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sidecar, hubManifestName), data, 0o600); err != nil {
		return fmt.Errorf("write hub manifest: %w", err)
	}
	rel, ok := hubFileRel(dataDir, itemFile)
	if !ok {
		return nil
	}
	dst := filepath.Join(sidecar, sidecarItemDir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("mkdir saved item path: %w", err)
	}
	return copyFile(filepath.Join(dataDir, rel), dst)
}

// hubFileRel returns the path of file relative to dataDir when it names an existing regular file inside the hub
// tree. Anything else (empty, outside the data directory, a link, missing) yields false.
func hubFileRel(dataDir, file string) (string, bool) {
	if file == "" {
		return "", false
	}
	rel, err := filepath.Rel(filepath.Clean(dataDir), filepath.Clean(file))
	if err != nil || !filepath.IsLocal(rel) || !IsHubPath(rel) {
		return "", false
	}
	info, err := os.Lstat(filepath.Join(dataDir, rel))
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return rel, true
}

// listHubTrees lists every entry of the hub trees below dataDir.
func listHubTrees(dataDir string) ([]hubManifestEntry, error) {
	var manifest []hubManifestEntry
	err := walkHubTrees(dataDir, func(rel string, d fs.DirEntry) error {
		info, err := d.Info()
		if err != nil {
			return err //nolint:wrapcheck // wrapped by walkHubTrees
		}
		manifest = append(manifest, hubManifestEntry{Path: rel, Size: info.Size(), Dir: d.IsDir()})
		return nil
	})
	return manifest, err
}

// walkHubTrees calls visit for every entry of the hub trees that exist below dataDir. Symlinks are never
// followed. visit may return fs.SkipDir.
func walkHubTrees(dataDir string, visit func(rel string, d fs.DirEntry) error) error {
	for _, root := range hubRoots {
		if _, err := os.Lstat(filepath.Join(dataDir, root)); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		err := filepath.WalkDir(filepath.Join(dataDir, root), func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, relErr := filepath.Rel(dataDir, path)
			if relErr != nil {
				return relErr //nolint:wrapcheck // wrapped below
			}
			return visit(rel, d)
		})
		if err != nil {
			return fmt.Errorf("walk hub tree %s: %w", root, err)
		}
	}
	return nil
}

// restoreWithoutHub restores a light snapshot into dataDir; see the comment at the top of this file.
func restoreWithoutHub(snapshotPath, dataDir string) error {
	data, err := os.ReadFile(filepath.Join(snapshotPath, snapshotSidecarDir, hubManifestName)) // #nosec G304 -- snapshotPath is server-resolved
	if err != nil {
		return fmt.Errorf("read hub manifest: %w", err)
	}
	var manifest []hubManifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse hub manifest: %w", err)
	}
	keep := func(rel string) bool { return keepForLightRestore(rel) || rel == snapshotSidecarDir }
	if err := RestoreKeeping(snapshotPath, dataDir, keep); err != nil {
		return err
	}
	if err := pruneHubToManifest(dataDir, manifest); err != nil {
		return err
	}
	return restoreSavedItem(snapshotPath, dataDir)
}

// pruneHubToManifest deletes every hub entry that the manifest does not list, so a failed install leaves no
// orphan files or directories. Listed files whose size changed cannot be put back and are logged.
func pruneHubToManifest(dataDir string, manifest []hubManifestEntry) error {
	listed := make(map[string]hubManifestEntry, len(manifest))
	for _, e := range manifest {
		listed[e.Path] = e
	}
	changed := 0
	err := walkHubTrees(dataDir, func(rel string, d fs.DirEntry) error {
		entry, ok := listed[rel]
		if !ok {
			if err := os.RemoveAll(filepath.Join(dataDir, rel)); err != nil {
				return err //nolint:wrapcheck // wrapped by walkHubTrees
			}
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil && info.Size() != entry.Size {
				changed++
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if changed > 0 {
		logger.Log().WithField("files", changed).Warn("preset rollback could not restore hub files changed during the apply")
	}
	return nil
}

// restoreSavedItem puts the saved top-level item file back, replacing whatever the apply left at its path.
func restoreSavedItem(snapshotPath, dataDir string) error {
	saved := filepath.Join(snapshotPath, snapshotSidecarDir, sidecarItemDir)
	if _, err := os.Lstat(saved); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	var files []string
	if err := filepath.WalkDir(saved, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && d.Type().IsRegular() {
			files = append(files, path)
		}
		return walkErr
	}); err != nil {
		return fmt.Errorf("read saved item: %w", err)
	}
	for _, src := range files {
		rel, err := filepath.Rel(saved, src)
		if err != nil {
			return fmt.Errorf("saved item path: %w", err)
		}
		dst := filepath.Join(dataDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return fmt.Errorf("mkdir saved item path: %w", err)
		}
		// Remove first so a link left at the path is replaced instead of written through.
		if err := os.Remove(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("replace saved item: %w", err)
		}
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("restore saved item: %w", err)
		}
	}
	return nil
}
