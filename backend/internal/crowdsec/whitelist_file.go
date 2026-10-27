package crowdsec

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// WhitelistRelPath is Charon's own allow-list parser, relative to DataDir. Charon generates it from the
// Whitelist tab: it is rewritten from the database at every CrowdSec start and after every import, so an edit to
// it does not last.
const WhitelistRelPath = "config/parsers/s02-enrich/charon-whitelist.yaml"

// maxWhitelistScanFiles bounds how many parser files an import inspects for other whitelist sections.
const maxWhitelistScanFiles = 200

// maxWhitelistScanBytes caps the size of one parser file the scan reads. A larger file is skipped.
const maxWhitelistScanBytes = 1 << 20

// IsManagedPath reports whether rel (relative to DataDir) names Charon's own whitelist file. The path is cleaned
// and compared in slash form, case-sensitively, like the rest of the editor's path rules.
func IsManagedPath(rel string) bool {
	return path.Clean(filepath.ToSlash(rel)) == WhitelistRelPath
}

// FindOtherWhitelistFiles lists, as sorted slash paths relative to dataDir, the YAML parser files under
// config/parsers that have a top-level whitelist section, other than Charon's own file. The walk is bounded: it
// reads regular files only (symlinks are not followed), at most maxWhitelistScanFiles candidates, and each file up
// to maxWhitelistScanBytes. Files that cannot be read or parsed are skipped. Nothing is changed. It is informational:
// the operator owns these files and CrowdSec reports their errors itself.
func FindOtherWhitelistFiles(dataDir string) []string {
	root := filepath.Join(dataDir, "config", "parsers")
	var found []string
	candidates := 0
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		rel, relErr := filepath.Rel(dataDir, p)
		if relErr != nil || IsManagedPath(rel) {
			return nil
		}
		if candidates == maxWhitelistScanFiles {
			return fs.SkipAll
		}
		candidates++
		if hasWhitelistSection(p) {
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(found)
	return found
}

// hasWhitelistSection reports whether the file at p parses as YAML with a top-level whitelist key. Oversized and
// unparsable files report false.
func hasWhitelistSection(p string) bool {
	f, err := os.Open(p) //nolint:gosec // G304: p comes from a bounded walk of DataDir/config/parsers
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxWhitelistScanBytes+1))
	if err != nil || len(data) > maxWhitelistScanBytes {
		return false
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	_, ok := doc["whitelist"]
	return ok
}
