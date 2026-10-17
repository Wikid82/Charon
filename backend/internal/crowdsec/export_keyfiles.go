package crowdsec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// keySniffBytes is the size of one private key armor search window.
const keySniffBytes = 64 << 10

// privateKeyArmor matches the header line of a PEM or PGP private key block (RSA, EC, OPENSSH,
// ENCRYPTED, PKCS#8 and "PGP PRIVATE KEY BLOCK"): five dashes, BEGIN, a label ending in PRIVATE KEY,
// optionally BLOCK, five dashes.
var privateKeyArmor = regexp.MustCompile(`-{5}BEGIN [A-Z ]*PRIVATE KEY( BLOCK)?-{5}`)

// errFileOmitted tells the export walk that a file stays on the host because it holds private key material.
var errFileOmitted = errors.New("file left out of the export")

// namesKeyFile reports whether the slash path is a key_path target or has a private key file name.
func namesKeyFile(targets map[string]struct{}, slash string) bool {
	_, named := targets[slash]
	return named || hasKeyFileName(slash)
}

// hasKeyFileName reports whether the file name marks a private key container (*.key, *.p12, *.pfx).
func hasKeyFileName(rel string) bool {
	switch strings.ToLower(path.Ext(rel)) {
	case ".key", ".p12", ".pfx":
		return true
	}
	return false
}

// keySniffOverlap is how many bytes consecutive search windows share, so a header cannot straddle a
// window boundary unseen (the longest header, "-----BEGIN ENCRYPTED PRIVATE KEY BLOCK-----", is 43 bytes).
const keySniffOverlap = 128

// holdsKeyArmor reports whether the opened file contains private key armor and leaves the file positioned
// at its start. Files up to maxInspectEntryBytes are searched completely, in keySniffBytes windows that
// overlap by keySniffOverlap bytes. Larger files are too big to inspect: only their first keySniffBytes
// are searched (documented limit), so armor further in goes unseen.
func holdsKeyArmor(f *os.File) (bool, error) {
	info, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("stat file: %w", err)
	}
	limit := info.Size()
	if limit > maxInspectEntryBytes {
		limit = keySniffBytes
	}
	buf := make([]byte, keySniffBytes)
	var found bool
	for off := int64(0); off < limit && !found; off += keySniffBytes - keySniffOverlap {
		n, readErr := f.ReadAt(buf[:min(int64(len(buf)), limit-off)], off)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return false, fmt.Errorf("read file window: %w", readErr)
		}
		found = privateKeyArmor.Match(buf[:n])
		if n < keySniffBytes {
			break
		}
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return false, fmt.Errorf("rewind file: %w", err)
	}
	return found, nil
}

// isKeyPathSetting reports whether a notification setting names a private key file.
func isKeyPathSetting(name string) bool {
	return name == "key_path" || strings.HasSuffix(name, "_key_path")
}

// keyPathTargets returns the slash paths that the notification files below dataDir name as key_path
// (or *_key_path) settings. Absolute values are resolved against refDir, the folder the settings were
// written for (dataDir itself, or the live folder when dataDir is a snapshot of it). Files that
// cannot be read or parsed contribute nothing: the content and name rules of the export still
// apply to the key files themselves.
func keyPathTargets(ctx context.Context, dataDir, refDir string, dirs []string) (map[string]struct{}, error) {
	targets := map[string]struct{}{}
	err := filepath.WalkDir(dataDir, func(p string, d fs.DirEntry, walkErr error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(dataDir, p)
		if relErr != nil || (rel != "." && IsExportExcluded(rel)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() || IsNotificationFile(filepath.ToSlash(rel), nil, dirs) != InDirYAML {
			return nil
		}
		for _, target := range keyPathsOf(dataDir, refDir, rel) {
			targets[target] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan notification files: %w", err)
	}
	return targets, nil
}

// keyPathsOf reads one notification file the way the export does and returns the tree paths of its
// key_path settings.
func keyPathsOf(dataDir, refDir, rel string) []string {
	f, info, err := openExportFile(dataDir, rel, exportHooks{})
	if f == nil || err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	if info.Size() > maxInspectEntryBytes {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, maxInspectEntryBytes+1))
	if err != nil || int64(len(data)) > maxInspectEntryBytes {
		return nil
	}
	docs, err := ParseYAMLDocuments(bytes.NewReader(data), false)
	if err != nil {
		return nil
	}
	var paths []string
	for _, doc := range docs {
		m := documentMapping(doc)
		if m == nil {
			continue
		}
		forEachMappingPair(m, map[*yaml.Node]struct{}{}, func(k, v *yaml.Node) {
			if k.Kind != yaml.ScalarNode || !isKeyPathSetting(k.Value) || v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
				return
			}
			if target, ok := keyPathTarget(refDir, v.Value); ok {
				paths = append(paths, target)
			}
		})
	}
	return paths
}

// keyPathTarget maps a key_path value to a tree path. Only absolute values count: CrowdSec hands the
// configured path to the OS as given, so a relative value depends on the process working directory and
// names no file in the tree (the name and armor rules still cover such a file). Dot segments in an
// absolute value are cleaned first, as the OS would resolve them; the result must still lie in the tree.
func keyPathTarget(refDir, value string) (string, bool) {
	value = filepath.ToSlash(value)
	if strings.HasPrefix(value, "/") {
		value = path.Clean(value)
	}
	return treeRelPath(refDir, value)
}

// forEachMappingPair calls fn for every key/value pair of m, including the pairs a merge key (<<)
// pulls in from other mappings, with aliased values resolved. Merged pairs are visited too even when the
// mapping sets the same key itself: the export errs on the side of keeping a possible key file home.
func forEachMappingPair(m *yaml.Node, seen map[*yaml.Node]struct{}, fn func(k, v *yaml.Node)) {
	if _, dup := seen[m]; dup {
		return
	}
	seen[m] = struct{}{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], resolveAlias(m.Content[i+1])
		if k.Tag != "!!merge" {
			fn(k, v)
			continue
		}
		switch v.Kind {
		case yaml.MappingNode:
			forEachMappingPair(v, seen, fn)
		case yaml.SequenceNode:
			for _, item := range v.Content {
				if mm := resolveAlias(item); mm.Kind == yaml.MappingNode {
					forEachMappingPair(mm, seen, fn)
				}
			}
		}
	}
}

// engineFiles are the top-level files CrowdSec itself reads at startup. An export without them cannot
// be imported, so the armor sniff does not apply to them: they are exported as written, as before
// (a comment that merely mentions a key header must not drop the whole file). The name rule cannot
// match them, and key_path targets are still honoured.
var engineFiles = map[string]struct{}{
	"config.yaml": {}, "profiles.yaml": {}, "simulation.yaml": {}, "acquis.yaml": {},
}

// isEngineFile reports whether the slash path is one of the engine files in the live locations
// (the data folder itself or its config folder).
func isEngineFile(slash string) bool {
	_, ok := engineFiles[strings.TrimPrefix(slash, "config/")]
	return ok
}
