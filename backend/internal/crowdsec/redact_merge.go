package crowdsec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// maxNeedsValues caps the number of file names ScanPlaceholders reports.
const maxNeedsValues = 50

// targetKeys lists, per plugin type, the keys that say where a notification goes. Types missing here are
// treated as unknown: every key outside commonKeepKeys counts.
var targetKeys = map[string][]string{
	"http":     {"url", "unix_socket", "skip_tls_verification", "ca_cert_path", "cert_path", "key_path"},
	"slack":    {"webhook"},
	"splunk":   {"url"},
	"email":    {"smtp_host", "smtp_port", "encryption_type", "auth_type", "helo_host"},
	"sentinel": {"customer_id"},
}

// notificationFile is a notification file of a tree: its parsed documents and the permission bits it
// had, capped at 0600.
type notificationFile struct {
	rel  string // slash path below the tree root
	docs []*yaml.Node
	mode os.FileMode
}

// MergeRedactedNotifications fills placeholders in the freshly extracted notification files of dataDir
// from the running system's files in the snapshot taken before the import. It only merges (and rewrites
// the files it changed); it does not report, see ScanPlaceholders.
//
// A document is merged only when its type and name match exactly one document of the snapshot and its
// target is unchanged (see sameTarget). Documents that use aliases or merge keys are never merged. Files
// that need no merge stay byte-identical.
func MergeRedactedNotifications(snapshotDir, dataDir string) error {
	archive, err := collectNotificationFiles(dataDir, treeNotificationDirs(dataDir))
	if err != nil {
		return fmt.Errorf("read the imported notification files: %w", err)
	}
	var pending []notificationFile
	for _, f := range archive {
		if filePlaceholders(f) > 0 {
			pending = append(pending, f)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	live, err := collectNotificationFiles(snapshotDir, treeNotificationDirs(snapshotDir))
	if err != nil {
		return fmt.Errorf("read the current notification files: %w", err)
	}
	index := indexDocuments(live)

	merged, remaining := 0, 0
	for _, f := range pending {
		mode, changed := os.FileMode(0o600), false
		for _, doc := range f.docs {
			if src, ok := matchLiveDocument(doc, index); ok && fillDocument(doc, src.root) {
				changed = true
				mode &= src.mode
			}
		}
		if !changed {
			remaining++
			continue
		}
		if filePlaceholders(f) == 0 {
			dropHeaderComment(f.docs)
		} else {
			remaining++
		}
		if err := rewriteNotificationFile(dataDir, f, mode); err != nil {
			return err
		}
		merged++
	}
	logger.Log().Infof("crowdsec import: %d notification files merged, %d still need values", merged, remaining)
	return nil
}

// ScanPlaceholders returns the notification files of the final tree below dataDir that still hold
// placeholders, named by their path below dataDir (a leading "config/" is removed in the nested layout,
// as for kept files): sorted, de-duplicated and capped at 50. It selects files exactly like the merge.
func ScanPlaceholders(dataDir string) ([]string, error) {
	files, err := collectNotificationFiles(dataDir, treeNotificationDirs(dataDir))
	if err != nil {
		return nil, fmt.Errorf("scan notification files: %w", err)
	}
	nested := DetectLayout(dataDir) == LayoutNested
	seen := map[string]struct{}{}
	for _, f := range files {
		if filePlaceholders(f) == 0 {
			continue
		}
		name := f.rel
		if nested {
			name = strings.TrimPrefix(name, "config/")
		}
		seen[name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > maxNeedsValues {
		names = names[:maxNeedsValues]
	}
	return names, nil
}

// collectNotificationFiles walks root and returns every notification file the shared selector picks
// (IsNotificationFile), parsed. Hub and preserved state are skipped, links are never followed, and a file
// that is too large or does not parse is skipped.
func collectNotificationFiles(root string, dirs []string) ([]notificationFile, error) {
	var files []notificationFile
	walkErr := fs.WalkDir(os.DirFS(root), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if rel == "." {
			return nil
		}
		if IsExportExcluded(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		kind := IsNotificationFile(rel, nil, dirs)
		if kind == InDirOther || (kind == NotAFile && !hasYAMLExt(rel)) {
			return nil
		}
		if len(files) >= maxInspectEntries {
			logger.Log().Warn("crowdsec import: too many notification files; the rest are left as they are")
			return fs.SkipAll
		}
		file, ok, readErr := readNotificationFile(root, rel, kind, dirs)
		if readErr != nil {
			return readErr
		}
		if ok {
			files = append(files, file)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk notification files: %w", withoutPath(walkErr))
	}
	return files, nil
}

// readNotificationFile reads and parses one candidate file. It reports false for a file that is gone,
// too large, unparsable, or (outside the notification folders) not shaped like a notification.
func readNotificationFile(root, rel string, kind NotificationFileKind, dirs []string) (notificationFile, bool, error) {
	f, info, err := openExportFile(root, filepath.FromSlash(rel), exportHooks{})
	if f == nil { // err is nil when the file vanished or was swapped for a link
		return notificationFile{}, false, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxInspectEntryBytes+1))
	if err != nil {
		return notificationFile{}, false, unreadableFile(rel, err)
	}
	if int64(len(data)) > maxInspectEntryBytes {
		logger.Log().Debugf("crowdsec import: %s is too large to examine", util.SanitizeForLog(rel))
		return notificationFile{}, false, nil
	}
	docs, parseErr := ParseYAMLDocuments(bytes.NewReader(data), false)
	if parseErr != nil {
		logger.Log().Debugf("crowdsec import: %s does not parse; leaving it alone", util.SanitizeForLog(rel))
		return notificationFile{}, false, nil
	}
	nodes := 0
	for _, d := range docs {
		nodes += countNodes(d)
	}
	if nodes > maxRedactNodes {
		return notificationFile{}, false, nil
	}
	if kind == NotAFile && (len(docs) == 0 || IsNotificationFile(rel, docs[0], dirs) != LookAlikeYAML) {
		return notificationFile{}, false, nil
	}
	return notificationFile{rel: rel, docs: docs, mode: info.Mode().Perm() & 0o600}, true, nil
}

// liveDocument is a root mapping of the snapshot with the mode of the file it came from.
type liveDocument struct {
	root *yaml.Node
	mode os.FileMode
}

// indexDocuments groups the snapshot's documents by (type, name). A name defined more than once is kept
// as a list so the caller can tell it is ambiguous.
func indexDocuments(files []notificationFile) map[string][]liveDocument {
	index := map[string][]liveDocument{}
	for _, f := range files {
		for _, doc := range f.docs {
			root := documentRoot(doc)
			if key, ok := identityOf(root); ok {
				index[key] = append(index[key], liveDocument{root: root, mode: f.mode})
			}
		}
	}
	return index
}

// matchLiveDocument returns the single snapshot document an archive document may be filled from. Several
// definitions of the name, a merge key in the snapshot document, or a different shape give no match.
func matchLiveDocument(doc *yaml.Node, index map[string][]liveDocument) (liveDocument, bool) {
	root := documentRoot(doc)
	key, ok := identityOf(root)
	if !ok || hasAliasOrMerge(doc) {
		return liveDocument{}, false
	}
	candidates := index[key]
	if len(candidates) != 1 || hasMergeKey(candidates[0].root) {
		return liveDocument{}, false
	}
	return candidates[0], true
}

// fillDocument fills the placeholders of an archive document from live when the target is unchanged. It
// reports whether anything was filled.
func fillDocument(doc, live *yaml.Node) bool {
	root := documentRoot(doc)
	if !sameTarget(root, live) {
		return false
	}
	return fillPlaceholders(root, live)
}

// documentRoot returns the root mapping of a document node, or nil.
func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return doc.Content[0]
}

// identityOf returns "type\x00name" of a root mapping when both are plain strings.
func identityOf(root *yaml.Node) (string, bool) {
	typ, ok1 := plainString(rawValue(root, "type"))
	name, ok2 := plainString(rawValue(root, "name"))
	if !ok1 || !ok2 {
		return "", false
	}
	return typ + "\x00" + name, true
}

func plainString(n *yaml.Node) (string, bool) {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return "", false
	}
	return n.Value, true
}

// rawValue returns the value node stored under key in m without resolving aliases (nil when absent).
func rawValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i]; k.Kind == yaml.ScalarNode && k.Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// hasMergeKey reports whether the root mapping uses a merge key.
func hasMergeKey(root *yaml.Node) bool {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Tag == "!!merge" {
			return true
		}
	}
	return false
}

// hasAliasOrMerge reports whether the tree holds an alias node or a merge key anywhere. Such documents
// are never merged: a value written into a shared node would change every place that refers to it.
func hasAliasOrMerge(n *yaml.Node) bool {
	if n.Kind == yaml.AliasNode || n.Tag == "!!merge" {
		return true
	}
	return slices.ContainsFunc(n.Content, hasAliasOrMerge)
}

// sameTarget reports whether the archive document still points where the current one does: only merge
// when the target is unchanged. For every key that says where the notification goes, both documents must
// lack it, or both must have a plain value that is either the placeholder (archive side) or equal to the
// current one. The comparison is deliberately exact (tag and text): no trimming, case folding, URL
// cleanup or number/string equivalence, because any such rule would make two different targets compare
// equal. It runs on the archive values as extracted, before anything is filled in.
func sameTarget(archive, live *yaml.Node) bool {
	for _, key := range targetKeysOf(archive, live) {
		a, l := rawValue(archive, key), rawValue(live, key)
		switch {
		case a == nil && l == nil:
			continue
		case a == nil || l == nil:
			return false
		case a.Kind != yaml.ScalarNode || l.Kind != yaml.ScalarNode:
			return false
		case IsPlaceholder(a):
			continue
		case a.Tag != l.Tag || a.Value != l.Value:
			return false
		}
	}
	return true
}

// targetKeysOf returns the keys sameTarget compares: the fixed list of a known type, otherwise every key
// of either document outside the common ones.
func targetKeysOf(archive, live *yaml.Node) []string {
	typ, _ := plainString(rawValue(archive, "type"))
	if keys, ok := targetKeys[typ]; ok {
		return keys
	}
	var keys []string
	for _, m := range []*yaml.Node{archive, live} {
		for i := 0; i+1 < len(m.Content); i += 2 {
			k := m.Content[i].Value
			if !slices.Contains(commonKeepKeys, k) && !slices.Contains(keys, k) {
				keys = append(keys, k)
			}
		}
	}
	return keys
}

// fillPlaceholders copies the current plain value into every placeholder of a that has a plain,
// non-placeholder counterpart at the same position in l. Values are copied into nodes, never spliced into
// text. Archive values that are not placeholders always win, and keys only l has are not added.
func fillPlaceholders(a, l *yaml.Node) bool {
	if a == nil || l == nil {
		return false
	}
	changed := false
	switch a.Kind {
	case yaml.ScalarNode:
		if IsPlaceholder(a) && l.Kind == yaml.ScalarNode && !IsPlaceholder(l) {
			a.Value, a.Tag, a.Style = l.Value, l.Tag, l.Style
			changed = true
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(a.Content); i += 2 {
			if fillPlaceholders(a.Content[i+1], rawValue(l, a.Content[i].Value)) {
				changed = true
			}
		}
	case yaml.SequenceNode:
		if l.Kind != yaml.SequenceNode {
			return false
		}
		for i := 0; i < len(a.Content) && i < len(l.Content); i++ {
			if fillPlaceholders(a.Content[i], l.Content[i]) {
				changed = true
			}
		}
	}
	return changed
}

func filePlaceholders(f notificationFile) int {
	total := 0
	for _, d := range f.docs {
		total += countPlaceholders(d)
	}
	return total
}

// dropHeaderComment removes the comment Charon adds to files that hold placeholders.
func dropHeaderComment(docs []*yaml.Node) {
	if len(docs) == 0 {
		return
	}
	header := "# " + redactHeaderComment
	// A comment above the first line is attached to the document, the root mapping or its first key.
	candidates := []*yaml.Node{docs[0], documentRoot(docs[0])}
	if root := candidates[1]; root != nil && len(root.Content) > 0 {
		candidates = append(candidates, root.Content[0])
	}
	for _, n := range candidates {
		if n != nil && strings.TrimSpace(n.HeadComment) == header {
			n.HeadComment = ""
		}
	}
}

// rewriteNotificationFile replaces the extracted file with the merged documents. It writes a new file
// next to the old one (created exclusively, never through a link) and renames it over the old one, so a
// link in the path is never followed and the permission bits are the ones given, whatever the archive
// entry had.
func rewriteNotificationFile(root string, f notificationFile, mode os.FileMode) error {
	out, err := encodeDocuments(f.docs)
	if err != nil {
		return fmt.Errorf("encode merged notification file: %w", err)
	}
	dest := filepath.Join(filepath.Clean(root), filepath.FromSlash(f.rel))
	if _, err = checkNoSymlinkBelow(root, dest); err != nil {
		return fmt.Errorf("write merged notification file: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".charon-merge-*")
	if err != nil {
		return fmt.Errorf("create merged notification file: %w", withoutPath(err))
	}
	if err = writeAndClose(tmp, out, mode); err == nil {
		err = os.Rename(tmp.Name(), dest)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write merged notification file: %w", withoutPath(err))
	}
	return nil
}

// writeAndClose sets mode on the open file, writes data and closes it.
func writeAndClose(f *os.File, data []byte, mode os.FileMode) error {
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
