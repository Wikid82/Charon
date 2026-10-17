package crowdsec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// databaseTargetKeys are the keys of db_config that say which database the credentials belong to.
var databaseTargetKeys = []string{"type", "host", "port", "db_name", "db_path"}

// readConfigFile reads and parses the engine configuration file rel of the tree root, keeping its
// permission bits. ok is false when the file is absent, not a regular file, too large or does not parse.
func readConfigFile(root, rel string) (notificationFile, bool, error) {
	if info, err := os.Lstat(filepath.Join(filepath.Clean(root), filepath.FromSlash(rel))); err == nil && !info.Mode().IsRegular() {
		// A link or directory in place of the file (a kept link, for example) is not something to merge.
		return notificationFile{}, false, nil
	}
	data, info, err := readBoundedYAML(root, rel)
	if data == nil {
		return notificationFile{}, false, err
	}
	docs, ok := parseBoundedYAML(rel, data)
	if !ok {
		return notificationFile{}, false, nil
	}
	return notificationFile{rel: rel, docs: docs, mode: info.Mode().Perm(), engine: true}, true, nil
}

// collectConfigFiles returns the parsed engine configuration files that exist below root.
func collectConfigFiles(root string) ([]notificationFile, error) {
	var files []notificationFile
	for _, rel := range configFileRels {
		f, ok, err := readConfigFile(root, rel)
		if err != nil {
			return nil, err
		}
		if ok {
			files = append(files, f)
		}
	}
	return files, nil
}

// MergeRedactedConfig fills the placeholders of the freshly extracted engine configuration files of
// dataDir (config.yaml and config.yaml.local) from the running system's file in the snapshot taken before
// the import, at the same key paths. The database credentials are filled only while the database they
// belong to is unchanged (databaseTargetKeys). Documents that use aliases or merge keys are never merged.
// Placeholders without a counterpart stay and are reported by ScanPlaceholders; files that need no merge
// stay byte-identical.
func MergeRedactedConfig(snapshotDir, dataDir string) error {
	imported, err := collectConfigFiles(dataDir)
	if err != nil {
		return fmt.Errorf("read the imported configuration: %w", err)
	}
	merged, remaining := 0, 0
	for _, f := range imported {
		if filePlaceholders(f) == 0 {
			continue
		}
		live, ok, err := readLiveConfig(snapshotDir, f.rel)
		if err != nil {
			return fmt.Errorf("read the current configuration: %w", err)
		}
		if !ok || !fillConfigFile(f, live) {
			remaining++
			continue
		}
		if filePlaceholders(f) == 0 {
			dropHeaderComment(f.docs)
		} else {
			remaining++
		}
		if err := rewriteNotificationFile(dataDir, f, f.mode); err != nil {
			return err
		}
		merged++
	}
	logger.Log().Infof("crowdsec import: %d configuration files merged, %d still need values", merged, remaining)
	return nil
}

// readLiveConfig reads the snapshot's counterpart of the imported file rel: the same location, otherwise
// the other layout's (a flat live tree can take a nested archive and the reverse).
func readLiveConfig(snapshotDir, rel string) (notificationFile, bool, error) {
	alt := "config/" + rel
	if trimmed, nested := strings.CutPrefix(rel, "config/"); nested {
		alt = trimmed
	}
	for _, candidate := range []string{rel, alt} {
		f, ok, err := readConfigFile(snapshotDir, candidate)
		if err != nil || ok {
			return f, ok, err
		}
	}
	return notificationFile{}, false, nil
}

// fillConfigFile fills f from live document by document and reports whether anything was filled.
func fillConfigFile(f, live notificationFile) bool {
	changed := false
	for i, doc := range f.docs {
		if i >= len(live.docs) || hasAliasOrMerge(doc) || hasAliasOrMerge(live.docs[i]) {
			continue
		}
		if fillConfigRoot(documentRoot(doc), documentRoot(live.docs[i])) {
			changed = true
		}
	}
	return changed
}

// fillConfigRoot fills the placeholders below the root mapping a from l. The database block is skipped
// when it points at another database than the current one, so credentials never follow a changed target.
func fillConfigRoot(a, l *yaml.Node) bool {
	if a == nil || l == nil {
		return false
	}
	changed := false
	for i := 0; i+1 < len(a.Content); i += 2 {
		key, val := a.Content[i], a.Content[i+1]
		live := rawValue(l, key.Value)
		if strings.EqualFold(key.Value, "db_config") && !sameTargetOn(val, live, databaseTargetKeys) {
			continue
		}
		if fillPlaceholders(val, live) {
			changed = true
		}
	}
	return changed
}

// isBlankedURL reports whether archive is live with only the URL password replaced by the placeholder.
func isBlankedURL(archive, live string) bool {
	if !strings.Contains(archive, RedactedPlaceholder) {
		return false
	}
	redacted, _, ok := redactURLPassword(live)
	return ok && redacted == archive
}
