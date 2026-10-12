package crowdsec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// requiredCompanions are the files CrowdSec cannot start without (verified with `crowdsec -t` on
// v1.8.1) that an import clears with the rest of the configuration. The name "patterns" is a directory,
// the others are files.
var requiredCompanions = []string{"acquis.yaml", "patterns", "profiles.yaml", "simulation.yaml"}

const (
	profilesCompanion  = "profiles.yaml"
	patternsCompanion  = "patterns"
	notificationsDir   = "notifications"
	keptNotificationNS = notificationsDir + "/"
)

// ErrDuplicateNotifications is returned when restoring the notification files a kept profile needs
// leaves a notification name defined more than once.
var ErrDuplicateNotifications = errors.New("a restored notification file defines a name the archive already defines")

// RestoreMissingCompanions copies the required companions that an import left out back from the
// snapshot taken before it. prefix is the folder below dataDir that holds the configuration ("config"
// for a nested install, "" for a flat one). Absence is decided with Lstat, so a link counts as present
// (it is kept across an import) and is never replaced or written through. When profiles.yaml was kept,
// the notification files it names that the new tree no longer defines are restored too. It returns the
// sorted names that were restored, notification files as "notifications/<file>".
func RestoreMissingCompanions(snapshotDir, dataDir, prefix string) ([]string, error) {
	liveBase := filepath.Join(filepath.Clean(dataDir), prefix)
	snapBase := filepath.Join(filepath.Clean(snapshotDir), prefix)
	var kept []string
	for _, name := range requiredCompanions {
		restored, err := restoreCompanion(snapBase, liveBase, name)
		if err != nil {
			return nil, err
		}
		if restored {
			kept = append(kept, name)
		}
	}
	if containsName(kept, profilesCompanion) {
		files, err := restoreNeededNotifications(snapshotDir, dataDir, prefix)
		if err != nil {
			return nil, err
		}
		kept = append(kept, files...)
	}
	sort.Strings(kept)
	return kept, nil
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// restoreCompanion copies one companion from the snapshot when nothing exists under that name in the
// live tree. It reports whether a copy was made.
func restoreCompanion(snapBase, liveBase, name string) (bool, error) {
	live := filepath.Join(liveBase, name)
	if _, err := os.Lstat(live); err == nil {
		return false, nil // present in any form, including a link
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("inspect %s: %w", name, withoutPath(err))
	}
	snap := filepath.Join(snapBase, name)
	st, err := os.Lstat(snap)
	if treeEntryAbsent(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect snapshot %s: %w", name, withoutPath(err))
	}
	wantDir := name == patternsCompanion
	if st.Mode()&os.ModeSymlink != 0 || st.IsDir() != wantDir || (!wantDir && !st.Mode().IsRegular()) {
		logger.Log().WithField("entry", name).Warn("crowdsec import: not restoring a companion that is a link or has the wrong type")
		return false, nil
	}
	if wantDir {
		if err = os.Mkdir(live, 0o750); err != nil {
			return false, fmt.Errorf("create %s: %w", name, withoutPath(err))
		}
		if err = copyTree(snap, live, nil); err != nil {
			return false, fmt.Errorf("restore %s: %w", name, withoutPath(err))
		}
		return true, nil
	}
	if err = copyFile(snap, live); err != nil {
		return false, fmt.Errorf("restore %s: %w", name, withoutPath(err))
	}
	return true, nil
}

// restoreNeededNotifications copies back from the snapshot the notification files that define the names
// the kept profiles.yaml references and the new tree does not define. It never overwrites or writes
// through anything: a linked notifications folder, an existing destination path (even one holding the
// archive's own file of the same name), a linked snapshot file and a file outside the bounds all leave
// the name undefined for the caller's verification to refuse. It returns "notifications/<file>" entries.
func restoreNeededNotifications(snapshotDir, dataDir, prefix string) ([]string, error) {
	tree, err := InspectTree(dataDir)
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Warn("crowdsec import: cannot read the new tree to restore notification files")
		return nil, nil
	}
	missing := CheckProfileNotifications(tree)
	if len(missing) == 0 {
		return nil, nil
	}
	snap, err := InspectTree(snapshotDir)
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Warn("crowdsec import: cannot read the snapshot to restore notification files")
		return nil, nil
	}
	dir := path.Join(filepath.ToSlash(prefix), notificationsDir)
	byFile, ok := definedNotificationsByFile(snap.Files, dir)
	if !ok {
		logger.Log().Warn("crowdsec import: a snapshot notification file cannot be read; not restoring notification files")
		return nil, nil
	}
	needed := map[string]bool{}
	for _, n := range missing {
		needed[n] = true
	}
	files := make([]string, 0, len(byFile))
	for rel := range byFile {
		files = append(files, rel)
	}
	sort.Strings(files)

	destDir := filepath.Join(filepath.Clean(dataDir), prefix, notificationsDir)
	var restored []string
	for _, rel := range files {
		if !definesAny(byFile[rel], needed) {
			continue
		}
		if st, lerr := os.Lstat(filepath.Join(filepath.Clean(snapshotDir), filepath.FromSlash(rel))); lerr != nil || st.Mode()&os.ModeSymlink != 0 {
			logger.Log().WithField("file", util.SanitizeForLog(path.Base(rel))).Warn("crowdsec import: a snapshot notification file is a link or cannot be inspected; not restoring it")
			continue
		}
		ready, err := prepareNotificationsDir(destDir)
		if err != nil {
			return nil, err
		}
		if !ready {
			return restored, nil
		}
		wrote, err := writeNotificationFile(filepath.Join(destDir, path.Base(rel)), snap.Files[rel])
		if err != nil {
			return nil, err
		}
		if !wrote {
			continue
		}
		restored = append(restored, keptNotificationNS+path.Base(rel))
		for _, n := range byFile[rel] {
			delete(needed, n)
		}
	}
	return restored, nil
}

