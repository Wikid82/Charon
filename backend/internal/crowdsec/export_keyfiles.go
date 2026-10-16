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

// keySniffBytes is how much of the start of a file is searched for private key armor.
const keySniffBytes = 64 << 10

// privateKeyArmor matches the header line of a PEM or PGP private key block (RSA, EC, OPENSSH,
// ENCRYPTED, PKCS#8 and "PGP PRIVATE KEY BLOCK"): five dashes, BEGIN, a label ending in PRIVATE KEY,
// optionally BLOCK, five dashes.
var privateKeyArmor = regexp.MustCompile(`-{5}BEGIN [A-Z ]*PRIVATE KEY( BLOCK)?-{5}`)

// errKeyFileOmitted tells the export walk that a file holds private key material and stays on the host.
var errKeyFileOmitted = errors.New("private key file left out of the export")

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

// holdsKeyArmor reports whether the first keySniffBytes of the opened file contain private key armor
// and leaves the file positioned at its start.
func holdsKeyArmor(f *os.File) (bool, error) {
	head, err := io.ReadAll(io.LimitReader(f, keySniffBytes))
	if err != nil {
		return false, fmt.Errorf("read file head: %w", err)
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return false, fmt.Errorf("rewind file: %w", err)
	}
	return privateKeyArmor.Match(head), nil
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
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], resolveAlias(m.Content[i+1])
			if k.Kind != yaml.ScalarNode || !isKeyPathSetting(k.Value) || v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
				continue
			}
			if target, ok := treeRelPath(refDir, v.Value); ok {
				paths = append(paths, target)
			}
		}
	}
	return paths
}