func definesAny(names []string, wanted map[string]bool) bool {
	for _, n := range names {
		if wanted[n] {
			return true
		}
	}
	return false
}

// prepareNotificationsDir makes sure dir is a plain directory, creating it when absent. It reports
// false (after logging) when dir is a link or not a directory.
func prepareNotificationsDir(dir string) (bool, error) {
	st, err := os.Lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err = os.Mkdir(dir, 0o750); err != nil {
			return false, fmt.Errorf("create notifications folder: %w", withoutPath(err))
		}
		return true, nil
	case err != nil:
		return false, fmt.Errorf("inspect notifications folder: %w", withoutPath(err))
	case !st.IsDir():
		logger.Log().Warn("crowdsec import: the notifications folder is a link or not a directory; not restoring notification files")
		return false, nil
	}
	return true, nil
}

// writeNotificationFile creates dest with data, never replacing anything: an existing path of any kind
// is skipped and logged. It reports whether the file was written.
func writeNotificationFile(dest string, data []byte) (bool, error) {
	if _, err := os.Lstat(dest); err == nil {
		logger.Log().WithField("file", util.SanitizeForLog(filepath.Base(dest))).Warn("crowdsec import: a notification file of that name already exists; not restoring it")
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("inspect notification file: %w", withoutPath(err))
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- dest is a direct child of the notifications folder, name taken from a bounded directory listing
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create notification file: %w", withoutPath(err))
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return false, fmt.Errorf("write notification file: %w", withoutPath(err))
	}
	if err = f.Close(); err != nil {
		return false, fmt.Errorf("close notification file: %w", withoutPath(err))
	}
	return true, nil
}

// VerifyKeptProfile checks the live tree after RestoreMissingCompanions: when profiles.yaml was kept it
// must not name a notification the tree has no file for (*UndefinedNotificationsError), and restored
// notification files must not define a name twice (ErrDuplicateNotifications). It fails open when the
// tree cannot be read completely.
func VerifyKeptProfile(dataDir string, kept []string) error {
	if !containsName(kept, profilesCompanion) {
		return nil
	}
	tree, err := InspectTree(dataDir)
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Warn("crowdsec import: kept profile check skipped")
		return nil
	}
	if err := VerifyInspectedNotifications(tree); err != nil {
		return err
	}
	if tree.Incomplete {
		return nil
	}
	for _, k := range kept {
		if strings.HasPrefix(k, keptNotificationNS) && hasDuplicateNotification(tree) {
			return ErrDuplicateNotifications
		}
	}
	return nil
}

// hasDuplicateNotification reports whether any notification name is defined twice in the tree's
// notification files (in the folder CrowdSec reads for the tree's layout).
func hasDuplicateNotification(tree ArchiveInfo) bool {
	dir := notificationsDir
	if _, nested := tree.Files["config/config.yaml"]; nested {
		dir = "config/" + notificationsDir
	}
	byFile, ok := definedNotificationsByFile(tree.Files, dir)
	if !ok {
		return false
	}
	seen := map[string]bool{}
	for _, names := range byFile {
		for _, n := range names {
			if seen[n] {
				return true
			}
			seen[n] = true
		}
	}
	return false
}
